package config

import (
	"os"
	"path/filepath"
	"testing"
)

func ecrireEnv(t *testing.T, contenu string) string {
	t.Helper()
	chemin := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(chemin, []byte(contenu), 0o600); err != nil {
		t.Fatal(err)
	}
	return chemin
}

func TestLoadEnvFilePoseLesValeurs(t *testing.T) {
	t.Setenv("ENV_FILE", ecrireEnv(t, `
# un commentaire
PORT=9090
export FIDELITY=contract
JWT_SECRET="secret # avec dièse"
OTP_STATIC_CODE='000000'
`))
	t.Setenv("PORT", "")
	t.Setenv("FIDELITY", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("OTP_STATIC_CODE", "")

	if err := LoadEnvFile(); err != nil {
		t.Fatalf("chargement : %v", err)
	}
	for clef, attendu := range map[string]string{
		"PORT":            "9090",
		"FIDELITY":        "contract",
		"JWT_SECRET":      "secret # avec dièse",
		"OTP_STATIC_CODE": "000000",
	} {
		if got := os.Getenv(clef); got != attendu {
			t.Errorf("%s = %q, attendu %q", clef, got, attendu)
		}
	}
}

// L'environnement du conteneur l'emporte sur le fichier : c'est ce qui permet
// à `docker run -e` et au bloc `environment:` de compose de surcharger un .env
// monté.
func TestLoadEnvFileNEcrasePas(t *testing.T) {
	t.Setenv("ENV_FILE", ecrireEnv(t, "PORT=9090\n"))
	t.Setenv("PORT", "7000")

	if err := LoadEnvFile(); err != nil {
		t.Fatalf("chargement : %v", err)
	}
	if got := os.Getenv("PORT"); got != "7000" {
		t.Errorf("PORT = %q, attendu 7000", got)
	}
}

func TestLoadEnvFileImpliciteAbsent(t *testing.T) {
	t.Setenv("ENV_FILE", "")
	t.Chdir(t.TempDir())

	if err := LoadEnvFile(); err != nil {
		t.Fatalf("un .env implicite absent n'est pas une erreur : %v", err)
	}
}

func TestLoadEnvFileExpliciteAbsent(t *testing.T) {
	t.Setenv("ENV_FILE", filepath.Join(t.TempDir(), "introuvable.env"))

	if err := LoadEnvFile(); err == nil {
		t.Fatal("un ENV_FILE demandé et absent doit être une erreur")
	}
}

func TestLoadEnvFileLigneInvalide(t *testing.T) {
	t.Setenv("ENV_FILE", ecrireEnv(t, "PORT 9090\n"))

	if err := LoadEnvFile(); err == nil {
		t.Fatal("une ligne sans = doit être une erreur")
	}
}

func TestApplyArguments(t *testing.T) {
	t.Setenv("PORT", "7000")
	t.Setenv("ENV_FILE", "")

	if err := ApplyArguments([]string{"--env-file", "/config/recette.env", "PORT=9090"}); err != nil {
		t.Fatalf("arguments : %v", err)
	}
	if got := os.Getenv("PORT"); got != "9090" {
		t.Errorf("un argument doit l'emporter sur l'environnement : PORT = %q", got)
	}
	if got := os.Getenv("ENV_FILE"); got != "/config/recette.env" {
		t.Errorf("ENV_FILE = %q", got)
	}
}

func TestApplyArgumentsRefuse(t *testing.T) {
	for _, args := range [][]string{
		{"--env-file"},
		{"-v"},
		{"PORT"},
		{"1PORT=9090"},
	} {
		if err := ApplyArguments(args); err == nil {
			t.Errorf("%v aurait dû être refusé", args)
		}
	}
}
