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

	params := fields(fn.Type.Params, e.imports)
	if len(params) == 0 {
		return "", false
	}
	for _, param := range params {
		if param.Type == "" {
			return "", false
		}
	}

	if e.hasSideEffects(fn) {
		return "", false
	}
	return "basic params, no I/O", true
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
