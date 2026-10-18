package whatsapp

import (
	"strings"
	"testing"
	"time"
)

func TestFormatReceiptText_WithItemDiscount(t *testing.T) {
	data := SaleReceiptData{
		CompanyName:   "ELGHAZ STORE",
		Tagline:       "Khusus menjual ke wibu",
		Address:       "Jalan Wahid Hasyim 2",
		Phone:         "0813-4801-4187",
		InvoiceNo:     "TRX-20261017-0022",
		Date:          time.Date(2026, 10, 17, 0, 11, 0, 0, time.UTC),
		CustomerName:  "Muhammad Indra",
		PaymentMethod: "tunai",
		PaymentStatus: "lunas",
		TotalAmount:   2000,
		PaidAmount:    2000,
		ChangeAmount:  0,
		Items: []SaleReceiptItem{
			{
				ProductName: "Fotocopy",
				UnitName:    "Lembar",
				Qty:         5,
				Price:       400,
				Discount:    1000,
				Subtotal:    2000,
				IsOpenPrice: true,
			},
		},
	}

	receipt := FormatReceiptText(data)

	if !strings.Contains(receipt, "5 Lembar                 3.000") {
		t.Errorf("Expected gross subtotal 3.000 on item line, got receipt:\n%s", receipt)
	}

	if !strings.Contains(receipt, "(Diskon item)           -1.000") {
		t.Errorf("Expected discount line (Diskon item) -1.000, got receipt:\n%s", receipt)
	}

	if !strings.Contains(receipt, "Subtotal:                  3.000") {
		t.Errorf("Expected Subtotal 3.000, got receipt:\n%s", receipt)
	}
	if !strings.Contains(receipt, "Total Diskon:             -1.000") {
		t.Errorf("Expected Total Diskon -1.000, got receipt:\n%s", receipt)
	}
	if !strings.Contains(receipt, "TOTAL:                  Rp 2.000") {
		t.Errorf("Expected TOTAL Rp 2.000, got receipt:\n%s", receipt)
	}
}

func TestFormatReceiptText_NoDiscount(t *testing.T) {
	data := SaleReceiptData{
		CompanyName:   "ELGHAZ STORE",
		InvoiceNo:     "TRX-20261017-0023",
		Date:          time.Now(),
		PaymentMethod: "tunai",
		PaymentStatus: "lunas",
		TotalAmount:   3000,
		PaidAmount:    5000,
		ChangeAmount:  2000,
		Items: []SaleReceiptItem{
			{
				ProductName: "Fotocopy",
				UnitName:    "Lembar",
				Qty:         5,
				Price:       600,
				Discount:    0,
				Subtotal:    3000,
				IsOpenPrice: true,
			},
		},
	}

	receipt := FormatReceiptText(data)

	if strings.Contains(receipt, "Total Diskon:") {
		t.Errorf("Expected no Total Diskon line when discount is 0, got receipt:\n%s", receipt)
	}
	if strings.Contains(receipt, "Subtotal:") {
		t.Errorf("Expected no Subtotal line when discount is 0, got receipt:\n%s", receipt)
	}
	if !strings.Contains(receipt, "TOTAL:                  Rp 3.000") {
		t.Errorf("Expected TOTAL Rp 3.000, got receipt:\n%s", receipt)
	}
}

func TestFormatReceiptText_ItemAndNotaDiscount(t *testing.T) {
	data := SaleReceiptData{
		CompanyName:   "ELGHAZ STORE",
		InvoiceNo:     "TRX-20261017-0024",
		Date:          time.Now(),
		PaymentMethod: "tunai",
		PaymentStatus: "lunas",
		DiscountNota:  2000,
		TotalAmount:   10000,
		PaidAmount:    10000,
		ChangeAmount:  0,
		Items: []SaleReceiptItem{
			{
				ProductName: "Fotocopy",
				UnitName:    "Lembar",
				Qty:         5,
				Price:       400,
				Discount:    1000,
				Subtotal:    2000,
				IsOpenPrice: true,
			},
			{
				ProductName: "Kertas HVS",
				UnitName:    "Rim",
				Qty:         1,
				Price:       10000,
				Discount:    0,
				Subtotal:    10000,
				IsOpenPrice: false,
			},
		},
	}

	receipt := FormatReceiptText(data)

	if !strings.Contains(receipt, "Subtotal:                 13.000") {
		t.Errorf("Expected Subtotal 13.000, got receipt:\n%s", receipt)
	}
	if !strings.Contains(receipt, "Diskon Item:              -1.000") {
		t.Errorf("Expected Diskon Item -1.000, got receipt:\n%s", receipt)
	}
	if !strings.Contains(receipt, "Diskon Nota:              -2.000") {
		t.Errorf("Expected Diskon Nota -2.000, got receipt:\n%s", receipt)
	}
	if !strings.Contains(receipt, "Total Diskon:             -3.000") {
		t.Errorf("Expected Total Diskon -3.000, got receipt:\n%s", receipt)
	}
	if !strings.Contains(receipt, "TOTAL:                 Rp 10.000") {
		t.Errorf("Expected TOTAL Rp 10.000, got receipt:\n%s", receipt)
	}
}
