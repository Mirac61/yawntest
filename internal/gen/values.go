package gen

import "strings"

var intTypes = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"byte": true, "rune": true,
}

// sample returns a non-zero Go expression of typ, or "" if typ is unsupported.
func (b *builder) sample(typ string) string {
	switch {
	case typ == "string":
		return `"lazytest"`
	case typ == "bool":
		return "true"
	case typ == "float32", typ == "float64":
		return "1.5"
	case intTypes[typ]:
		return "1"
	case typ == "time.Time":
		b.use("time")
		return "time.Date(2024, time.February, 29, 12, 0, 0, 0, time.UTC)"
	case strings.HasPrefix(typ, "[]"):
		elem := b.sample(strings.TrimPrefix(typ, "[]"))
		if elem == "" {
			return ""
		}
		return typ + "{" + elem + "}"
	default:
		return ""
	}
}
