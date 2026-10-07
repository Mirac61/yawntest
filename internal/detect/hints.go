package detect

import (
	"go/ast"
	"go/token"
	"slices"
	"strings"
	"unicode"
)

type Hint struct {
	Line    int
	Domain  string // money, dates or auth
	Message string
}

func Hints(fset *token.FileSet, file *ast.File) []Hint {
	imports := newImportTable(file)

	var hints []Hint
	hints = append(hints, floatMoney(fset, file)...)
	hints = append(hints, nowWithoutLocation(fset, file, imports)...)

	slices.SortStableFunc(hints, func(a, b Hint) int { return a.Line - b.Line })
	return hints
}

// words splits an identifier into lowercase words: unitPrice → unit, price; HTTPServer → http, server.
func words(name string) []string {
	var result []string
	var current []rune
	runes := []rune(name)
	for i, r := range runes {
		if r == '_' {
			result = appendWord(result, current)
			current = nil
			continue
		}
		if i > 0 && unicode.IsUpper(r) {
			previous := runes[i-1]
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			startsWord := unicode.IsLower(previous) || unicode.IsDigit(previous) || unicode.IsUpper(previous) && nextIsLower
			if startsWord {
				result = appendWord(result, current)
				current = nil
			}
		}
		current = append(current, r)
	}
	return appendWord(result, current)
}

func appendWord(result []string, word []rune) []string {
	if len(word) == 0 {
		return result
	}
	return append(result, strings.ToLower(string(word)))
}

// hasWord also accepts the plural, so "fees" matches "fee".
func hasWord(name string, wanted map[string]bool) bool {
	for _, word := range words(name) {
		if wanted[word] || wanted[strings.TrimSuffix(word, "s")] {
			return true
		}
	}
	return false
}
