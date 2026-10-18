package suppliers

import (
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/suppliers", h.ListSuppliers)
	r.Get("/suppliers/new", h.HandleNewSupplierPage)
	r.Post("/suppliers", h.CreateSupplier)
	r.Get("/suppliers/{id}/edit", h.HandleEditSupplierPage)
	r.Put("/suppliers/{id}", h.UpdateSupplier)
	r.Delete("/suppliers/{id}", h.DeleteSupplier)
}

func (h *Handler) ListSuppliers(w http.ResponseWriter, r *http.Request) {
	all, err := h.service.ListSuppliers(r.Context())
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "name_asc"
	}

	var filtered []repo.Supplier
	for _, s := range all {
		if q == "" ||
			strings.Contains(strings.ToLower(s.Name), q) ||
			strings.Contains(strings.ToLower(s.Phone.String), q) ||
			strings.Contains(strings.ToLower(s.Address.String), q) {
			filtered = append(filtered, s)
		}
	}

	slices.SortFunc(filtered, func(a, b repo.Supplier) int {
		switch sortBy {
		case "name_desc":
			return strings.Compare(strings.ToLower(b.Name), strings.ToLower(a.Name))
		case "newest":
			return int(b.ID - a.ID)
		default:
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		}
	})

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		SupplierTable(filtered).Render(r.Context(), w)
		return
	}

	SupplierPage(filtered, r.URL.Query().Get("q"), sortBy).Render(r.Context(), w)
}

func (h *Handler) HandleNewSupplierPage(w http.ResponseWriter, r *http.Request) {
	CreateSupplierPage().Render(r.Context(), w)
}

func (h *Handler) CreateSupplier(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	phone := r.FormValue("phone")
	address := r.FormValue("address")

	_, err := h.service.CreateSupplier(r.Context(), name, phone, address)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/suppliers")
}

func (h *Handler) HandleEditSupplierPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	sup, err := h.service.GetSupplierByID(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "Supplier tidak ditemukan", http.StatusNotFound)
		return
	}

	EditSupplierPage(sup).Render(r.Context(), w)
}

func (h *Handler) UpdateSupplier(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	phone := r.FormValue("phone")
	address := r.FormValue("address")

	if err := h.service.UpdateSupplier(r.Context(), int32(id), name, phone, address); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/suppliers")
}

func (h *Handler) DeleteSupplier(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteSupplier(r.Context(), int32(id)); err != nil {
		log.Println(err)
		http.Error(w, "Gagal menghapus supplier", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
