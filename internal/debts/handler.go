package debts

import (
	"encoding/json"
	"log"
	"math"
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

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/debts", h.HandleDebtsPage)
	r.Post("/debts", h.HandleRecordSalePayment)
	r.Get("/debts/customer/{id}", h.HandleCustomerDebtDetail)
	r.Post("/debts/pay-sale", h.HandleRecordSalePayment)
	r.Post("/debts/customer/{id}/pay", h.HandleRecordCustomerPayment)
	r.Get("/debts/customer/{id}/draft-wa", h.HandleGetDebtReminderDraft)
	r.Post("/debts/customer/{id}/send-wa", h.HandleSendReminderWhatsApp)
	r.Post("/debts/customer/{id}/remind-wa", h.HandleSendReminderWhatsApp)
}

func (h *Handler) isJSONRequest(r *http.Request) bool {
	return strings.Contains(r.Header.Get("Accept"), "application/json") ||
		r.Header.Get("X-Requested-With") == "XMLHttpRequest" ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json")
}

func (h *Handler) respondError(w http.ResponseWriter, r *http.Request, msg string, status int) {
	if h.isJSONRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		json.NewEncoder(w).Encode(map[string]any{
			"success": false,
			"error":   msg,
		})
		return
	}
	http.Error(w, msg, status)
}

func (h *Handler) HandleDebtsPage(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab != "invoices" {
		tab = "customers"
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	filter := r.URL.Query().Get("filter")

	summary, err := h.svc.GetDebtsSummary(r.Context())
	if err != nil {
		log.Println("Error loading debts summary:", err)
		summary = &DebtsSummary{}
	}

	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}
	limit := 15

	company, _ := h.svc.GetCompanyConfig(r.Context())

	data := &DebtsPageData{
		ActiveTab:    tab,
		Summary:      *summary,
		Query:        q,
		Filter:       filter,
		Page:         page,
		Limit:        limit,
		CompanyName:  company.Name,
		CompanyPhone: company.Phone,
	}

	if tab == "customers" {
		customerRecap, err := h.svc.ListCustomerDebtRecap(r.Context(), q, filter)
		if err != nil {
			log.Println("Error listing customer debt recap:", err)
		}
		data.CustomerRecap = customerRecap
	} else {
		invoices, total, err := h.svc.ListDebtInvoices(r.Context(), q, filter, page, limit)
		if err != nil {
			log.Println("Error listing debt invoices:", err)
		}
		data.Invoices = invoices
		data.TotalInvoices = total
		data.TotalPages = int(math.Ceil(float64(total) / float64(limit)))
		if data.TotalPages == 0 {
			data.TotalPages = 1
		}
	}

	DebtsPage(data).Render(r.Context(), w)
}

func (h *Handler) HandleCustomerDebtDetail(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID pelanggan tidak valid", http.StatusBadRequest)
		return
	}

	detail, err := h.svc.GetCustomerDebtDetail(r.Context(), int32(id))
	if err != nil {
		http.Error(w, "Gagal memuat detail piutang pelanggan: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(detail)
}

func (h *Handler) HandleRecordSalePayment(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseMultipartForm(32 << 20)
	_ = r.ParseForm()

	saleIDStr := r.FormValue("sale_id")
	saleID, err := strconv.Atoi(saleIDStr)
	if err != nil || saleID <= 0 {
		h.respondError(w, r, "ID nota penjualan tidak valid", http.StatusBadRequest)
		return
	}

	amountStr := r.FormValue("amount")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		h.respondError(w, r, "Nominal pembayaran tidak valid", http.StatusBadRequest)
		return
	}

	notes := r.FormValue("notes")
	err = h.svc.RecordSaleDebtPayment(r.Context(), int32(saleID), amount, notes)
	if err != nil {
		h.respondError(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	redirectTab := r.FormValue("tab")
	if redirectTab == "" {
		redirectTab = "invoices"
	}

	if h.isJSONRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": "Pembayaran nota kasbon berhasil dicatat",
		})
		return
	}

	http.Redirect(w, r, "/debts?tab="+redirectTab, http.StatusSeeOther)
}

func (h *Handler) HandleRecordCustomerPayment(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		h.respondError(w, r, "ID pelanggan tidak valid", http.StatusBadRequest)
		return
	}

	_ = r.ParseMultipartForm(32 << 20)
	_ = r.ParseForm()

	amountStr := r.FormValue("amount")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amount <= 0 {
		h.respondError(w, r, "Nominal pembayaran tidak valid", http.StatusBadRequest)
		return
	}

	notes := r.FormValue("notes")
	err = h.svc.RecordCustomerDebtPaymentFIFO(r.Context(), int32(id), amount, notes)
	if err != nil {
		h.respondError(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	if h.isJSONRequest(r) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"success": true,
			"message": "Pembayaran kasbon pelanggan berhasil dicatat",
		})
		return
	}

	http.Redirect(w, r, "/debts?tab=customers", http.StatusSeeOther)
}

func (h *Handler) HandleGetDebtReminderDraft(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.respondError(w, r, "ID pelanggan tidak valid", http.StatusBadRequest)
		return
	}

	phone := r.URL.Query().Get("phone")
	draft, resolvedPhone, isConnected, err := h.svc.GetDebtReminderDraft(r.Context(), int32(id), phone)
	if err != nil {
		h.respondError(w, r, "Gagal memuat draf pesan tagihan: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success":      true,
		"message":      draft,
		"phone":        resolvedPhone,
		"wa_connected": isConnected,
	})
}

func (h *Handler) HandleSendReminderWhatsApp(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		h.respondError(w, r, "ID pelanggan tidak valid", http.StatusBadRequest)
		return
	}

	_ = r.ParseMultipartForm(32 << 20)
	_ = r.ParseForm()

	phone := r.FormValue("phone")
	message := r.FormValue("message")

	err = h.svc.SendDebtReminderWhatsApp(r.Context(), int32(id), phone, message)
	if err != nil {
		h.respondError(w, r, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"success": true,
		"message": "Pesan pengingat tagihan kasbon berhasil dikirim langsung via WhatsApp (Whatsmeow)!",
	})
}
