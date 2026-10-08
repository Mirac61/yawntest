package shop

type Item struct {
	Name       string
	UnitPrice  float64
	Fees       []float64 // not flagged: only plain float types
	Discount   float64
	Feedback   float64 // not flagged: "feedback" is one word
	TotalCents int64
}

func Charge(amount float32, rate float64) float64 { return float64(amount) * rate }

// Not flagged: without a written type there is no type information.
var shippingCost = 4.99

var defaultBalance float64

// not flagged: "total" alone is not money

type Stats struct {
	Total     float64
	SizeTotal float64
}
