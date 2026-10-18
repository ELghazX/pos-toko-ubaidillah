package utils

import (
	"testing"
	"time"
)

func TestDateFilter_RecentTransaction(t *testing.T) {
	now := time.Now()
	saleTimeUTC := now.UTC()
	saleTimeLocal := now.Local()

	for _, saleTime := range []time.Time{saleTimeUTC, saleTimeLocal} {
		start30, end30, hasFilter30 := ParseDateFilter("30_days", "", "")
		if !hasFilter30 {
			t.Errorf("Expected hasFilter30 = true")
		}
		if !InDateRange(saleTime, start30, end30) {
			t.Errorf("Expected saleTime (%v) to be in range for 30_days [%v, %v]", saleTime, start30, end30)
		}

		startToday, endToday, hasFilterToday := ParseDateFilter("today", "", "")
		if !hasFilterToday {
			t.Errorf("Expected hasFilterToday = true")
		}
		if !InDateRange(saleTime, startToday, endToday) {
			t.Errorf("Expected saleTime (%v) to be in range for today [%v, %v]", saleTime, startToday, endToday)
		}

		startMonth, endMonth, hasFilterMonth := ParseDateFilter("this_month", "", "")
		if !hasFilterMonth {
			t.Errorf("Expected hasFilterMonth = true")
		}
		if !InDateRange(saleTime, startMonth, endMonth) {
			t.Errorf("Expected saleTime (%v) to be in range for this_month [%v, %v]", saleTime, startMonth, endMonth)
		}

		todayStr := now.Format("2006-01-02")
		startCustom, endCustom, hasFilterCustom := ParseDateFilter("custom", todayStr, todayStr)
		if !hasFilterCustom {
			t.Errorf("Expected hasFilterCustom = true")
		}
		if !InDateRange(saleTime, startCustom, endCustom) {
			t.Errorf("Expected saleTime (%v) to be in range for custom [%v, %v]", saleTime, startCustom, endCustom)
		}
	}
}

func TestDateFilter_OlderTransaction(t *testing.T) {
	now := time.Now()
	oldSale := now.AddDate(0, 0, -45).UTC()

	start30, end30, _ := ParseDateFilter("30_days", "", "")
	if InDateRange(oldSale, start30, end30) {
		t.Errorf("Expected oldSale (%v) to NOT be in range for 30_days [%v, %v]", oldSale, start30, end30)
	}

	startToday, endToday, _ := ParseDateFilter("today", "", "")
	if InDateRange(oldSale, startToday, endToday) {
		t.Errorf("Expected oldSale (%v) to NOT be in range for today [%v, %v]", oldSale, startToday, endToday)
	}
}
