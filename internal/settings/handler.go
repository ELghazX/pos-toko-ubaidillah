package settings

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/settings/company", h.GetCompanyPage)
	r.Post("/settings/company", h.UpdateCompany)
}

func (h *Handler) GetCompanyPage(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.service.GetCompanyConfig(r.Context())
	if err != nil {
		log.Println("Error loading company config:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	successMsg := ""
	if r.URL.Query().Get("saved") == "1" {
		successMsg = "Profil dan informasi toko berhasil disimpan!"
	}

	CompanySettingsPage(cfg, successMsg, "").Render(r.Context(), w)
}

func (h *Handler) UpdateCompany(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Data formulir tidak valid: "+err.Error(), http.StatusBadRequest)
		return
	}

	req := UpdateCompanyReq{
		Name:          r.FormValue("name"),
		Tagline:       r.FormValue("tagline"),
		Phone:         r.FormValue("phone"),
		Address:       r.FormValue("address"),
		ReceiptFooter: r.FormValue("receipt_footer"),
	}

	err := h.service.UpdateCompanyConfig(r.Context(), req)
	if err != nil {
		cfg, _ := h.service.GetCompanyConfig(r.Context())
		w.WriteHeader(http.StatusBadRequest)
		CompanySettingsPage(cfg, "", "Gagal menyimpan: "+err.Error()).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/settings/company?saved=1", http.StatusSeeOther)
}
