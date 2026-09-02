package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// The .env file and the startup arguments are only ways to *set* environment
// variables: Load() alone still interprets them, and the list of settings is
// never duplicated. Precedence, strongest first:
//
//  1. the binary's arguments — docker run image PORT=9090
//  2. the process environment — docker run -e, compose's `environment:`
//  3. the .env file — -v ./.env:/app/.env, or --env-file / ENV_FILE
//  4. Load()'s defaults
//
// A container therefore only ever needs DATABASE_URL, wherever it comes from.

// EnvFilePath gives the file to load, and says whether it was requested
// explicitly. An explicit request that is not satisfied is an error; the
// absence of the implicit `.env` is the normal case.
func EnvFilePath() (path string, explicit bool) {
	if v, ok := os.LookupEnv("ENV_FILE"); ok && v != "" {
		return v, true
	}
	return ".env", false
}

// LoadEnvFile applies the environment file's assignments without
// overwriting what is already set: a variable passed to the container
// always wins over the file.
func LoadEnvFile() error {
	path, explicit := EnvFilePath()
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return nil
		}
		return fmt.Errorf("fichier d'environnement : %w", err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for lineNumber := 1; sc.Scan(); lineNumber++ {
		key, value, err := parseLine(sc.Text())
		if err != nil {
			return fmt.Errorf("%s:%d : %w", path, lineNumber, err)
		}
		if key == "" {
			continue
		}
		// An empty value counts as absent, as in str(): `-e PORT=` therefore
		// does not shadow the file.
		if v, ok := os.LookupEnv(key); ok && v != "" {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return sc.Err()
}

// ApplyArguments interprets the startup arguments, which win over both the
// environment and the file:
//
//	--env-file <chemin>    picks the environment file
//	CLEF=valeur            sets a variable
func ApplyArguments(args []string) error {
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
			key, value, err := parseAssignment(arg)
			if err != nil {
				return err
			}
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// parseLine returns an empty key for blank lines and comments.
func parseLine(line string) (string, string, error) {
	l := strings.TrimSpace(line)
	if l == "" || strings.HasPrefix(l, "#") {
		return "", "", nil
	}
	return parseAssignment(strings.TrimPrefix(l, "export "))
}

func parseAssignment(s string) (string, string, error) {
	key, value, ok := strings.Cut(s, "=")
	if !ok {
		return "", "", fmt.Errorf("affectation attendue sous la forme CLEF=valeur, reçu %q", s)
	}
	key = strings.TrimSpace(key)
	if !validKey(key) {
		return "", "", fmt.Errorf("nom de variable invalide : %q", key)
	}
	return key, unquote(strings.TrimSpace(value)), nil
}

func validKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		alpha := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !alpha && !(i > 0 && r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// unquote strips surrounding quotes and, for double quotes, interprets the
// usual escapes. Outside quotes the value is taken as-is: a `#` belongs to
// the value — a secret happily contains one — so a comment must occupy its
// own line.
func unquote(v string) string {
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
