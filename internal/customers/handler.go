package customers

import (
	"encoding/json"
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
	r.Get("/customers", h.ListCustomers)
	r.Get("/customers/new", h.HandleNewCustomerPage)
	r.Post("/customers", h.CreateCustomer)
	r.Get("/customers/{id}/edit", h.HandleEditCustomerPage)
	r.Put("/customers/{id}", h.UpdateCustomer)
	r.Delete("/customers/{id}", h.DeleteCustomer)
	r.Post("/customers/{id}/restore", h.RestoreCustomer)
}

func (h *Handler) ListCustomers(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "active"
	}

	all, err := h.service.ListCustomersWithFilter(r.Context(), status)
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

	var filtered []repo.Customer
	for _, c := range all {
		if q == "" ||
			strings.Contains(strings.ToLower(c.Name), q) ||
			strings.Contains(strings.ToLower(c.Phone.String), q) ||
			strings.Contains(strings.ToLower(c.Address.String), q) {
			filtered = append(filtered, c)
		}
	}

	slices.SortFunc(filtered, func(a, b repo.Customer) int {
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
		CustomerTable(filtered, status).Render(r.Context(), w)
		return
	}

	CustomerPage(filtered, r.URL.Query().Get("q"), sortBy, status).Render(r.Context(), w)
}

func (h *Handler) HandleNewCustomerPage(w http.ResponseWriter, r *http.Request) {
	CreateCustomerPage().Render(r.Context(), w)
}

func (h *Handler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	phone := r.FormValue("phone")
	address := r.FormValue("address")

	_, err := h.service.CreateCustomer(r.Context(), name, phone, address)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/customers")
}

func (h *Handler) HandleEditCustomerPage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	c, err := h.service.GetCustomerByID(r.Context(), int32(id))
	if err != nil {
		log.Println(err)
		http.Error(w, "Pelanggan tidak ditemukan", http.StatusNotFound)
		return
	}

	EditCustomerPage(c).Render(r.Context(), w)
}

func (h *Handler) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	name := r.FormValue("name")
	phone := r.FormValue("phone")
	address := r.FormValue("address")

	err = h.service.UpdateCustomer(r.Context(), int32(id), name, phone, address)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/customers")
}

func (h *Handler) DeleteCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	err = h.service.DeleteCustomer(r.Context(), int32(id))
	if err != nil {
		log.Println("Gagal menghapus pelanggan:", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) RestoreCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	err = h.service.RestoreCustomer(r.Context(), int32(id))
	if err != nil {
		log.Println("Gagal mengaktifkan kembali pelanggan:", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/customers")
		w.WriteHeader(http.StatusOK)
		return
	}

	if strings.Contains(r.Header.Get("Accept"), "application/json") || r.Header.Get("Content-Type") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"success": true, "message": "Pelanggan berhasil diaktifkan kembali"})
		return
	}

	referer := r.Header.Get("Referer")
	if referer != "" {
		http.Redirect(w, r, referer, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/customers", http.StatusSeeOther)
}
