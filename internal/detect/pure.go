package detect

import (
	"go/ast"
	"slices"
	"strings"
)

var ioPackages = map[string]bool{
	"bufio": true, "io": true, "io/fs": true, "io/ioutil": true,
	"os": true, "os/exec": true, "syscall": true,
	"net": true, "net/http": true, "database/sql": true,
	"log": true, "log/slog": true,
	"crypto/rand": true, "math/rand": true, "math/rand/v2": true,
	"os/user": true, "os/signal": true, "net/smtp": true, "net/rpc": true, "crypto/tls": true,
	"runtime/debug": true, "testing": true,
}

var impureTimeFuncs = map[string]bool{
	"Now": true, "Since": true, "Until": true, "Sleep": true,
	"After": true, "AfterFunc": true, "Tick": true, "NewTimer": true, "NewTicker": true,
}

var fmtIOPrefixes = []string{"Print", "Fprint", "Scan", "Fscan"}

// Packages that are mostly pure but have a few funcs touching the file system.
var impureMembers = map[string]map[string]bool{
	"path/filepath": {"Abs": true, "EvalSymlinks": true, "Glob": true, "Walk": true, "WalkDir": true},
}

func detectPureFunc(fn *ast.FuncDecl, e env) (string, bool) {
	isPlainFunc := fn.Recv == nil && fn.Name.IsExported() && fn.Type.TypeParams == nil
	if !isPlainFunc {
		return "", false
	}
	// Without a result there is nothing to compare for determinism.
	if fn.Type.Results == nil {
		return "", false
	}

	params := e.pkg.withUnderlying(fields(fn.Type.Params, e.imports))
	if len(params) == 0 {
		return "", false
	}
	for _, param := range params {
		if param.Basic() == "" {
			return "", false
		}
	}

	// Funcs, channels and interfaces (other than error) can't be compared for determinism:
	// two calls of NewMiddleware() return two closures that are never equal.
	if e.pkg.uncomparableResult(fn, e.imports) {
		return "", false
	}
	if e.hasSideEffects(fn) {
		return "", false
	}
	return "basic params, no I/O", true
}

// mustPrefixes name funcs that panic on bad input by convention: MustParse, RequireFromString.
var mustPrefixes = []string{"Must", "Require"}

// Words that make a comment about a panic a note about a bug, not a contract.
var bugMarkers = []string{"bug", "todo", "fixme", "xxx"}

// panicsByDesign: named Must*/Require*, or a doc comment that says it panics, like
// "panics if n < 0". "Bug: panics on empty input" is the opposite, and stays a candidate.
func panicsByDesign(fn *ast.FuncDecl) bool {
	for _, prefix := range mustPrefixes {
		rest, found := strings.CutPrefix(fn.Name.Name, prefix)
		if found && (rest == "" || startsUpper(rest)) {
			return true
		}
	}
	if fn.Doc == nil {
		return false
	}
	doc := strings.ToLower(fn.Doc.Text())
	return strings.Contains(doc, "panic") && !slices.ContainsFunc(bugMarkers, func(marker string) bool {
		return strings.Contains(doc, marker)
	})
}

func isImpureMember(pkgPath, member string) bool {
	switch pkgPath {
	case "time":
		return impureTimeFuncs[member]
	case "fmt":
		return slices.ContainsFunc(fmtIOPrefixes, func(prefix string) bool {
			return strings.HasPrefix(member, prefix)
		})
	default:
		return ioPackages[pkgPath] || impureMembers[pkgPath][member]
	}
}
