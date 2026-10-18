package whatsapp

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/settings/whatsapp", h.HandleSettingsPage)
	r.Get("/settings/whatsapp/status", h.HandleGetStatus)
	r.Post("/settings/whatsapp/refresh-qr", h.HandleRefreshQR)
	r.Post("/settings/whatsapp/logout", h.HandleLogout)
	r.Post("/settings/whatsapp/test", h.HandleSendTest)
}

func (h *Handler) HandleSettingsPage(w http.ResponseWriter, r *http.Request) {
	status := h.service.GetStatus()
	if !status.Connected && status.QRCode == "" {
		_ = h.service.RestartQRChannel()
		status = h.service.GetStatus()
	}
	WhatsAppSettingsPage(status).Render(r.Context(), w)
}

func (h *Handler) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	status := h.service.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (h *Handler) HandleRefreshQR(w http.ResponseWriter, r *http.Request) {
	err := h.service.RestartQRChannel()
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Memulai pembuatan QR Code baru...",
	})
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	err := h.service.Logout(r.Context())
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Berhasil memutuskan koneksi WhatsApp",
	})
}

type TestMessageReq struct {
	Phone string `json:"phone"`
}

func (h *Handler) HandleSendTest(w http.ResponseWriter, r *http.Request) {
	var req TestMessageReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Data tidak valid", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	testMsg := "Halo! Ini adalah pesan uji coba dari sistem POS Toko Ubaidillah. Integrasi WhatsApp Anda telah berhasil aktif dan siap digunakan! ✅"
	err := h.service.SendMessage(r.Context(), req.Phone, testMsg)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Pesan uji coba berhasil terkirim!",
	})
}
