package scan

import (
	"go/ast"
	"go/build/constraint"
	"path/filepath"
	"strings"
)

// BuildConstraint returns the //go:build line of a file, or one derived from a _GOOS or
// _GOARCH file name, so a yawntest file can build under the same conditions as its source.
func BuildConstraint(path string, file *ast.File) string {
	for _, group := range file.Comments {
		if group.Pos() >= file.Package {
			break
		}
		for _, comment := range group.List {
			if constraint.IsGoBuild(comment.Text) {
				return comment.Text
			}
		}
	}
	return fileNameConstraint(filepath.Base(path))
}

// Mirrors go/build: the part before the first "_" never counts, so linux.go applies everywhere
// while open_linux.go and linux_amd64.go don't.
func fileNameConstraint(name string) string {
	parts := strings.Split(strings.TrimSuffix(name, ".go"), "_")[1:]
	count := len(parts)
	switch {
	case count >= 2 && knownOS[parts[count-2]] && knownArch[parts[count-1]]:
		return "//go:build " + parts[count-2] + " && " + parts[count-1]
	case count >= 1 && (knownOS[parts[count-1]] || knownArch[parts[count-1]]):
		return "//go:build " + parts[count-1]
	default:
		return ""
	}
}

// ponytail: copy of go/build's unexported lists; add new ports when Go gains them.
var knownOS = toSet("aix android darwin dragonfly freebsd hurd illumos ios js linux nacl netbsd openbsd plan9 solaris wasip1 windows zos")

var knownArch = toSet("386 amd64 amd64p32 arm armbe arm64 arm64be loong64 mips mipsle mips64 mips64le mips64p32 mips64p32le ppc ppc64 ppc64le riscv riscv64 s390 s390x sparc sparc64 wasm")

func toSet(words string) map[string]bool {
	set := map[string]bool{}
	for _, word := range strings.Fields(words) {
		set[word] = true
	}
	return set
}
