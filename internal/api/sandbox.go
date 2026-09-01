package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ouznoreyni/numflex-sandbox/internal/entity"
)

// prefixeSandbox porte ce que la plateforme réelle n'a pas. Il est délibérément
// distinct de prefixeGateway : la promesse du sandbox est que /api/gateway/v1
// expose exactement les 33 routes du contrat ARTP, et une commodité de bac à
// sable ne doit pas s'y glisser. Un client qui bascule son baseUrl vers l'ARTP
// ne perd donc que ce qu'il sait ne pas exister là-bas.
const prefixeSandbox = "/api/sandbox/v1"

func (d *Deps) routesSandbox(g *gin.RouterGroup) {
	g.DELETE("/demandes", d.deletePurgeDemandes)
}

// deletePurgeDemandes efface les données de test de l'opérateur appelant et
// remet les numéros concernés en état d'être rejoués.
//
// Le périmètre est createur_operateur_id, pas le filtre de /mes-demandes : une
// demande appartient à deux opérateurs à la fois, et seul son créateur l'a
// fabriquée. Un Port-IN créé par un partenaire pour exercer le Port-OUT de
// l'appelant ne se purge donc pas depuis ce jeton — c'est au partenaire de le
// faire, avec le sien.
//
// La restauration du registre est ce qui rend l'opération utile : sans elle un
// numéro déjà porté resterait bloqué par DELAI_PORTAGE_NON_RESPECTE pendant
// trois mois, et purger la demande ne permettrait pas de rejouer le scénario.
// La règle est « le numéro rentre chez lui » — operateur_origine_id — et non
// « le numéro retrouve son état de seed » : pour une tranche ensemencée déjà
// portée (77200, détenue par ORANGE mais d'origine YAS), la purge la ramène
// chez YAS, pas chez ORANGE.
func (d *Deps) deletePurgeDemandes(c *gin.Context) {
	operateurID := Appelant(c).OperatorID

	tx, err := d.DB.Pool.Begin(c)
	if err != nil {
		d.R.Fail(c, entity.InternalError("ouverture de transaction"))
		return
	}
	defer tx.Rollback(c)

	ids := []string{}
	rows, err := tx.Query(c, `SELECT id FROM demande WHERE createur_operateur_id = $1`, operateurID)
	if err != nil {
		d.R.Fail(c, entity.InternalError("lecture des demandes à purger"))
		return
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			d.R.Fail(c, entity.InternalError("lecture des demandes à purger"))
			return
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		d.R.Fail(c, entity.InternalError("lecture des demandes à purger"))
		return
	}

	// Le numéro d'une demande particulier vit sur demande.numero ; ceux d'une
	// flotte sur demande_numero. Les deux sources comptent, y compris les
	// numéros exclus : eux aussi ont pu bouger avant de l'être.
	numeros := []string{}
	rows, err = tx.Query(c, `
		SELECT numero FROM demande WHERE id = ANY($1)
		 UNION
		SELECT numero FROM demande_numero WHERE demande_id = ANY($1)`, ids)
	if err != nil {
		d.R.Fail(c, entity.InternalError("lecture des numéros à restaurer"))
		return
	}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			d.R.Fail(c, entity.InternalError("lecture des numéros à restaurer"))
			return
		}
		numeros = append(numeros, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		d.R.Fail(c, entity.InternalError("lecture des numéros à restaurer"))
		return
	}

	// Avant le DELETE des demandes : la clé étrangère de reverse_request n'a pas
	// d'ON DELETE CASCADE et bloquerait la suppression. Les demandes de reverse
	// encore en attente, sans demande rattachée, partent aussi — ce sont les
	// données de test de l'appelant au même titre.
	tagReverse, err := tx.Exec(c,
		`DELETE FROM reverse_request WHERE operateur_id = $1 OR demande_id = ANY($2)`,
		operateurID, ids)
	if err != nil {
		d.R.Fail(c, entity.InternalError("purge des demandes de reverse"))
		return
	}

	// L'OTP est consommé à la création de la demande. Le laisser interdirait de
	// rejouer le même numéro sans repasser par otp/send, et laisserait une
	// ligne sans objet — la table n'a aucune clé étrangère vers demande.
	tagOTP, err := tx.Exec(c, `DELETE FROM otp WHERE numero = ANY($1)`, numeros)
	if err != nil {
		d.R.Fail(c, entity.InternalError("purge des OTP"))
		return
	}

	// demande_numero, demande_client, etape_historique et confirmation portent
	// ON DELETE CASCADE : elles suivent.
	tagDemandes, err := tx.Exec(c, `DELETE FROM demande WHERE id = ANY($1)`, ids)
	if err != nil {
		d.R.Fail(c, entity.InternalError("purge des demandes"))
		return
	}

	tagNumeros, err := tx.Exec(c, `
		UPDATE numero
		   SET operateur_actuel_id = operateur_origine_id,
		       date_dernier_portage = NULL,
		       deja_restitue = false
		 WHERE msisdn = ANY($1)`, numeros)
	if err != nil {
		d.R.Fail(c, entity.InternalError("restauration du registre"))
		return
	}

	if err := tx.Commit(c); err != nil {
		d.R.Fail(c, entity.InternalError("validation de la transaction"))
		return
	}

	d.R.OK(c, http.StatusOK, "Demandes purgées avec succès", map[string]any{
		"demandesSupprimees": tagDemandes.RowsAffected(),
		"numerosRestaures":   tagNumeros.RowsAffected(),
		"otpSupprimes":       tagOTP.RowsAffected(),
		"reverseSupprimees":  tagReverse.RowsAffected(),
	})
}
