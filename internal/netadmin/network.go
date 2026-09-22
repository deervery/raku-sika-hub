// Package netadmin exposes the Pi's Wi-Fi state, and switching between the
// Wi-Fi profiles NetworkManager already knows about, to the hub admin GUI.
//
// It deliberately cannot create or edit a profile: the GUI activates one of the
// connections an operator (or ops/setup) has already defined. That keeps the
// sudoers grant narrow and means a mistake in the browser cannot leave the Pi
// with credentials nobody has.
package netadmin

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// Manager reads and switches Wi-Fi state via nmcli.
type Manager struct {
	// nmcli is the binary to invoke; overridden in tests.
	nmcli string
	// timeout bounds a single nmcli call. Activation on a weak link can take
	// several seconds, so it is generous.
	timeout time.Duration
}

// NewManager returns a Manager driving the system nmcli.
func NewManager() *Manager {
	return &Manager{nmcli: "nmcli", timeout: 45 * time.Second}
}

// Device is one Wi-Fi interface.
type Device struct {
	Name string `json:"name"`
	// State is NetworkManager's device state ("connected", "disconnected").
	State string `json:"state"`
	// Connection is the active profile name, empty when disconnected.
	Connection string `json:"connection"`
	// SSID / Band / SignalPercent describe the current association.
	SSID          string `json:"ssid,omitempty"`
	FreqMHz       int    `json:"freqMhz,omitempty"`
	Band          string `json:"band,omitempty"`
	SignalPercent int    `json:"signalPercent,omitempty"`
	// RateMbps is the AP's advertised rate for the associated BSS.
	RateMbps int `json:"rateMbps,omitempty"`
}

// Profile is a saved Wi-Fi connection the GUI may activate.
type Profile struct {
	Name string `json:"name"`
	SSID string `json:"ssid,omitempty"`
	// Band is "a" (5GHz), "bg" (2.4GHz) or "" (unlocked) as stored on the
	// profile, not as currently associated.
	Band     string `json:"band,omitempty"`
	Device   string `json:"device,omitempty"`
	Active   bool   `json:"active"`
	Priority int    `json:"priority"`
}

// Status is the whole Wi-Fi picture the GUI renders.
type Status struct {
	Devices  []Device  `json:"devices"`
	Profiles []Profile `json:"profiles"`
	// Warning carries an operator-facing note, e.g. that the USB adapter the
	// site relies on is not enumerated.
	Warning string `json:"warning,omitempty"`
}

func (m *Manager) run(args ...string) (string, error) {
	cmd := exec.Command(m.nmcli, args...)
	done := make(chan struct{})
	var out []byte
	var err error
	go func() {
		out, err = cmd.CombinedOutput()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(m.timeout):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("nmcli %s: タイムアウトしました", strings.Join(args, " "))
	}
	if err != nil {
		return string(out), fmt.Errorf("nmcli %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// Status reports the Wi-Fi devices and the profiles available to them.
func (m *Manager) Status() (Status, error) {
	var st Status

	devOut, err := m.run("-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device")
	if err != nil {
		return st, err
	}
	for _, line := range splitLines(devOut) {
		f := splitFields(line, 4)
		if len(f) < 4 || f[1] != "wifi" {
			continue
		}
		st.Devices = append(st.Devices, Device{Name: f[0], State: f[2], Connection: f[3]})
	}

	// Fill the association details per device from the scan list, whose
	// in-use row is the BSS we are on.
	for i, dev := range st.Devices {
		listOut, err := m.run("-t", "-f", "IN-USE,SSID,FREQ,SIGNAL,RATE", "device", "wifi", "list",
			"ifname", dev.Name, "--rescan", "no")
		if err != nil {
			continue
		}
		for _, line := range splitLines(listOut) {
			f := splitFields(line, 5)
			if len(f) < 5 || strings.TrimSpace(f[0]) != "*" {
				continue
			}
			st.Devices[i].SSID = f[1]
			st.Devices[i].FreqMHz = parseLeadingInt(f[2])
			st.Devices[i].Band = bandFor(st.Devices[i].FreqMHz)
			st.Devices[i].SignalPercent = parseLeadingInt(f[3])
			st.Devices[i].RateMbps = parseLeadingInt(f[4])
			break
		}
	}

	conOut, err := m.run("-t", "-f", "NAME,TYPE,DEVICE", "connection", "show")
	if err != nil {
		return st, err
	}
	for _, line := range splitLines(conOut) {
		f := splitFields(line, 3)
		if len(f) < 3 || !strings.Contains(f[1], "wireless") {
			continue
		}
		p := Profile{Name: f[0], Device: f[2], Active: f[2] != "" && f[2] != "--"}
		p.SSID, p.Band, p.Priority = m.profileDetail(f[0])
		st.Profiles = append(st.Profiles, p)
	}

	if len(st.Devices) < 2 {
		st.Warning = "Wi-Fi インターフェースが 1 つだけです。USB 無線アダプタが認識されていない可能性があります。"
	}
	return st, nil
}

func (m *Manager) profileDetail(name string) (ssid, band string, priority int) {
	out, err := m.run("-t", "-f",
		"802-11-wireless.ssid,802-11-wireless.band,connection.autoconnect-priority",
		"connection", "show", name)
	if err != nil {
		return "", "", 0
	}
	for _, line := range splitLines(out) {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if value == "--" {
			value = ""
		}
		switch key {
		case "802-11-wireless.ssid":
			ssid = value
		case "802-11-wireless.band":
			band = value
		case "connection.autoconnect-priority":
			priority = parseLeadingInt(value)
		}
	}
	return ssid, band, priority
}

// Activate brings up a saved profile. The profile must already exist; an
// unknown name is refused rather than created.
func (m *Manager) Activate(profile string) error {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return fmt.Errorf("接続プロファイル名が指定されていません")
	}
	st, err := m.Status()
	if err != nil {
		return err
	}
	known := false
	for _, p := range st.Profiles {
		if p.Name == profile {
			known = true
			break
		}
	}
	if !known {
		return fmt.Errorf("接続プロファイル %q は登録されていません", profile)
	}
	if _, err := m.run("connection", "up", profile); err != nil {
		return err
	}
	return nil
}

func bandFor(freqMHz int) string {
	switch {
	case freqMHz >= 4900:
		return "5GHz"
	case freqMHz > 0:
		return "2.4GHz"
	default:
		return ""
	}
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// splitFields splits an nmcli terse line on unescaped colons. nmcli escapes a
// literal colon in a value as "\:", which a plain Split would tear apart —
// SSIDs and MAC-like values hit this.
func splitFields(line string, n int) []string {
	fields := make([]string, 0, n)
	var cur strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == ':':
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	fields = append(fields, cur.String())
	return fields
}

func parseLeadingInt(s string) int {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0
	}
	v, err := strconv.Atoi(s[:end])
	if err != nil {
		return 0
	}
	return v
}
