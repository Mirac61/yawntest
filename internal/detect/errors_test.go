package detect

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

func TestErrorReturns(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string // the returns that are worth a test
	}{
		{
			name: "made here",
			body: `func f(s string) error {
	if s == "" {
		return errors.New("empty")
	}
	if len(s) > 9 {
		return fmt.Errorf("too long: %d", len(s))
	}
	if s == "x" {
		return ErrBad
	}
	if s == "y" {
		return &ParseError{s}
	}
	return nil
}`,
			want: []string{`return errors.New("empty")`, `return fmt.Errorf("too long: %d", len(s))`, "return ErrBad", "return &ParseError{s}"},
		},
		{
			name: "passed on from input-only calls",
			body: `func f(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("parse %q: %w", s, err)
	}
	if _, err := url.Parse(s); err != nil {
		return 0, err
	}
	if err := check(s); err != nil {
		return 0, err
	}
	return n, nil
}

func check(s string) error { return nil }`,
			want: []string{`return 0, fmt.Errorf("parse %q: %w", s, err)`, "return 0, err", "return 0, err"},
		},
		{
			name: "passed on from I/O is left out",
			body: `func f(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := save(data); err != nil {
		return nil, err
	}
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return data, nil
}

func save(data []byte) error { return os.WriteFile("x", data, 0o644) }`,
		},
		{
			name: "params, fields and funcs without an error result",
			body: `func f(err error) error {
	if err != nil {
		return err
	}
	return fmt.Errorf("wrap: %w", s.err)
}

func g() int { return 1 }`,
		},
		{
			name: "the assignment before the return wins",
			body: `func f(s string) error {
	err := errors.New("bad")
	err = os.Remove(s)
	return err
}`,
		},
		{
			name: "closures are judged on their own",
			body: `func f(paths []string) error {
	return walk(func(p string) error {
		if p == "" {
			return errors.New("empty path")
		}
		return nil
	})
}`,
			want: []string{`return errors.New("empty path")`},
		},
		{
			name: "pure methods of the same receiver",
			body: `func (u User) Validate() error {
	if err := u.validateName(); err != nil {
		return err
	}
	return nil
}

func (u User) validateName() error { return nil }`,
			want: []string{"return err"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			src := "package p\n\n" + imports + test.body + "\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "p.go", src, 0)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}

			var got []string
			for _, ret := range NewPackage([]*ast.File{file}).ErrorReturns(fset, file, []byte(src)) {
				got = append(got, ret.Code)
				if ret.Func == "" {
					t.Errorf("%s: no func name", ret.Code)
				}
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

const imports = `import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strconv"
)

`

func TestErrorReturnFuncNames(t *testing.T) {
	src := `package p

import "errors"

func (u User) Validate() error { return errors.New("a") }

var check = func() error { return errors.New("b") }

func Walk() error {
	return each(func() error { return errors.New("c") })
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var got []string
	for _, ret := range NewPackage([]*ast.File{file}).ErrorReturns(fset, file, []byte(src)) {
		got = append(got, ret.Func)
	}
	want := []string{"User.Validate", "func literal", "Walk (func literal)"}
	if !slices.Equal(got, want) {
		t.Errorf("funcs = %q, want %q", got, want)
	}
}

func TestIgnores(t *testing.T) {
	src := `package p

//yawntest:ignore talks to the payment provider
func Charge() error {
	return nil
}

func Other() {
	x := 1 //yawntest:ignore
	_ = x
	//yawntest:ignore
	_ = x
	_ = x
}

//yawntest:ignored is another word, not the directive
func Last() {}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "p.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ignored := Ignores(fset, file)

	var got []int
	for line := 1; line <= 18; line++ {
		if ignored(line) {
			got = append(got, line)
		}
	}
	// 3-6: the whole Charge func. 9-10: same line and the next. 11-12: the comment and the next.
	want := []int{3, 4, 5, 6, 9, 10, 11, 12}
	if !slices.Equal(got, want) {
		t.Errorf("ignored lines = %v, want %v", got, want)
	}
}
