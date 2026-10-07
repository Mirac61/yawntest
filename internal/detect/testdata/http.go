package api

import (
	"encoding/json"
	"io"
	"net/http"
	nethttp "net/http"
)

type Server struct{}

func Health(w http.ResponseWriter, r *http.Request) {}

func (s *Server) CreateTask(w http.ResponseWriter, r *http.Request) {
	var v struct{ Title string }
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
}

func (s *Server) handleList() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {}
}

func aliased(w nethttp.ResponseWriter, r *nethttp.Request) {}

// not a handler: io.Writer instead of http.ResponseWriter
func Write(w io.Writer, r *http.Request) {}
