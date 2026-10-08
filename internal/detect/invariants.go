package detect

type Invariant string

const (
	InvariantPartsSumToTotal   Invariant = "parts sum to total"
	InvariantEndNotBeforeStart Invariant = "end not before start"
)

var (
	splitTotalWords = map[string]bool{"total": true, "amount": true, "price": true, "cost": true, "balance": true, "fee": true}
	startWords      = map[string]bool{"start": true, "from": true, "begin": true}
	endWords        = map[string]bool{"end": true, "to": true, "until": true, "due": true, "deadline": true}
	rangeWords      = map[string]bool{"range": true, "bounds": true, "period": true, "window": true, "span": true}
)

var integerTypes = map[string]bool{
	"int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
}

// Works on gen-ready field types, so an invariant only fires where gen can build the inputs.
func invariantOf(funcName string, params, results []Field) Invariant {
	switch {
	case splitsTotal(params, results):
		return InvariantPartsSumToTotal
	case returnsTimeRange(funcName, results):
		return InvariantEndNotBeforeStart
	default:
		return ""
	}
}

// Split(total int64, parts int) []int64: an integer money param first, a slice of it as the only result.
// Float totals are left out; rounding makes them fail anyway and floatMoney already hints them.
func splitsTotal(params, results []Field) bool {
	if len(params) == 0 || len(results) != 1 {
		return false
	}
	total := params[0]
	return hasWord(total.Name, splitTotalWords) &&
		integerTypes[total.Type] &&
		results[0].Type == "[]"+total.Type
}

// (start, end time.Time) by result names, or two unnamed times from a func named like WeekRange.
func returnsTimeRange(funcName string, results []Field) bool {
	if len(results) != 2 || results[0].Type != "time.Time" || results[1].Type != "time.Time" {
		return false
	}

	start, end := results[0].Name, results[1].Name
	if start == "" && end == "" {
		return hasWord(funcName, rangeWords)
	}
	return hasWord(start, startWords) && hasWord(end, endWords)
}
