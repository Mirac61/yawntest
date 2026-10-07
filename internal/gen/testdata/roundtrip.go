package fixture

import "time"

type Base struct{ CreatedBy string }

type Task struct {
	Base
	ID       int       `json:"id"`
	Title    string    `json:"title,omitempty"`
	Tags     []string  `json:"tags"`
	Due      time.Time `json:"due"`
	Owner    *string   `json:"owner"`
	Price    float64   `json:"price"`
	Done     bool
	Secret   string `json:"-"`
	internal string
}

type Comment struct {
	Text string `json:"text,omitempty"`
}
