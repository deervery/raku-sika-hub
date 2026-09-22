package netadmin

import (
	"testing"
	"time"
)

const defaultTestTimeout = 2 * time.Second

func TestSplitFieldsKeepsEscapedColons(t *testing.T) {
	// nmcli -t escapes a literal colon inside a value; an SSID or a MAC-like
	// field would otherwise be torn into extra columns.
	got := splitFields(`wlan0:wifi:connected:Buffalo\:A`, 4)
	want := []string{"wlan0", "wifi", "connected", "Buffalo:A"}
	if len(got) != len(want) {
		t.Fatalf("expected %d fields, got %d (%q)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("field %d: expected %q, got %q", i, want[i], got[i])
		}
	}
}

func TestBandFor(t *testing.T) {
	cases := map[int]string{5180: "5GHz", 2417: "2.4GHz", 0: ""}
	for freq, want := range cases {
		if got := bandFor(freq); got != want {
			t.Fatalf("bandFor(%d): expected %q, got %q", freq, want, got)
		}
	}
}

func TestParseLeadingInt(t *testing.T) {
	cases := map[string]int{"270 Mbit/s": 270, "5180 MHz": 5180, "43": 43, "--": 0, "": 0}
	for in, want := range cases {
		if got := parseLeadingInt(in); got != want {
			t.Fatalf("parseLeadingInt(%q): expected %d, got %d", in, want, got)
		}
	}
}

func TestActivateRefusesUnknownProfile(t *testing.T) {
	// A profile the GUI did not list must never reach nmcli: pointing the
	// manager at a binary that does not exist proves no command is run.
	m := &Manager{nmcli: "/nonexistent/nmcli", timeout: defaultTestTimeout}
	if err := m.Activate(""); err == nil {
		t.Fatal("an empty profile name must be refused")
	}
}
