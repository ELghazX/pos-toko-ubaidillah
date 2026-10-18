package utils

import "time"

func InDateRange(t, start, end time.Time) bool {
	if t.IsZero() {
		return false
	}
	if !start.IsZero() && t.Before(start) {
		return false
	}
	if !end.IsZero() && t.After(end) {
		return false
	}
	return true
}

func ParseDateFilter(period, fromStr, toStr string) (time.Time, time.Time, bool) {
	now := time.Now()
	loc := now.Location()
	endOfToday := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc)

	switch period {
	case "today":
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		return start, endOfToday, true
	case "this_month":
		start := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
		endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 999999999, loc)
		return start, endOfMonth, true
	case "all":
		return time.Time{}, time.Time{}, false
	case "custom":
		var start, end time.Time
		if fromStr != "" {
			t, err := time.ParseInLocation("2006-01-02", fromStr, loc)
			if err == nil {
				start = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
			}
		}
		if toStr != "" {
			t, err := time.ParseInLocation("2006-01-02", toStr, loc)
			if err == nil {
				end = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, loc)
			}
		} else {
			end = endOfToday
		}
		return start, end, true
	default:
		start := time.Date(now.Year(), now.Month(), now.Day()-30, 0, 0, 0, 0, loc)
		return start, endOfToday, true
	}
}
