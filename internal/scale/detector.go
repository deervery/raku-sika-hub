package scale

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DetectPort searches for an FTDI serial device matching the given VID and PID
// on Linux by reading /sys/bus/usb-serial/devices/.
func DetectPort(vid, pid string) (string, error) {
	basePath := "/sys/bus/usb-serial/devices"
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return "", fmt.Errorf("FTDI_NOT_FOUND: /sys/bus/usb-serial/devices を読み取れません: %w", err)
	}

	vid = strings.ToLower(vid)
	pid = strings.ToLower(pid)

	for _, e := range entries {
		devName := e.Name() // e.g. "ttyUSB0"
		// Resolve symlink to find the USB device path
		linkPath := filepath.Join(basePath, devName)
		realPath, err := filepath.EvalSymlinks(linkPath)
		if err != nil {
			continue
		}

		// Walk up to find the USB device directory containing idVendor/idProduct
		usbDir := findUSBDeviceDir(realPath)
		if usbDir == "" {
			continue
		}

		devVID := readSysFile(filepath.Join(usbDir, "idVendor"))
		devPID := readSysFile(filepath.Join(usbDir, "idProduct"))

		if strings.ToLower(devVID) == vid && strings.ToLower(devPID) == pid {
			return "/dev/" + devName, nil
		}
	}

	return "", fmt.Errorf("FTDI_NOT_FOUND: デバイスが見つかりません (VID:%s PID:%s)", vid, pid)
}

// findUSBDeviceDir walks up the directory tree to find a USB device dir
// (one that contains idVendor and idProduct files).
func findUSBDeviceDir(path string) string {
	for {
		if _, err := os.Stat(filepath.Join(path, "idVendor")); err == nil {
			return path
		}
		parent := filepath.Dir(path)
		if parent == path {
			return ""
		}
		path = parent
	}
}

func readSysFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}
