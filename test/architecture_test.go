package test

import (
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/ouznoreyni/numflex-sandbox"

// layerOf maps a package path to its Clean Architecture layer number.
// Lower is more inward. A package may only import packages with a
// number less than or equal to its own.
func layerOf(pkg string) (int, bool) {
	switch {
	case strings.HasPrefix(pkg, module+"/internal/entity"):
		return 0, true
	case strings.HasPrefix(pkg, module+"/internal/usecase"):
		return 1, true
	case strings.HasPrefix(pkg, module+"/internal/adapter"):
		return 2, true
	case strings.HasPrefix(pkg, module+"/internal/framework"),
		strings.HasPrefix(pkg, module+"/cmd"):
		return 3, true
	}
	return 0, false
}

func TestDependencyRule(t *testing.T) {
	out, err := exec.Command("go", "list",
		"-f", "{{.ImportPath}} {{join .Imports \" \"}}", "../...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pkg := fields[0]
		pkgLayer, known := layerOf(pkg)
		if !known {
			continue
		}
		for _, imp := range fields[1:] {
			impLayer, known := layerOf(imp)
			if !known {
				continue
			}
			if impLayer > pkgLayer {
				t.Errorf("dependency rule violated: %s (layer %d) imports %s (layer %d)",
					pkg, pkgLayer, imp, impLayer)
			}
		}
	}
}

// TestEntityIsPure asserts the innermost layer imports nothing from this module.
func TestEntityIsPure(t *testing.T) {
	out, err := exec.Command("go", "list",
		"-f", "{{.ImportPath}} {{join .Imports \" \"}}", "../internal/entity/...").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		for _, imp := range fields[1:] {
			if strings.HasPrefix(imp, module+"/") {
				t.Errorf("entity must import nothing from this module, found %s", imp)
			}
		}
	}
}
