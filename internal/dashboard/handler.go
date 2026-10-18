package dashboard

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/dashboard", h.HandleDashboard)
}

func (h *Handler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	data, err := h.svc.GetDashboardData(r.Context())
	if err != nil {
		log.Println("Error loading dashboard data:", err)
		http.Error(w, "Gagal memuat data dashboard: "+err.Error(), http.StatusInternalServerError)
		return
	}

	DashboardPage(data).Render(r.Context(), w)
}
