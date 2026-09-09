package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/cgars/icmn/internal/httpapi"
	"github.com/cgars/icmn/internal/identity"
)

func main() {
	addr := os.Getenv("ICMN_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	store := identity.NewMemoryStore()
	handler := httpapi.New(store)

	slog.Info("ICMN listening", "address", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
