package api

type Task struct {
	ID, Owner int    `json:"id"`
	Title     string `json:"title,omitempty"`
	secret    string
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
