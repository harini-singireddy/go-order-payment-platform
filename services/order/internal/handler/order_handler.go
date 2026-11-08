package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/domain"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/repository"
	"github.com/harini-singireddy/go-order-payment-platform/services/order/internal/service"
)

type OrderHandler struct {
	svc *service.OrderService
}

func NewOrderHandler(svc *service.OrderService) *OrderHandler {
	return &OrderHandler{svc: svc}
}

func (h *OrderHandler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
	r.Post("/orders", h.create)
	r.Get("/orders/{id}", h.get)
	return r
}

type createReq struct {
	CustomerID string `json:"customer_id"`
	Items      []struct {
		ProductID  string `json:"product_id"`
		Quantity   int    `json:"quantity"`
		PriceCents int64  `json:"price_cents"`
	} `json:"items"`
}

func (h *OrderHandler) create(w http.ResponseWriter, r *http.Request) {
	idem := r.Header.Get("Idempotency-Key")

	var req createReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	o := &domain.Order{CustomerID: req.CustomerID}
	for _, it := range req.Items {
		o.Items = append(o.Items, domain.OrderItem{
			ProductID:  it.ProductID,
			Quantity:   it.Quantity,
			PriceCents: it.PriceCents,
		})
	}

	created, isNew, err := h.svc.CreateOrder(r.Context(), idem, o)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrEmptyItems), errors.Is(err, domain.ErrInvalidQuantity):
			writeError(w, http.StatusBadRequest, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, "internal error")
		}
		return
	}

	status := http.StatusOK
	if isNew {
		status = http.StatusCreated
	}
	writeJSON(w, status, created)
}

func (h *OrderHandler) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	o, err := h.svc.GetOrder(r.Context(), id)
	if errors.Is(err, repository.ErrNotFound) {
		writeError(w, http.StatusNotFound, "order not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
