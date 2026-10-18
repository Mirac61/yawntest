package detect

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"testing"
)

// With type information, calls are judged by what they resolve to, not by how they're written.
func TestTypedDetection(t *testing.T) {
	src := `package p

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"
)

func ReadsFile(s string) string {
	data, _ := os.ReadFile(s)
	return string(data)
}

// The method time.Time.After compares, the func time.After starts a timer.
func Later(x, y time.Time) bool { return x.After(y) }

func Wait(n int) bool {
	<-time.After(time.Duration(n))
	return true
}

func Run(s string) error {
	if err := exec.Command(s).Run(); err != nil {
		return err
	}
	return nil
}

// Decode's error depends on s alone, even though it comes from a method on a call's result.
func Decode(s string) (int, error) {
	var n int
	if err := json.NewDecoder(strings.NewReader(s)).Decode(&n); err != nil {
		return 0, err
	}
	return n, nil
}

type Store interface{ Load() error }

// Use's error comes from whatever Store is: a fake would be needed.
func Use(st Store) error {
	if err := st.Load(); err != nil {
		return err
	}
	return nil
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info := &types.Info{
		Uses:       map[*ast.Ident]types.Object{},
		Defs:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
		Types:      map[ast.Expr]types.TypeAndValue{},
	}
	config := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := config.Check("example.com/p", fset, []*ast.File{file}, info); err != nil {
		t.Fatalf("type-check: %v", err)
	}

	pkg := NewPackage([]*ast.File{file})
	pkg.SetTypes("example.com/p", info, NewProgram(nil))

	var candidates []string
	for _, match := range pkg.File(fset, file) {
		candidates = append(candidates, match.Name)
	}
	if want := []string{"Later", "Decode"}; !slices.Equal(candidates, want) {
		t.Errorf("candidates = %v, want %v", candidates, want)
	}

	var returns []string
	for _, ret := range pkg.ErrorReturns(fset, file, []byte(src)) {
		returns = append(returns, ret.Func)
	}
	if want := []string{"Decode"}; !slices.Equal(returns, want) {
		t.Errorf("error returns in %v, want %v", returns, want)
	}
}

func TestIsStandard(t *testing.T) {
	for path, want := range map[string]bool{"os": true, "encoding/json": true, "example.com/m": false, "github.com/x/y": false} {
		if got := isStandard(path); got != want {
			t.Errorf("isStandard(%q) = %v", path, got)
		}
	}
}

// A package that doesn't type-check is loaded syntax only. Every lookup must get that same
// package, or callers visited after the first would see none and judge its funcs pure.
func TestLookupKeepsSyntaxOnlyPackage(t *testing.T) {
	loads := 0
	program := NewProgram(func(path string) *Package {
		loads++
		return NewPackage(nil) // no SetTypes: the type check failed
	})
	first := program.lookup("example.com/broken")
	if first == nil {
		t.Fatal("first lookup returned nil")
	}
	if again := program.lookup("example.com/broken"); again != first {
		t.Errorf("second lookup = %p, want the first package %p", again, first)
	}
	if loads != 1 {
		t.Errorf("loaded %d times, want once", loads)
	}
}
