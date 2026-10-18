package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

type StatusResponse struct {
	Connected bool   `json:"connected"`
	Phone     string `json:"phone"`
	PushName  string `json:"push_name"`
	QRCode    string `json:"qr_code"`
	Status    string `json:"status"`
}

type Service struct {
	Client      *whatsmeow.Client
	container   *sqlstore.Container
	postgresDSN string
	logger      waLog.Logger

	mu          sync.RWMutex
	currentQR   string
	isConnected bool
	statusText  string
	phone       string
	pushName    string
	qrActive    bool

	cooldownMu sync.Mutex
	lastSent   map[string]time.Time
}

func NewService(dsn string) (*Service, error) {
	logger := waLog.Stdout("WhatsApp", "INFO", true)

	ctx := context.Background()
	container, err := sqlstore.New(ctx, "postgres", dsn, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to init whatsmeow postgres store: %w", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get whatsmeow device: %w", err)
	}
	if deviceStore == nil {
		deviceStore = container.NewDevice()
	}

	client := whatsmeow.NewClient(deviceStore, logger)

	svc := &Service{
		Client:      client,
		container:   container,
		postgresDSN: dsn,
		logger:      logger,
		statusText:  "Inisialisasi...",
		lastSent:    make(map[string]time.Time),
	}

	client.AddEventHandler(svc.handleEvents)

	if client.Store.ID != nil {
		svc.phone = client.Store.ID.User
		svc.pushName = client.Store.PushName
		svc.statusText = "Menghubungkan..."
		go func() {
			err := client.Connect()
			if err != nil {
				svc.logger.Errorf("Failed to connect whatsmeow: %v", err)
			}
		}()
	} else {
		go svc.startQRChannel()
	}

	return svc, nil
}

func (s *Service) handleEvents(evt interface{}) {
	switch v := evt.(type) {
	case *events.Connected:
		s.mu.Lock()
		s.isConnected = true
		s.currentQR = ""
		s.statusText = "Terhubung"
		if s.Client.Store.ID != nil {
			s.phone = s.Client.Store.ID.User
		}
		s.pushName = s.Client.Store.PushName
		s.mu.Unlock()
		s.logger.Infof("WhatsApp connected: %s", s.phone)

	case *events.Disconnected:
		s.mu.Lock()
		s.isConnected = false
		s.statusText = "Terputus"
		s.mu.Unlock()
		s.logger.Warnf("WhatsApp disconnected")

	case *events.LoggedOut:
		s.mu.Lock()
		s.isConnected = false
		s.phone = ""
		s.pushName = ""
		s.currentQR = ""
		s.statusText = "Logged out"
		s.mu.Unlock()
		s.logger.Warnf("WhatsApp logged out (reason: %s). Clearing store...", v.Reason)
		_ = s.Client.Store.Delete(context.Background())
		go s.startQRChannel()
	}
}

func (s *Service) startQRChannel() {
	s.mu.Lock()
	if (s.Client.Store.ID != nil && s.Client.IsConnected()) || s.qrActive {
		s.mu.Unlock()
		return
	}
	s.qrActive = true
	s.statusText = "Menyiapkan QR Code..."
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.qrActive = false
		s.mu.Unlock()
	}()

	if s.Client.IsConnected() {
		s.Client.Disconnect()
	}

	qrChan, err := s.Client.GetQRChannel(context.Background())
	if err != nil {
		s.mu.Lock()
		s.currentQR = ""
		s.statusText = "Gagal memuat QR Code: " + err.Error()
		s.mu.Unlock()
		s.logger.Errorf("Failed to get QR channel: %v", err)
		return
	}

	err = s.Client.Connect()
	if err != nil {
		s.mu.Lock()
		s.currentQR = ""
		s.statusText = "Gagal menghubungkan WhatsApp: " + err.Error()
		s.mu.Unlock()
		s.logger.Errorf("Failed to connect for QR: %v", err)
		return
	}

	for evt := range qrChan {
		switch evt.Event {
		case "code":
			pngBytes, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
			if err != nil {
				s.logger.Errorf("Failed to encode QR code: %v", err)
				continue
			}
			base64Img := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

			s.mu.Lock()
			s.currentQR = base64Img
			s.statusText = "Scan QR Code untuk menghubungkan"
			s.mu.Unlock()

		case "success":
			s.mu.Lock()
			s.isConnected = true
			s.currentQR = ""
			s.statusText = "Terhubung"
			if s.Client.Store.ID != nil {
				s.phone = s.Client.Store.ID.User
			}
			s.pushName = s.Client.Store.PushName
			s.mu.Unlock()
			s.logger.Infof("WhatsApp successfully paired and connected!")

		case "timeout":
			s.mu.Lock()
			s.currentQR = ""
			s.statusText = "QR Code kedaluwarsa. Silakan muat ulang."
			s.mu.Unlock()
			s.logger.Warnf("WhatsApp QR code timed out")
		}
	}

	s.mu.Lock()
	if !s.isConnected && s.currentQR == "" {
		s.statusText = "QR Code kedaluwarsa. Silakan muat ulang."
	}
	s.mu.Unlock()
}

func (s *Service) RestartQRChannel() error {
	s.mu.Lock()
	if s.Client != nil && s.Client.Store.ID != nil && s.Client.IsConnected() {
		s.mu.Unlock()
		return nil
	}
	s.currentQR = ""
	s.statusText = "Menyiapkan QR Code..."
	s.mu.Unlock()

	if s.Client != nil && s.Client.IsConnected() {
		s.Client.Disconnect()
	}

	go func() {
		for i := 0; i < 20; i++ {
			s.mu.RLock()
			active := s.qrActive
			s.mu.RUnlock()
			if !active {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		s.startQRChannel()
	}()

	return nil
}

func (s *Service) GetStatus() StatusResponse {
	s.mu.RLock()
	defer s.mu.RUnlock()

	connected := s.Client != nil && s.Client.IsConnected() && s.Client.Store.ID != nil
	phone := s.phone
	if phone == "" && s.Client != nil && s.Client.Store.ID != nil {
		phone = s.Client.Store.ID.User
	}

	return StatusResponse{
		Connected: connected,
		Phone:     phone,
		PushName:  s.pushName,
		QRCode:    s.currentQR,
		Status:    s.statusText,
	}
}

func (s *Service) Logout(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Client != nil {
		_ = s.Client.Logout(ctx)
		_ = s.Client.Store.Delete(ctx)
		s.isConnected = false
		s.phone = ""
		s.pushName = ""
		s.currentQR = ""
		s.statusText = "Terputus"
	}

	newDevice := s.container.NewDevice()
	s.Client.Store = newDevice

	go s.startQRChannel()
	return nil
}

func NormalizePhoneNumber(phone string) (string, error) {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)

	if strings.HasPrefix(digits, "0") {
		digits = "62" + digits[1:]
	} else if strings.HasPrefix(digits, "+62") {
		digits = digits[1:]
	} else if !strings.HasPrefix(digits, "62") && len(digits) > 8 {
		digits = "62" + digits
	}

	if len(digits) < 9 {
		return "", fmt.Errorf("nomor telepon tidak valid: %s", phone)
	}

	return digits, nil
}

func (s *Service) SendMessage(ctx context.Context, phone string, message string) error {
	if s.Client == nil || !s.Client.IsConnected() {
		return fmt.Errorf("whatsapp belum terhubung. Silakan hubungkan WhatsApp terlebih dahulu")
	}

	digits, err := NormalizePhoneNumber(phone)
	if err != nil {
		return err
	}

	s.cooldownMu.Lock()
	if last, exists := s.lastSent[digits]; exists {
		if elapsed := time.Since(last); elapsed < 5*time.Second {
			remaining := int(5-elapsed.Seconds()) + 1
			s.cooldownMu.Unlock()
			return fmt.Errorf("mohon tunggu %d detik sebelum mengirim pesan ke nomor ini lagi", remaining)
		}
	}
	s.lastSent[digits] = time.Now()
	if len(s.lastSent) > 100 {
		cutoff := time.Now().Add(-2 * time.Minute)
		for k, v := range s.lastSent {
			if v.Before(cutoff) {
				delete(s.lastSent, k)
			}
		}
	}
	s.cooldownMu.Unlock()

	recipient := types.NewJID(digits, types.DefaultUserServer)

	msg := &waE2E.Message{
		Conversation: proto.String(message),
	}

	_, err = s.Client.SendMessage(ctx, recipient, msg)
	if err != nil {
		return fmt.Errorf("gagal mengirim pesan WhatsApp: %w", err)
	}

	return nil
}

type SaleReceiptData struct {
	CompanyName   string
	Tagline       string
	Address       string
	Phone         string
	ReceiptFooter string

	InvoiceNo     string
	Date          time.Time
	CustomerName  string
	CustomerPhone string

	Items []SaleReceiptItem

	DiscountNota  float64
	TotalAmount   float64
	PaidAmount    float64
	ChangeAmount  float64
	PaymentMethod string
	PaymentStatus string
	DueDate       *time.Time
}

type SaleReceiptItem struct {
	ProductName string
	UnitName    string
	Qty         int32
	Price       float64
	Discount    float64
	Subtotal    float64
	IsOpenPrice bool
}

func FormatReceiptText(data SaleReceiptData) string {
	var sb strings.Builder

	storeName := data.CompanyName
	if storeName == "" {
		storeName = "POS STORE"
	}

	sb.WriteString(fmt.Sprintf("🧾 *NOTA TRANSAKSI - %s*\n", strings.ToUpper(storeName)))
	if data.Tagline != "" {
		sb.WriteString(fmt.Sprintf("%s\n", data.Tagline))
	}
	if data.Address != "" {
		sb.WriteString(fmt.Sprintf("%s\n", data.Address))
	}
	if data.Phone != "" {
		sb.WriteString(fmt.Sprintf("Telp: %s\n", data.Phone))
	}
	sb.WriteString("--------------------------------\n")
	sb.WriteString(fmt.Sprintf("*No. Nota :* %s\n", data.InvoiceNo))
	sb.WriteString(fmt.Sprintf("*Tanggal  :* %s\n", data.Date.Format("02/01/2006 15:04 WIB")))
	if data.CustomerName != "" {
		sb.WriteString(fmt.Sprintf("*Pelanggan:* %s\n", data.CustomerName))
	}
	sb.WriteString("--------------------------------\n")
	sb.WriteString("```\n")

	var totalItemDiscount float64
	var grossTotal float64

	for _, it := range data.Items {
		sb.WriteString(fmt.Sprintf("%s\n", it.ProductName))
		var priceDetail string
		if it.IsOpenPrice {
			priceDetail = fmt.Sprintf("  %d %s", it.Qty, it.UnitName)
		} else {
			priceDetail = fmt.Sprintf("  %d %s x %s", it.Qty, it.UnitName, formatNumberDot(it.Price))
		}
		itemGross := it.Subtotal + it.Discount
		grossTotal += itemGross
		totalItemDiscount += it.Discount

		subStr := formatNumberDot(itemGross)
		pad := 32 - len(priceDetail) - len(subStr)
		if pad < 1 {
			pad = 1
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", priceDetail, strings.Repeat(" ", pad), subStr))

		if it.Discount > 0 {
			discDetail := "  (Diskon item)"
			discStr := fmt.Sprintf("-%s", formatNumberDot(it.Discount))
			discPad := 32 - len(discDetail) - len(discStr)
			if discPad < 1 {
				discPad = 1
			}
			sb.WriteString(fmt.Sprintf("%s%s%s\n", discDetail, strings.Repeat(" ", discPad), discStr))
		}
	}

	totalDiscount := totalItemDiscount + data.DiscountNota

	sb.WriteString("--------------------------------\n")
	if totalDiscount > 0 {
		subLabel := "Subtotal:"
		subVal := formatNumberDot(grossTotal)
		padSub := 32 - len(subLabel) - len(subVal)
		if padSub < 1 {
			padSub = 1
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", subLabel, strings.Repeat(" ", padSub), subVal))

		if totalItemDiscount > 0 && data.DiscountNota > 0 {
			discItemLabel := "Diskon Item:"
			discItemVal := fmt.Sprintf("-%s", formatNumberDot(totalItemDiscount))
			padItem := 32 - len(discItemLabel) - len(discItemVal)
			if padItem < 1 {
				padItem = 1
			}
			sb.WriteString(fmt.Sprintf("%s%s%s\n", discItemLabel, strings.Repeat(" ", padItem), discItemVal))

			discNotaLabel := "Diskon Nota:"
			discNotaVal := fmt.Sprintf("-%s", formatNumberDot(data.DiscountNota))
			padNota := 32 - len(discNotaLabel) - len(discNotaVal)
			if padNota < 1 {
				padNota = 1
			}
			sb.WriteString(fmt.Sprintf("%s%s%s\n", discNotaLabel, strings.Repeat(" ", padNota), discNotaVal))

			totDiscLabel := "Total Diskon:"
			totDiscVal := fmt.Sprintf("-%s", formatNumberDot(totalDiscount))
			padTotDisc := 32 - len(totDiscLabel) - len(totDiscVal)
			if padTotDisc < 1 {
				padTotDisc = 1
			}
			sb.WriteString(fmt.Sprintf("%s%s%s\n", totDiscLabel, strings.Repeat(" ", padTotDisc), totDiscVal))
		} else {
			totDiscLabel := "Total Diskon:"
			totDiscVal := fmt.Sprintf("-%s", formatNumberDot(totalDiscount))
			padTotDisc := 32 - len(totDiscLabel) - len(totDiscVal)
			if padTotDisc < 1 {
				padTotDisc = 1
			}
			sb.WriteString(fmt.Sprintf("%s%s%s\n", totDiscLabel, strings.Repeat(" ", padTotDisc), totDiscVal))
		}
	}

	totLabel := "TOTAL:"
	totVal := fmt.Sprintf("Rp %s", formatNumberDot(data.TotalAmount))
	padTot := 32 - len(totLabel) - len(totVal)
	if padTot < 1 {
		padTot = 1
	}
	sb.WriteString(fmt.Sprintf("%s%s%s\n", totLabel, strings.Repeat(" ", padTot), totVal))

	if strings.ToLower(data.PaymentStatus) == "kasbon" {
		if data.PaidAmount > 0 {
			dpLabel := "Uang Muka (DP):"
			dpVal := formatNumberDot(data.PaidAmount)
			padDP := 32 - len(dpLabel) - len(dpVal)
			if padDP < 1 {
				padDP = 1
			}
			sb.WriteString(fmt.Sprintf("%s%s%s\n", dpLabel, strings.Repeat(" ", padDP), dpVal))
		}
		sisa := data.TotalAmount - data.PaidAmount
		if sisa < 0 {
			sisa = 0
		}
		sisaLabel := "SISA KURANG:"
		sisaVal := fmt.Sprintf("Rp %s", formatNumberDot(sisa))
		padSisa := 32 - len(sisaLabel) - len(sisaVal)
		if padSisa < 1 {
			padSisa = 1
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", sisaLabel, strings.Repeat(" ", padSisa), sisaVal))

		if data.DueDate != nil {
			tempoLabel := "Jatuh Tempo:"
			tempoVal := data.DueDate.Format("02/01/2006")
			padTempo := 32 - len(tempoLabel) - len(tempoVal)
			if padTempo < 1 {
				padTempo = 1
			}
			sb.WriteString(fmt.Sprintf("%s%s%s\n", tempoLabel, strings.Repeat(" ", padTempo), tempoVal))
		}
	} else {
		bayarLabel := fmt.Sprintf("Bayar (%s):", strings.ToUpper(data.PaymentMethod))
		bayarVal := formatNumberDot(data.PaidAmount)
		padBayar := 32 - len(bayarLabel) - len(bayarVal)
		if padBayar < 1 {
			padBayar = 1
		}
		sb.WriteString(fmt.Sprintf("%s%s%s\n", bayarLabel, strings.Repeat(" ", padBayar), bayarVal))

		if strings.ToLower(data.PaymentMethod) == "tunai" {
			kembaliLabel := "Kembali:"
			kembaliVal := formatNumberDot(data.ChangeAmount)
			padKembali := 32 - len(kembaliLabel) - len(kembaliVal)
			if padKembali < 1 {
				padKembali = 1
			}
			sb.WriteString(fmt.Sprintf("%s%s%s\n", kembaliLabel, strings.Repeat(" ", padKembali), kembaliVal))
		}
	}

	sb.WriteString("```\n")
	sb.WriteString("--------------------------------\n")
	if strings.ToLower(data.PaymentStatus) == "kasbon" {
		sisa := data.TotalAmount - data.PaidAmount
		if sisa > 0 {
			if data.DueDate != nil {
				sb.WriteString(fmt.Sprintf("📌 *Informasi Kasbon:*\nSisa tagihan yang harus dibayar adalah *Rp %s* dengan batas jatuh tempo pada *%s*.\n--------------------------------\n", formatNumberDot(sisa), data.DueDate.Format("02/01/2006")))
			} else {
				sb.WriteString(fmt.Sprintf("📌 *Informasi Kasbon:*\nSisa tagihan yang harus dibayar adalah *Rp %s*.\n--------------------------------\n", formatNumberDot(sisa)))
			}
		}
	}
	if data.ReceiptFooter != "" {
		sb.WriteString(fmt.Sprintf("%s\n", data.ReceiptFooter))
	} else {
		sb.WriteString("Terima kasih atas kunjungan Anda!\n")
	}

	return sb.String()
}

func formatNumberDot(val float64) string {
	s := fmt.Sprintf("%d", int64(val+0.5))
	var result []string
	for len(s) > 3 {
		result = append([]string{s[len(s)-3:]}, result...)
		s = s[:len(s)-3]
	}
	result = append([]string{s}, result...)
	return strings.Join(result, ".")
}

type PurchaseOrderData struct {
	CompanyName   string
	StoreAddress  string
	StorePhone    string
	InvoiceNumber string
	Date          time.Time
	SupplierName  string
	SupplierPhone string
	Items         []PurchaseOrderItem
	TotalCost     float64
	Notes         string
}

type PurchaseOrderItem struct {
	ProductName string
	UnitName    string
	Qty         int32
	Cost        float64
	Subtotal    float64
}

func FormatPurchaseOrderText(data PurchaseOrderData) string {
	var sb strings.Builder

	storeName := data.CompanyName
	if storeName == "" {
		storeName = "POS TOKO"
	}

	sb.WriteString(fmt.Sprintf("Permisi, kami dari *%s* ingin memesan barang dengan rincian Purchase Order berikut:\n\n", storeName))
	sb.WriteString("📦 *SURAT PESANAN BARANG (PURCHASE ORDER)*\n")
	sb.WriteString(fmt.Sprintf("• *No. PO   :* %s\n", data.InvoiceNumber))
	sb.WriteString(fmt.Sprintf("• *Tanggal  :* %s\n", data.Date.Format("02/01/2006 15:04 WIB")))
	if data.StorePhone != "" {
		sb.WriteString(fmt.Sprintf("• *Kontak   :* %s\n", data.StorePhone))
	}
	sb.WriteString("--------------------------------\n")
	sb.WriteString("*Daftar Barang yang Dipesan:*\n")

	for i, it := range data.Items {
		sb.WriteString(fmt.Sprintf("%d. *%s*\n   ➔ Jumlah: *%d %s*\n", i+1, it.ProductName, it.Qty, it.UnitName))
	}

	sb.WriteString("--------------------------------\n")
	if strings.TrimSpace(data.Notes) != "" {
		sb.WriteString(fmt.Sprintf("*Catatan:*\n%s\n\n", strings.TrimSpace(data.Notes)))
	}
	sb.WriteString("Mohon konfirmasi ketersediaan barang dan estimasi pengirimannya jika berkenan. Terima kasih banyak atas kerja sama dan bantuannya! 🙏\n")

	return sb.String()
}


