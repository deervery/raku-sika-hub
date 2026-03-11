package scale

import (
	"math"
	"testing"
)

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 0.001
}

func TestParseWeighResponse_Stable(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		value  float64
		unit   string
		stable bool
	}{
		{"basic stable", "ST,+00012.34  kg\r\n", 12.34, "kg", true},
		{"zero value", "ST,+00000.00  kg\r\n", 0.0, "kg", true},
		{"negative value", "ST,-00001.00  kg\r\n", -1.0, "kg", true},
		{"large value", "ST,+00060.00  kg\r\n", 60.0, "kg", true},
		{"no leading zeros", "ST,+12.34  kg\r\n", 12.34, "kg", true},
		{"gram unit", "ST,+00012.34   g\r\n", 12.34, "g", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := ParseWeighResponse(tt.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.Header != HeaderST {
				t.Errorf("header = %q, want ST", r.Header)
			}
			if !almostEqual(r.Value, tt.value) {
				t.Errorf("value = %v, want %v", r.Value, tt.value)
			}
			if r.Unit != tt.unit {
				t.Errorf("unit = %q, want %q", r.Unit, tt.unit)
			}
			if r.Stable != tt.stable {
				t.Errorf("stable = %v, want %v", r.Stable, tt.stable)
			}
		})
	}
}

func TestParseWeighResponse_Unstable(t *testing.T) {
	r, err := ParseWeighResponse("US,+00012.30  kg\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Header != HeaderUS {
		t.Errorf("header = %q, want US", r.Header)
	}
	if !almostEqual(r.Value, 12.30) {
		t.Errorf("value = %v, want 12.30", r.Value)
	}
	if r.Stable {
		t.Error("expected unstable")
	}
}

func TestParseWeighResponse_Overload(t *testing.T) {
	r, err := ParseWeighResponse("OL\r\n")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Header != HeaderOL {
		t.Errorf("header = %q, want OL", r.Header)
	}
}

func TestParseWeighResponse_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"empty with crlf", "\r\n"},
		{"no comma", "GARBAGE\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseWeighResponse(tt.input)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestParseAckResponse(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		header ResponseHeader
		ok     bool
	}{
		{"tare QT", "QT\r\n", HeaderQT, true},
		{"tare TA", "TA\r\n", HeaderTA, true},
		{"zero ZR", "ZR\r\n", HeaderZR, true},
		{"tare with comma", "QT,\r\n", HeaderQT, true},
		{"not ack ST", "ST,+00012.34  kg\r\n", "", false},
		{"not ack random", "HELLO\r\n", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header, ok := ParseAckResponse(tt.input)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
			}
			if header != tt.header {
				t.Errorf("header = %q, want %q", header, tt.header)
			}
		})
	}
}
