package internal_test

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const moduleRootPath = "github.com/trandinh0506/hotel-management/backend"

func TestArchitectureBoundaries(t *testing.T) {
	pkgs, err := scanInternalImports(".")
	if err != nil {
		t.Fatalf("failed to scan internal packages: %v", err)
	}

	violations := checkImportViolations(moduleRootPath, pkgs)
	if len(violations) > 0 {
		t.Fatalf("modular monolith architecture boundary violations found:\n- %s", strings.Join(violations, "\n- "))
	}
}

func TestArchitectureBoundaryRules_DetectsViolations(t *testing.T) {
	cases := []struct {
		name          string
		pkgs          map[string][]string
		wantViolation bool
	}{
		{
			name: "domain_imports_net_http",
			pkgs: map[string][]string{
				moduleRootPath + "/internal/modules/booking/domain": {"net/http"},
			},
			wantViolation: true,
		},
		{
			name: "domain_imports_chi",
			pkgs: map[string][]string{
				moduleRootPath + "/internal/modules/booking/domain": {"github.com/go-chi/chi/v5"},
			},
			wantViolation: true,
		},
		{
			name: "domain_imports_pgx",
			pkgs: map[string][]string{
				moduleRootPath + "/internal/modules/booking/domain": {"github.com/jackc/pgx/v5"},
			},
			wantViolation: true,
		},
		{
			name: "domain_imports_persistence",
			pkgs: map[string][]string{
				moduleRootPath + "/internal/modules/booking/domain": {moduleRootPath + "/internal/modules/booking/persistence/postgres"},
			},
			wantViolation: true,
		},
		{
			name: "module_a_imports_module_b_persistence",
			pkgs: map[string][]string{
				moduleRootPath + "/internal/modules/stayops/application": {moduleRootPath + "/internal/modules/booking/persistence/postgres"},
			},
			wantViolation: true,
		},
		{
			name: "module_a_imports_module_b_domain",
			pkgs: map[string][]string{
				moduleRootPath + "/internal/modules/stayops/application": {moduleRootPath + "/internal/modules/booking/domain"},
			},
			wantViolation: true,
		},
		{
			name: "delivery_http_imports_persistence",
			pkgs: map[string][]string{
				moduleRootPath + "/internal/modules/booking/delivery/http": {moduleRootPath + "/internal/modules/booking/persistence/postgres"},
			},
			wantViolation: true,
		},
		{
			name: "allowed_cross_module_application_import",
			pkgs: map[string][]string{
				moduleRootPath + "/internal/modules/stayops/application": {
					moduleRootPath + "/internal/modules/booking/application",
					moduleRootPath + "/internal/modules/stayops/domain",
					moduleRootPath + "/internal/platform/db",
				},
				moduleRootPath + "/internal/modules/booking/delivery/http": {
					moduleRootPath + "/internal/modules/booking/application",
					moduleRootPath + "/internal/platform/http",
				},
			},
			wantViolation: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			violations := checkImportViolations(moduleRootPath, tc.pkgs)
			if tc.wantViolation && len(violations) == 0 {
				t.Fatalf("expected boundary violation for %s, got none", tc.name)
			}
			if !tc.wantViolation && len(violations) > 0 {
				t.Fatalf("expected no violations for %s, got: %v", tc.name, violations)
			}
		})
	}
}

func scanInternalImports(root string) (map[string][]string, error) {
	fset := token.NewFileSet()
	importsByPkg := make(map[string]map[string]struct{})

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}

		fileAST, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}

		dir := filepath.ToSlash(filepath.Dir(path))
		var pkgImportPath string
		if dir == "." {
			pkgImportPath = moduleRootPath + "/internal"
		} else {
			pkgImportPath = moduleRootPath + "/internal/" + strings.TrimPrefix(dir, "./")
		}

		if _, ok := importsByPkg[pkgImportPath]; !ok {
			importsByPkg[pkgImportPath] = make(map[string]struct{})
		}
		for _, imp := range fileAST.Imports {
			unquoted, unqErr := strconv.Unquote(imp.Path.Value)
			if unqErr != nil {
				continue
			}
			importsByPkg[pkgImportPath][unquoted] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	result := make(map[string][]string, len(importsByPkg))
	for pkg, set := range importsByPkg {
		list := make([]string, 0, len(set))
		for imp := range set {
			list = append(list, imp)
		}
		sort.Strings(list)
		result[pkg] = list
	}
	return result, nil
}

func checkImportViolations(modPrefix string, pkgs map[string][]string) []string {
	modulesPrefix := modPrefix + "/internal/modules/"
	var violations []string

	pkgNames := make([]string, 0, len(pkgs))
	for p := range pkgs {
		pkgNames = append(pkgNames, p)
	}
	sort.Strings(pkgNames)

	for _, pkg := range pkgNames {
		if !strings.HasPrefix(pkg, modulesPrefix) {
			continue
		}
		srcModule, srcSubpath := splitModuleSubpath(strings.TrimPrefix(pkg, modulesPrefix))
		isDomain := hasSegment(srcSubpath, "domain")
		isDeliveryHTTP := strings.HasPrefix(srcSubpath, "delivery/http") || hasSegment(srcSubpath, "delivery")

		for _, imp := range pkgs[pkg] {
			// Rule 1: domain cannot import net/http, chi, pgx, platform/http, platform/db, or persistence.
			if isDomain {
				if imp == "net/http" ||
					strings.HasPrefix(imp, "net/http/") ||
					strings.HasPrefix(imp, "github.com/go-chi/chi") ||
					strings.HasPrefix(imp, "github.com/jackc/pgx") ||
					strings.HasPrefix(imp, modPrefix+"/internal/platform/http") ||
					strings.HasPrefix(imp, modPrefix+"/internal/platform/db") ||
					hasSegment(imp, "persistence") {
					violations = append(violations, fmt.Sprintf("%s (domain) must not import %s", pkg, imp))
				}
			}

			// Rule 2: delivery/http cannot import persistence.
			if isDeliveryHTTP && hasSegment(imp, "persistence") {
				violations = append(violations, fmt.Sprintf("%s (delivery) must not import persistence package %s", pkg, imp))
			}

			// Rule 3: Module A may only import Module B's exported application layer.
			if strings.HasPrefix(imp, modulesPrefix) {
				dstModule, dstSubpath := splitModuleSubpath(strings.TrimPrefix(imp, modulesPrefix))
				if dstModule != "" && dstModule != srcModule {
					if dstSubpath != "application" && !strings.HasPrefix(dstSubpath, "application/") {
						violations = append(violations, fmt.Sprintf("%s (module %s) must only import module %s via application layer, got %s", pkg, srcModule, dstModule, imp))
					}
				}
			}
		}
	}

	return violations
}

func splitModuleSubpath(rel string) (moduleName, subpath string) {
	parts := strings.SplitN(rel, "/", 2)
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func hasSegment(path, segment string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}
