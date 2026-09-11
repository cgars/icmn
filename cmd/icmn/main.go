package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/cgars/icmn/internal/httpapi"
	"github.com/cgars/icmn/internal/identity"
	"github.com/cgars/icmn/internal/postgres"
)

func main() {
	addr := os.Getenv("ICMN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	var store identity.Store = identity.NewMemoryStore()
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		pg, err := postgres.Open(context.Background(), databaseURL)
		if err != nil {
			slog.Error("open PostgreSQL", "error", err)
			os.Exit(1)
		}
		defer pg.Close()
		store = pg
	}
	handler := httpapi.New(store)

	slog.Info("ICMN listening", "address", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
