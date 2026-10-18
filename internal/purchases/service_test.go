package purchases

import (
	"math"
	"testing"
)

func TestCalculateReversedMovingAverage(t *testing.T) {
	// Case 1: 50 Pcs @ 1500 (75.000), received 50 Pcs @ 15.000 (750.000) -> avgCost = 8250, stock = 100.
	// Voiding the 50 Pcs @ 15.000 should restore avgCost to 1500.
	got := calculateReversedMovingAverage(100, 50, 8250, 750000, 0)
	if math.Abs(got-1500) > 0.01 {
		t.Errorf("Expected 1500, got %v", got)
	}

	// Case 2: New product (0 stock, 0 cost), received 20 Pcs @ 50.000 (1.000.000), then voided -> stock = 0, cost = 0.
	got2 := calculateReversedMovingAverage(20, 20, 50000, 1000000, 0)
	if got2 != 0 {
		t.Errorf("Expected 0 when newStock is 0 and no previous purchase, got %v", got2)
	}

	// Case 3: Product had 0 stock left, previous valid purchase was 2000, received 10 Pcs @ 20.000, then voided -> fallback to 2000.
	got3 := calculateReversedMovingAverage(10, 10, 20000, 200000, 2000)
	if got3 != 2000 {
		t.Errorf("Expected fallback 2000, got %v", got3)
	}
}
