// Package horodatage rend les horodatages comme la plateforme les rend.
//
// Deux mesures, cohérentes entre elles (captures particulier du 2026-08-27 et
// entreprise du 2026-09-18) :
//
//   - un horodatage que la requête vient d'écrire sort à la nanoseconde —
//     dateDemande sur la création (« 22:39:23.583043149Z »), dateFinalisation
//     sur la COMPLETION (« 23:14:55.794666796Z ») : la plateforme sérialise
//     l'objet Java qu'elle tient encore en mémoire ;
//   - le même horodatage relu par une requête ultérieure sort à la
//     milliseconde (« 22:39:23.583Z ») : il revient de Mongo, qui ne stocke
//     pas plus fin.
//
// Le sandbox stocke en Postgres, à la microseconde. Pour reproduire la
// nanoseconde, chaque écriture note sa valeur Go dans la requête (Marquer),
// et le rendu la retrouve à la relecture (Frais) ; tout ce qui n'a pas été
// marqué est tronqué à la milliseconde.
package horodatage

import (
	"context"
	"fmt"
	"time"
)

// Cle est la clé sous laquelle un registre est posé dans le contexte de la
// requête — une chaîne, pour que gin.Context.Value la retrouve via c.Get.
const Cle = "horodatage.frais"

// Registre recense les horodatages écrits par la requête en cours.
type Registre struct{ ecrits []time.Time }

func Nouveau() *Registre { return &Registre{} }

// Marquer note qu'une écriture vient d'utiliser t. Sans registre dans le
// contexte — un tick du moteur, hors requête — l'appel ne fait rien.
func Marquer(ctx context.Context, t time.Time) {
	if r, ok := ctx.Value(Cle).(*Registre); ok && r != nil {
		r.ecrits = append(r.ecrits, t)
	}
}

// Frais rend, pour un horodatage relu en base, la valeur pleine précision
// que la requête en cours a écrite — ou false si l'écriture vient d'une
// requête antérieure. Postgres tient la microseconde : la correspondance se
// fait à cette précision.
func Frais(ctx context.Context, relu time.Time) (time.Time, bool) {
	r, ok := ctx.Value(Cle).(*Registre)
	if !ok || r == nil {
		return time.Time{}, false
	}
	for _, t := range r.ecrits {
		if t.Truncate(time.Microsecond).Equal(relu.Truncate(time.Microsecond)) {
			return t, true
		}
	}
	return time.Time{}, false
}

// Instant sérialise un time.Time comme java.time.Instant.toString().
type Instant time.Time

func (i Instant) MarshalJSON() ([]byte, error) {
	return []byte(`"` + Format(time.Time(i)) + `"`), nil
}

// Format reproduit java.time.format.DateTimeFormatter.ISO_INSTANT : UTC, et
// la fraction de seconde par groupes de trois chiffres — aucune, trois, six
// ou neuf — selon le dernier chiffre significatif. « .580Z », jamais « .58Z »
// comme le ferait time.RFC3339Nano.
func Format(t time.Time) string {
	t = t.UTC()
	base := t.Format("2006-01-02T15:04:05")
	ns := t.Nanosecond()
	switch {
	case ns == 0:
		return base + "Z"
	case ns%1_000_000 == 0:
		return fmt.Sprintf("%s.%03dZ", base, ns/1_000_000)
	case ns%1_000 == 0:
		return fmt.Sprintf("%s.%06dZ", base, ns/1_000)
	default:
		return fmt.Sprintf("%s.%09dZ", base, ns)
	}
}
