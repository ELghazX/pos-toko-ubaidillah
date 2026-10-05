package categories

import (
	"log"
	"net/http"
	"strconv"

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

func (h *handler) ListCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := h.service.ListCategories(r.Context())
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	component := CategoryPage(categories)
	component.Render(r.Context(), w)

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
