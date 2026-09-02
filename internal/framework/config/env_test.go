package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEnv(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadEnvFileSetsValues(t *testing.T) {
	t.Setenv("ENV_FILE", writeEnv(t, `
# a comment
PORT=9090
export FIDELITY=contract
JWT_SECRET="secret # with hash"
OTP_STATIC_CODE='000000'
`))
	t.Setenv("PORT", "")
	t.Setenv("FIDELITY", "")
	t.Setenv("JWT_SECRET", "")
	t.Setenv("OTP_STATIC_CODE", "")

	if err := LoadEnvFile(); err != nil {
		t.Fatalf("chargement : %v", err)
	}
	for key, expected := range map[string]string{
		"PORT":            "9090",
		"FIDELITY":        "contract",
		"JWT_SECRET":      "secret # with hash",
		"OTP_STATIC_CODE": "000000",
	} {
		if got := os.Getenv(key); got != expected {
			t.Errorf("%s = %q, attendu %q", key, got, expected)
		}
	}
}

// The container's environment wins over the file: this is what lets
// `docker run -e` and compose's `environment:` block override a mounted
// .env.
func TestLoadEnvFileDoesNotOverwrite(t *testing.T) {
	t.Setenv("ENV_FILE", writeEnv(t, "PORT=9090\n"))
	t.Setenv("PORT", "7000")

	if err := LoadEnvFile(); err != nil {
		t.Fatalf("chargement : %v", err)
	}
	if got := os.Getenv("PORT"); got != "7000" {
		t.Errorf("PORT = %q, attendu 7000", got)
	}
}

func TestLoadEnvFileImplicitAbsent(t *testing.T) {
	t.Setenv("ENV_FILE", "")
	t.Chdir(t.TempDir())

	if err := LoadEnvFile(); err != nil {
		t.Fatalf("a missing implicit .env is not an error: %v", err)
	}
}

func TestLoadEnvFileExplicitAbsent(t *testing.T) {
	t.Setenv("ENV_FILE", filepath.Join(t.TempDir(), "introuvable.env"))

	if err := LoadEnvFile(); err == nil {
		t.Fatal("a requested but missing ENV_FILE must be an error")
	}
}

func TestLoadEnvFileInvalidLine(t *testing.T) {
	t.Setenv("ENV_FILE", writeEnv(t, "PORT 9090\n"))

	if err := LoadEnvFile(); err == nil {
		t.Fatal("a line without = must be an error")
	}
}

func TestApplyArguments(t *testing.T) {
	t.Setenv("PORT", "7000")
	t.Setenv("ENV_FILE", "")

	if err := ApplyArguments([]string{"--env-file", "/config/recette.env", "PORT=9090"}); err != nil {
		t.Fatalf("arguments : %v", err)
	}
	if got := os.Getenv("PORT"); got != "9090" {
		t.Errorf("an argument must win over the environment: PORT = %q", got)
	}
	if got := os.Getenv("ENV_FILE"); got != "/config/recette.env" {
		t.Errorf("ENV_FILE = %q", got)
	}
}

func TestApplyArgumentsRejects(t *testing.T) {
	for _, args := range [][]string{
		{"--env-file"},
		{"-v"},
		{"PORT"},
		{"1PORT=9090"},
	} {
		if err := ApplyArguments(args); err == nil {
			t.Errorf("%v should have been rejected", args)
		}
	}
}
