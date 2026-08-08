package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
)

type pinger interface {
	Ping(ctx context.Context) error
}

func handleHealth(p pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := p.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{"status": "unavailable"})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}
