// Package conv holds small parsing helpers shared by handlers.
package conv

import (
	"fmt"
	"strconv"
	"time"
)

// ParseTime accepts RFC 3339 timestamps, date-only values and Unix seconds.
func ParseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(n, 0), nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format: %s", s)
}

// ParseDate accepts YYYY-MM-DD and returns a date-only time.
func ParseDate(s string) (time.Time, error) {
	return time.Parse("2006-01-02", s)
}

// Int returns the parsed integer or the fallback.
func Int(s string, fallback int) int {
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return fallback
}

// Bool returns the parsed boolean or the fallback.
func Bool(s string, fallback bool) bool {
	if b, err := strconv.ParseBool(s); err == nil {
		return b
	}
	return fallback
}

// Jakarta returns the WIB location, falling back to a fixed +07:00 zone when
// the tz database is unavailable (for example in a scratch container).
func Jakarta() *time.Location {
	if loc, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return loc
	}
	return time.FixedZone("WIB", 7*60*60)
}
