package auth

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	svc Service
}

func NewHandler(svc Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterPublicRoutes(r chi.Router) {
	r.Get("/login", h.HandleLoginPage)
	r.Post("/login", h.HandleLogin)
	r.Post("/logout", h.HandleLogout)
	r.Get("/logout", h.HandleLogout)
}

func (h *Handler) RegisterAdminRoutes(r chi.Router) {
	r.Get("/settings/users", h.HandleUsersPage)
	r.Post("/settings/users", h.HandleCreateUser)
	r.Post("/settings/users/{id}/update", h.HandleUpdateUser)
}

func (h *Handler) HandleRootRedirect(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user != nil && user.IsAdmin() {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/pos", http.StatusSeeOther)
}

func (h *Handler) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	if userID, ok := ParseSessionCookie(r); ok {
		if user, err := h.svc.GetSessionUserByID(r.Context(), userID); err == nil && user != nil {
			if user.IsAdmin() {
				http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/pos", http.StatusSeeOther)
			return
		}
	}

	LoginPage("", "").Render(r.Context(), w)
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		LoginPage("Data login tidak valid", "").Render(r.Context(), w)
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")

	user, err := h.svc.Authenticate(r.Context(), username, password)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		LoginPage(err.Error(), username).Render(r.Context(), w)
		return
	}

	SetSessionCookie(w, user.ID)

	if user.IsAdmin() {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/pos", http.StatusSeeOther)
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) HandleUsersPage(w http.ResponseWriter, r *http.Request) {
	users, err := h.svc.ListUsers(r.Context())
	if err != nil {
		http.Error(w, "Gagal memuat daftar pengguna: "+err.Error(), http.StatusInternalServerError)
		return
	}

	successMsg := ""
	switch r.URL.Query().Get("saved") {
	case "created":
		successMsg = "Akun pengguna baru berhasil ditambahkan!"
	case "updated":
		successMsg = "Data akun pengguna berhasil diperbarui!"
	}

	UsersPage(users, successMsg, "").Render(r.Context(), w)
}

func (h *Handler) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Data formulir tidak valid", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	name := r.FormValue("name")
	role := r.FormValue("role")
	password := r.FormValue("password")

	if err := h.svc.CreateUser(r.Context(), username, name, password, role); err != nil {
		users, _ := h.svc.ListUsers(r.Context())
		w.WriteHeader(http.StatusBadRequest)
		UsersPage(users, "", "Gagal menambah pengguna: "+err.Error()).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/settings/users?saved=created", http.StatusSeeOther)
}

func (h *Handler) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id64, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 32)
	if err != nil || id64 <= 0 {
		http.Error(w, "ID pengguna tidak valid", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Data formulir tidak valid", http.StatusBadRequest)
		return
	}

	username := r.FormValue("username")
	name := r.FormValue("name")
	role := r.FormValue("role")
	password := r.FormValue("password")
	isActive := r.FormValue("is_active") == "true" || r.FormValue("is_active") == "on"

	if err := h.svc.UpdateUser(r.Context(), int32(id64), username, name, password, role, isActive); err != nil {
		users, _ := h.svc.ListUsers(r.Context())
		w.WriteHeader(http.StatusBadRequest)
		UsersPage(users, "", "Gagal memperbarui pengguna: "+err.Error()).Render(r.Context(), w)
		return
	}

	http.Redirect(w, r, "/settings/users?saved=updated", http.StatusSeeOther)
}
