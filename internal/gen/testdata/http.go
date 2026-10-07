package fixture

import (
	"encoding/json"
	"net/http"
)

type Server struct{ prefix string }

type taskInput struct {
	Title string `json:"title"`
}

func Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s *Server) CreateTask(w http.ResponseWriter, r *http.Request) {
	var input taskInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (s Server) handleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(s.prefix))
	}
}

// Skipped: lazytest can't invent the prefix dependency.
func NewHandler(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {}
}
