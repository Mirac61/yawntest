package shop

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

type Order struct {
	ID       int       `json:"id"`
	Customer string    `json:"customer"`
	Items    []string  `json:"items"`
	Price    float64   `json:"price"`
	Created  time.Time `json:"created"`
}

// Bug: a client sending bad JSON gets a 500 instead of a 400.
func CreateOrder(w http.ResponseWriter, r *http.Request) {
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// Bug: drops the remainder, so 100 split three ways only adds up to 99.
func Split(total int64, parts int) []int64 {
	if parts <= 0 || parts > 1000 {
		return nil
	}
	result := make([]int64, parts)
	for i := range result {
		result[i] = total / int64(parts)
	}
	return result
}

// Bug: panics on an empty name.
func Initial(name string) string { return name[:1] }

func ParseQuantity(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("parse quantity %q: %w", s, err)
	}
	return n, nil
}

func InvoiceYear() int { return time.Now().Year() }
