package server

import (
	_ "embed"
	"log/slog"
	"net/http"

	httpSwagger "github.com/swaggo/http-swagger/v2"
)

//go:embed swagger.swagger.json
var swaggerJSON []byte

func swaggerDocHandler(log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(swaggerJSON); err != nil {
			log.ErrorContext(r.Context(), "failed to write swagger spec", "error", err)
		}
	}
}

func registerSwagger(mux *http.ServeMux, log *slog.Logger) {
	mux.HandleFunc("/swagger/doc.json", swaggerDocHandler(log))
	mux.Handle("/swagger/", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))
}
