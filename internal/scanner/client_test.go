package scanner

import (
	"testing"
	"time"
)

func TestConsume_Empty(t *testing.T) {
	c := &Client{}
	c.devicePath.Store("")
	_, _, ok := c.Consume()
	if ok {
		t.Error("expected ok=false for empty scanner")
	}
}

func TestConsume_ReturnsAndClears(t *testing.T) {
	c := &Client{}
	c.devicePath.Store("")

	// Simulate a scan.
	c.latest = &scanResult{
		value:     "https://example.com/t/abc123",
		scannedAt: time.Date(2026, 3, 17, 5, 30, 0, 0, time.UTC),
	}

	value, scannedAt, ok := c.Consume()
	if !ok {
		t.Fatal("expected ok=true")
	}
	if value != "https://example.com/t/abc123" {
		t.Errorf("value = %q, want %q", value, "https://example.com/t/abc123")
	}
	if scannedAt == "" {
		t.Error("scannedAt should not be empty")
	}

	// Second call should return empty (consumed).
	_, _, ok = c.Consume()
	if ok {
		t.Error("expected ok=false after consume")
	}
}

func TestConnected_Default(t *testing.T) {
	c := &Client{}
	c.devicePath.Store("")
	if c.Connected() {
		t.Error("expected not connected by default")
	}
}

func TestDevicePath_Default(t *testing.T) {
	c := &Client{}
	c.devicePath.Store("")
	if c.DevicePath() != "" {
		t.Error("expected empty device path by default")
	}
}

func TestKeyMap_Coverage(t *testing.T) {
	// Verify basic mappings exist.
	tests := []struct {
		code    uint16
		shifted bool
		want    byte
	}{
		{30, false, 'a'},
		{30, true, 'A'},
		{2, false, '1'},
		{2, true, '!'},
		{57, false, ' '},
		{52, false, '.'},
		{53, false, '/'},
		{53, true, '?'},
	}
	for _, tt := range tests {
		var got byte
		if tt.shifted {
			got = shiftKeyMap[tt.code]
		} else {
			got = keyMap[tt.code]
		}
		if got != tt.want {
			t.Errorf("keyMap[%d] (shift=%v) = %q, want %q", tt.code, tt.shifted, got, tt.want)
		}
	}
}
