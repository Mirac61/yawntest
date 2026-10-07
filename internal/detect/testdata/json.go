package api

import "time"

type Base struct{ CreatedBy string }

type Task struct {
	Base
	ID       int       `json:"id"`
	Title    string    `json:"title,omitempty"`
	Tags     []string  `json:"tags"`
	Due      time.Time `json:"due"`
	Owner    *string   `json:"owner"`
	Done     bool
	Secret   string `json:"-"`
	internal string
}

type Hidden struct {
	Secret string `json:"-"`
	Raw    string `db:"raw"`
}

type plain struct {
	Name string `json:"name"`
}

type NoTags struct{ Name string }

type Page[T any] struct {
	Items []T `json:"items"`
}
