package scale

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

// DetectPort searches for an FTDI serial device matching the given VID and PID.
// It tries multiple detection methods in order of reliability:
//  1. /sys/bus/usb-serial/devices (standard Linux sysfs)
//  2. /dev/serial/by-id (udev symlinks, reliable on Raspberry Pi)
//  3. /sys/class/tty/ttyUSB* (broad fallback)
func DetectPort(vid, pid string, logger *logging.Logger) (string, error) {
	vid = strings.ToLower(vid)
	pid = strings.ToLower(pid)

	// Method 1: /sys/bus/usb-serial/devices
	if port, err := detectViaSysBus(vid, pid); err == nil {
		logger.Info("FTDI検出: /sys/bus/usb-serial/devices で %s を発見", port)
		return port, nil
	} else {
		logger.Info("FTDI検出: /sys/bus/usb-serial/devices 失敗: %v", err)
	}

	// Method 2: /dev/serial/by-id (udev symlinks)
	if port, err := detectViaDevSerial(vid, pid); err == nil {
		logger.Info("FTDI検出: /dev/serial/by-id で %s を発見", port)
		return port, nil
	} else {
		logger.Info("FTDI検出: /dev/serial/by-id 失敗: %v", err)
	}

	// Method 3: /sys/class/tty/ttyUSB*
	if port, err := detectViaSysClassTty(vid, pid); err == nil {
		logger.Info("FTDI検出: /sys/class/tty で %s を発見", port)
		return port, nil
	} else {
		logger.Info("FTDI検出: /sys/class/tty 失敗: %v", err)
	}

	return "", fmt.Errorf("FTDI_NOT_FOUND: デバイスが見つかりません (VID:%s PID:%s)。全検出パス失敗", vid, pid)
}

// detectViaSysBus searches /sys/bus/usb-serial/devices/ symlinks.
func detectViaSysBus(vid, pid string) (string, error) {
	basePath := "/sys/bus/usb-serial/devices"
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return "", fmt.Errorf("/sys/bus/usb-serial/devices を読み取れません: %w", err)
	}

	for _, e := range entries {
		devName := e.Name()
		linkPath := filepath.Join(basePath, devName)
		realPath, err := filepath.EvalSymlinks(linkPath)
		if err != nil {
			continue
		}

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

	return "", fmt.Errorf("一致するデバイスなし")
}

// detectViaDevSerial searches /dev/serial/by-id/ for FTDI devices.
// udev creates symlinks like: usb-FTDI_FT232R_USB_UART_XXXXXXXX-if00-port0 -> ../../ttyUSB0
func detectViaDevSerial(vid, pid string) (string, error) {
	basePath := "/dev/serial/by-id"
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return "", fmt.Errorf("/dev/serial/by-id を読み取れません: %w", err)
	}

	// FTDI devices typically have "FTDI" or the VID in the symlink name
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if !strings.Contains(name, "ftdi") && !strings.Contains(name, vid) {
			continue
		}

		linkPath := filepath.Join(basePath, e.Name())
		realPath, err := filepath.EvalSymlinks(linkPath)
		if err != nil {
			continue
		}

		// Verify VID/PID via sysfs
		devName := filepath.Base(realPath)
		sysPath := "/sys/class/tty/" + devName + "/device"
		usbDir := findUSBDeviceDir(sysPath)
		if usbDir != "" {
			devVID := readSysFile(filepath.Join(usbDir, "idVendor"))
			devPID := readSysFile(filepath.Join(usbDir, "idProduct"))
			if strings.ToLower(devVID) == vid && strings.ToLower(devPID) == pid {
				return realPath, nil
			}
		}

		// If sysfs check fails but name matched FTDI, still return it
		if strings.Contains(name, "ftdi") {
			return realPath, nil
		}
	}

	return "", fmt.Errorf("一致するデバイスなし")
}

// detectViaSysClassTty scans /sys/class/tty/ttyUSB* for matching VID/PID.
func detectViaSysClassTty(vid, pid string) (string, error) {
	matches, err := filepath.Glob("/sys/class/tty/ttyUSB*")
	if err != nil {
		return "", fmt.Errorf("/sys/class/tty glob失敗: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("ttyUSB* デバイスなし")
	}

	for _, ttyPath := range matches {
		devName := filepath.Base(ttyPath)
		devicePath := filepath.Join(ttyPath, "device")
		usbDir := findUSBDeviceDir(devicePath)
		if usbDir == "" {
			continue
		}

		devVID := readSysFile(filepath.Join(usbDir, "idVendor"))
		devPID := readSysFile(filepath.Join(usbDir, "idProduct"))

		if strings.ToLower(devVID) == vid && strings.ToLower(devPID) == pid {
			return "/dev/" + devName, nil
		}
	}

	return "", fmt.Errorf("一致するVID/PIDなし")
}

// findUSBDeviceDir walks up the directory tree to find a USB device dir
// (one that contains idVendor and idProduct files).
func findUSBDeviceDir(path string) string {
	// Resolve symlinks first
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}

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
