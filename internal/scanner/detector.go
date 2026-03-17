package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DetectDevice searches /sys/class/input/ for a matching HID barcode reader.
// It matches by device name pattern first, then falls back to VID/PID matching.
// Returns the /dev/input/eventN path on success.
func DetectDevice(deviceName, vid, pid string) (string, error) {
	// List all event devices.
	matches, err := filepath.Glob("/sys/class/input/event*/device/name")
	if err != nil {
		return "", fmt.Errorf("glob /sys/class/input: %w", err)
	}

	for _, namePath := range matches {
		nameBytes, err := os.ReadFile(namePath)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(nameBytes))

		// Extract eventN from path.
		// namePath is like /sys/class/input/event3/device/name
		parts := strings.Split(namePath, "/")
		if len(parts) < 5 {
			continue
		}
		eventName := parts[4] // "event3"

		// Match by device name if specified.
		if deviceName != "" && strings.Contains(name, deviceName) {
			return "/dev/input/" + eventName, nil
		}

		// Match by VID/PID if specified.
		if vid != "" && pid != "" {
			if matchVIDPID(namePath, vid, pid) {
				return "/dev/input/" + eventName, nil
			}
		}
	}

	return "", fmt.Errorf("barcode scanner not found (name=%q vid=%q pid=%q)", deviceName, vid, pid)
}

// matchVIDPID checks if the device at the given sys path matches the given VID/PID.
func matchVIDPID(namePath, vid, pid string) bool {
	// Navigate from /sys/class/input/eventN/device/name to the device directory
	// and look for a uevent file with PRODUCT info.
	deviceDir := filepath.Dir(namePath) // .../eventN/device/
	ueventPath := filepath.Join(deviceDir, "id", "vendor")

	vendorBytes, err := os.ReadFile(ueventPath)
	if err != nil {
		return false
	}
	productBytes, err := os.ReadFile(filepath.Join(deviceDir, "id", "product"))
	if err != nil {
		return false
	}

	vendorID := strings.TrimSpace(string(vendorBytes))
	productID := strings.TrimSpace(string(productBytes))

	return strings.EqualFold(vendorID, vid) && strings.EqualFold(productID, pid)
}
