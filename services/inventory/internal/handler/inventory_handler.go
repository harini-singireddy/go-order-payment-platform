package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/harini-singireddy/go-order-payment-platform/services/inventory/internal/repository"
)

type InventoryHandler struct{ repo *repository.InventoryRepo }

func New(repo *repository.InventoryRepo) *InventoryHandler { return &InventoryHandler{repo: repo} }

func (h *InventoryHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/inventory/{productID}", h.get)
	return r
}

func (h *InventoryHandler) get(w http.ResponseWriter, r *http.Request) {
	pid := chi.URLParam(r, "productID")
	s, err := h.repo.Get(r.Context(), pid)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s)
}
