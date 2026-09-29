package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplicationTransactionsDoNotCallGORMMethods(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	modulesDirectory := filepath.Join(repositoryRoot, "internal", "modules")
	gormMethods := map[string]struct{}{
		"Model": {}, "Table": {}, "Where": {}, "Create": {}, "Save": {},
		"Updates": {}, "Update": {}, "Delete": {}, "Exec": {}, "Raw": {},
	}

	err := filepath.WalkDir(modulesDirectory, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.Contains(filepath.ToSlash(path), "/application/") ||
			!strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			return nil
		}
		fileSet := token.NewFileSet()
		parsed, err := parser.ParseFile(fileSet, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}

		ast.Inspect(parsed, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			receiver, ok := selector.X.(*ast.Ident)
			if !ok || receiver.Name != "tx" {
				return true
			}
			if _, isGORMMethod := gormMethods[selector.Sel.Name]; !isGORMMethod {
				return true
			}

			position := fileSet.Position(call.Pos())
			relativePath, relErr := filepath.Rel(repositoryRoot, position.Filename)
			if relErr != nil {
				relativePath = position.Filename
			}
			t.Errorf(
				"%s:%d calls tx.%s directly; application transactions must use contract ports instead of GORM methods",
				filepath.ToSlash(relativePath),
				position.Line,
				selector.Sel.Name,
			)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("inspect application transactions: %v", err)
	}
}
