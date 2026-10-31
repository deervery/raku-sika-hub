package qlbackend

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Scheme is the CUPS device URI scheme this backend serves.
const Scheme = "rakuql"

// DeviceURI is the URI raku-sika-ops gives the queue for a printer serial.
func DeviceURI(serial string) string {
	return Scheme + "://brother?serial=" + url.QueryEscape(serial)
}

// ResolveDevice turns a device URI into a /dev/usb/lp* path.
//
//	rakuql://brother?serial=000M5G736596  the usblp node of that printer
//	rakuql:///dev/usb/lp0                 that node, for manual tests
//
// Looking the printer up by serial keeps the queue valid when the kernel
// numbers the node differently after a replug.
func ResolveDevice(uri, sysRoot string) (string, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", fmt.Errorf("デバイス URI を読めません: %w", err)
	}
	if u.Scheme != Scheme {
		return "", fmt.Errorf("デバイス URI の種類が %q ではありません: %s", Scheme, uri)
	}
	if serial := u.Query().Get("serial"); serial != "" {
		return findBySerial(serial, sysRoot)
	}
	if strings.HasPrefix(u.Path, "/dev/") {
		return u.Path, nil
	}
	return "", fmt.Errorf("デバイス URI にシリアル番号もデバイスのパスもありません: %s", uri)
}

// findBySerial scans the usblp nodes for the printer with this serial number.
// Each /sys/class/usbmisc/lpN/device points at the USB interface; its parent
// directory is the USB device, which holds the serial.
func findBySerial(serial, sysRoot string) (string, error) {
	nodes, _ := filepath.Glob(filepath.Join(sysRoot, "sys/class/usbmisc/lp*"))
	for _, node := range nodes {
		iface, err := filepath.EvalSymlinks(filepath.Join(node, "device"))
		if err != nil {
			continue
		}
		got, err := os.ReadFile(filepath.Join(filepath.Dir(iface), "serial"))
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(got)) == serial {
			return filepath.Join(sysRoot, "dev/usb", filepath.Base(node)), nil
		}
	}
	return "", fmt.Errorf("プリンタ（シリアル %s）が USB に見つかりません。電源と USB ケーブルを確認してください。", serial)
}

// OpenDevice opens a usblp node non-blocking, so that reads can time out
// through the Go runtime's poller instead of hanging the backend.
func OpenDevice(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(fd), path), nil
}

// idleReadPause is how long to wait after a read that returned nothing.
const idleReadPause = 20 * time.Millisecond

// readFrames reads from the printer until r fails, sending each status frame
// it finds. Bytes that do not start a frame are skipped, so a partial read
// cannot shift every later frame.
//
// A read that returns nothing is not the end of the device: the QL-820NWB
// answers the host's IN requests with zero-length packets when it has nothing
// to say, which usblp hands up as a 0-byte read (io.EOF from *os.File). Only a
// real error — the device closed or unplugged — ends the loop.
func readFrames(r io.Reader, out chan<- Status) {
	defer close(out)
	var buf []byte
	chunk := make([]byte, 256)
	for {
		n, err := r.Read(chunk)
		if n == 0 && (errors.Is(err, io.EOF) || errors.Is(err, syscall.EAGAIN)) {
			time.Sleep(idleReadPause)
			continue
		}
		if n > 0 {
			buf = append(buf, chunk[:n]...)
			for {
				start := bytes.Index(buf, []byte{0x80, 0x20})
				if start < 0 {
					// Keep a trailing 0x80: it may be the first byte of a frame
					// whose second byte has not arrived yet.
					if len(buf) > 0 && buf[len(buf)-1] == 0x80 {
						buf = append(buf[:0], 0x80)
					} else {
						buf = buf[:0]
					}
					break
				}
				if len(buf)-start < StatusSize {
					buf = buf[start:]
					break
				}
				if s, ok := ParseStatus(buf[start : start+StatusSize]); ok {
					out <- s
				}
				buf = buf[start+StatusSize:]
			}
		}
		if err != nil {
			return
		}
	}
}
