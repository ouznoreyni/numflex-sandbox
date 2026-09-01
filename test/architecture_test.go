package test

import (
	"os/exec"
	"strings"
	"testing"
)

const module = "github.com/ouznoreyni/numflex-sandbox"

// listFormat asks `go list` for the import path plus every kind of import a
// package's files can carry: .Imports (non-test files), .TestImports (the
// package's own _test.go files) and .XTestImports (an external <pkg>_test
// package). A violation introduced only through a test file would be invisible
// to the dependency rule if .Imports were the only field consulted.
const listFormat = `{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}`

// hasPathPrefix reports whether pkg is prefix itself or a package nested under
// it (prefix followed by "/"), so that e.g. "internal/entity" does not also
// match a hypothetical "internal/entityx". Same shape as the whole-segment
// guard already used for the /api/gateway/v1 route prefix.
func hasPathPrefix(pkg, prefix string) bool {
	return pkg == prefix || strings.HasPrefix(pkg, prefix+"/")
}

// layerOf maps a package path to its Clean Architecture layer number.
// Lower is more inward. A package may only import packages with a
// number less than or equal to its own.
func layerOf(pkg string) (int, bool) {
	switch {
	case hasPathPrefix(pkg, module+"/internal/entity"):
		return 0, true
	case hasPathPrefix(pkg, module+"/internal/usecase"):
		return 1, true
	case hasPathPrefix(pkg, module+"/internal/adapter"):
		return 2, true
	case hasPathPrefix(pkg, module+"/internal/framework"),
		hasPathPrefix(pkg, module+"/cmd"):
		return 3, true
	}
	return 0, false
}

func TestDependencyRule(t *testing.T) {
	out, err := exec.Command("go", "list", "-tags=integration",
		"-f", listFormat, "../...").Output()
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
	out, err := exec.Command("go", "list", "-tags=integration",
		"-f", listFormat, "../internal/entity/...").Output()
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
