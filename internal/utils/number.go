package utils

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

func FloatToNumeric(val float64) pgtype.Numeric {
	var n pgtype.Numeric
	n.Scan(fmt.Sprintf("%.2f", val))
	return n
}

func NumericToFloat(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

// FormatRupiah formats pgtype.Numeric with thousand dots (e.g., 15000 -> "15.000")
func FormatRupiah(n pgtype.Numeric) string {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return "0"
	}
	val := int64(f.Float64 + 0.5)
	s := fmt.Sprintf("%d", val)
	var result []string
	for len(s) > 3 {
		result = append([]string{s[len(s)-3:]}, result...)
		s = s[:len(s)-3]
	}
	result = append([]string{s}, result...)
	return strings.Join(result, ".")
}

// FormatPlainFloat formats pgtype.Numeric as an unformatted integer string for <input type="number"> values
func FormatPlainFloat(n pgtype.Numeric) string {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return "0"
	}
	return fmt.Sprintf("%.0f", f.Float64)
}

// FormatFloatRupiah formats float64 with thousand dots (e.g., 15000 -> "15.000")
func FormatFloatRupiah(val float64) string {
	s := fmt.Sprintf("%d", int64(val+0.5))
	var result []string
	for len(s) > 3 {
		result = append([]string{s[len(s)-3:]}, result...)
		s = s[:len(s)-3]
	}
	result = append([]string{s}, result...)
	return strings.Join(result, ".")
}

