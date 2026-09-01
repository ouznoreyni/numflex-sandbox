package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Le fichier .env et les arguments de démarrage ne sont qu'un moyen de *poser*
// des variables d'environnement : Load() reste seule à les interpréter, et la
// liste des réglages ne se dédouble pas. Précédence, du plus fort au plus
// faible :
//
//  1. les arguments du binaire — docker run image PORT=9090
//  2. l'environnement du processus — docker run -e, `environment:` de compose
//  3. le fichier .env — -v ./.env:/app/.env, ou --env-file / ENV_FILE
//  4. les défauts de Load()
//
// Un conteneur n'a donc besoin que de DATABASE_URL, d'où qu'elle vienne.

// CheminFichierEnv donne le fichier à charger, et dit s'il a été demandé
// explicitement. Une demande explicite non satisfaite est une erreur ;
// l'absence du `.env` implicite est le cas normal.
func CheminFichierEnv() (chemin string, explicite bool) {
	if v, ok := os.LookupEnv("ENV_FILE"); ok && v != "" {
		return v, true
	}
	return ".env", false
}

// ChargerFichierEnv applique les affectations du fichier d'environnement sans
// écraser ce qui est déjà posé : une variable passée au conteneur l'emporte
// toujours sur le fichier.
func ChargerFichierEnv() error {
	chemin, explicite := CheminFichierEnv()
	f, err := os.Open(chemin)
	if err != nil {
		if os.IsNotExist(err) && !explicite {
			return nil
		}
		return fmt.Errorf("fichier d'environnement : %w", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for numero := 1; sc.Scan(); numero++ {
		clef, valeur, err := analyserLigne(sc.Text())
		if err != nil {
			return fmt.Errorf("%s:%d : %w", chemin, numero, err)
		}
		if clef == "" {
			continue
		}
		// Une valeur vide compte pour absente, comme dans str() : `-e PORT=`
		// ne masque donc pas le fichier.
		if v, ok := os.LookupEnv(clef); ok && v != "" {
			continue
		}
		if err := os.Setenv(clef, valeur); err != nil {
			return err
		}
	}
	return sc.Err()
}

// AppliquerArguments interprète les arguments de démarrage, qui l'emportent
// sur l'environnement comme sur le fichier :
//
//	--env-file <chemin>    choisit le fichier d'environnement
//	CLEF=valeur            pose une variable
func AppliquerArguments(args []string) error {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--env-file":
			if i+1 >= len(args) {
				return fmt.Errorf("--env-file attend un chemin")
			}
			i++
			if err := os.Setenv("ENV_FILE", args[i]); err != nil {
				return err
			}
		case strings.HasPrefix(arg, "--env-file="):
			if err := os.Setenv("ENV_FILE", strings.TrimPrefix(arg, "--env-file=")); err != nil {
				return err
			}
		case strings.HasPrefix(arg, "-"):
			return fmt.Errorf("argument inconnu %q ; formes acceptées : --env-file <chemin>, CLEF=valeur", arg)
		default:
			clef, valeur, err := analyserAffectation(arg)
			if err != nil {
				return err
			}
			if err := os.Setenv(clef, valeur); err != nil {
				return err
			}
		}
	}
	return nil
}

// analyserLigne rend une clef vide pour les lignes vides et les commentaires.
func analyserLigne(ligne string) (string, string, error) {
	l := strings.TrimSpace(ligne)
	if l == "" || strings.HasPrefix(l, "#") {
		return "", "", nil
	}
	return analyserAffectation(strings.TrimPrefix(l, "export "))
}

func analyserAffectation(s string) (string, string, error) {
	clef, valeur, ok := strings.Cut(s, "=")
	if !ok {
		return "", "", fmt.Errorf("affectation attendue sous la forme CLEF=valeur, reçu %q", s)
	}
	clef = strings.TrimSpace(clef)
	if !clefValide(clef) {
		return "", "", fmt.Errorf("nom de variable invalide : %q", clef)
	}
	return clef, deciter(strings.TrimSpace(valeur)), nil
}

func clefValide(clef string) bool {
	if clef == "" {
		return false
	}
	for i, r := range clef {
		alpha := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !alpha && !(i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// deciter retire les guillemets encadrants et, pour les guillemets doubles,
// interprète les échappements usuels. Hors guillemets la valeur est prise
// telle quelle : un `#` appartient à la valeur — un secret en contient
// volontiers — et un commentaire doit donc occuper sa propre ligne.
func deciter(v string) string {
	if len(v) < 2 {
		return v
	}
	switch {
	case v[0] == '\'' && v[len(v)-1] == '\'':
		return v[1 : len(v)-1]
	case v[0] == '"' && v[len(v)-1] == '"':
		return strings.NewReplacer(
			`\n`, "\n", `\r`, "\r", `\t`, "\t", `\"`, `"`, `\\`, `\`,
		).Replace(v[1 : len(v)-1])
	}
	return v
}
