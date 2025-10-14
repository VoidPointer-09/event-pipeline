package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	istore "github.com/example/event-pipeline/internal/storage"
	imetrics "github.com/example/event-pipeline/internal/metrics"
)

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

func main() {
	imetrics.Serve()
	db, err := istore.Connect()
	if err != nil { panic(err) }
	r := chi.NewRouter()
	r.Get("/users/{id}", func(w http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "id")
		u, err := db.GetUserWithLastOrders(req.Context(), id, 5)
		if err != nil {
			if err == sql.ErrNoRows {
				writeJSONError(w, "user not found", http.StatusNotFound)
			} else {
				writeJSONError(w, "internal server error", http.StatusInternalServerError)
				log.Error().Err(err).Str("userId", id).Msg("failed to get user")
			}
			return
		}
		writeJSON(w, u)
	})
	r.Get("/orders/{id}", func(w http.ResponseWriter, req *http.Request) {
		id := chi.URLParam(req, "id")
		o, err := db.GetOrderWithPayment(req.Context(), id)
		if err != nil {
			if err == sql.ErrNoRows {
				writeJSONError(w, "order not found", http.StatusNotFound)
			} else {
				writeJSONError(w, "internal server error", http.StatusInternalServerError)
				log.Error().Err(err).Str("orderId", id).Msg("failed to get order")
			}
			return
		}
		writeJSON(w, o)
	})
	addr := os.Getenv("API_ADDR")
	if addr == "" { addr = ":8080" }
	log.Info().Str("addr", addr).Msg("api listening")
	http.ListenAndServe(addr, r)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:   http.StatusText(statusCode),
		Message: message,
	})
}
