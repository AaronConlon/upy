package ui

import (
	"testing"
	"time"
)

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name string
		d    time.Duration
		want string
	}{
		{"0s", 0, "0.0s"},
		{"negative", -5 * time.Second, "0.0s"},
		{"subsecond", 450 * time.Millisecond, "0.5s"},
		{"3.2s", 3200 * time.Millisecond, "3.2s"},
		{"59.4s", 59400 * time.Millisecond, "59.4s"},
		{"exactly 60s", 60 * time.Second, "1m"},
		{"65s", 65 * time.Second, "1m5s"},
		{"75s", 75 * time.Second, "1m15s"},
		{"120s", 120 * time.Second, "2m"},
		{"3599s", 3599 * time.Second, "59m59s"},
		{"exactly 1h", 3600 * time.Second, "1h"},
		{"1h2m", 3720 * time.Second, "1h2m"},
		{"1h2m5s", 3725 * time.Second, "1h2m5s"},
		{"2h", 7200 * time.Second, "2h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDuration(tt.d)
			if got != tt.want {
				t.Errorf("FormatDuration(%v) = %q, want %q", tt.d, got, tt.want)
			}
		})
	}
}
