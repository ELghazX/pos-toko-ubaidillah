package categories

import (
	"log"
	"net/http"
	"slices"
	"strconv"
	"strings"

	repo "github.com/ELghazX/pos-toko-ubaidillah/internal/adapters/postgresql/sqlc"
	"github.com/go-chi/chi/v5"
)

type handler struct {
	service Service
}

func NewHandler(service Service) *handler {
	return &handler{
		service: service,
	}
}

func (h *handler) RegisterRoutes(r chi.Router) {
	r.Get("/categories", h.ListCategories)
	r.Get("/categories/new", h.HandleNewCategoryPage)
	r.Post("/categories", h.CreateCategory)
	r.Get("/categories/{id}/edit", h.HandleEditCategoryPage)
	r.Put("/categories/{id}", h.UpdateCategory)
	r.Delete("/categories/{id}", h.DeleteCategory)
}

func (h *handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	all, err := h.service.ListCategories(r.Context())
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "code_asc"
	}

	var filtered []repo.Category
	for _, c := range all {
		if q == "" || strings.Contains(strings.ToLower(c.Code), q) || strings.Contains(strings.ToLower(c.Name), q) {
			filtered = append(filtered, c)
		}
	}

	slices.SortFunc(filtered, func(a, b repo.Category) int {
		switch sortBy {
		case "name_asc":
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		case "name_desc":
			return strings.Compare(strings.ToLower(b.Name), strings.ToLower(a.Name))
		case "newest":
			return int(b.ID - a.ID)
		default:
			return strings.Compare(strings.ToLower(a.Code), strings.ToLower(b.Code))
		}
	})

	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		CategoryTable(filtered).Render(r.Context(), w)
		return
	}

	CategoryPage(filtered, r.URL.Query().Get("q"), sortBy).Render(r.Context(), w)
}

func (h *handler) HandleNewCategoryPage(w http.ResponseWriter, r *http.Request) {
	CreateCategoryPage().Render(r.Context(), w)
}

func (h *handler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("code")
	name := r.FormValue("name")

	_, err := h.service.CreateCategory(r.Context(), code, name)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("HX-Redirect", "/categories")
}

func (h *handler) HandleEditCategoryPage(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	cat, err := h.service.GetCategoryByID(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "Kategori tidak ditemukan", http.StatusNotFound)
		return
	}

	EditCategoryPage(cat).Render(r.Context(), w)
}

func (h *handler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Println(err)
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}
	code := r.FormValue("code")
	name := r.FormValue("name")

	if err := h.service.UpdateCategory(r.Context(), int32(id), code, name); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Redirect", "/categories")
}

func (h *handler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		log.Println(err)
		http.Error(w, "ID tidak valid", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteCategory(r.Context(), int32(id)); err != nil {
		log.Println(err)
		http.Error(w, "Gagal menghapus kategori", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
