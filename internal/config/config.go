package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the application configuration loaded from config.json.
type Config struct {
	VID               string `json:"vid"`
	PID               string `json:"pid"`
	Port              string `json:"port"`
	BaudRate          int    `json:"baudRate"`
	DataBits          int    `json:"dataBits"`
	Parity            string `json:"parity"`
	StopBits          int    `json:"stopBits"`
	PrinterName       string `json:"printerName"`
	FontPath          string `json:"fontPath"`
	AssetsDir         string `json:"assetsDir"`
	ListenAddr        string `json:"listenAddr"`
	LogLevel          string `json:"logLevel"`
	EnableWebSocket   bool   `json:"enableWebSocket"`
	ScannerVid        string `json:"scannerVid"`
	ScannerPid        string `json:"scannerPid"`
	ScannerDeviceName string `json:"scannerDeviceName"`
	ProcessorName     string `json:"processorName"`
	ProcessorLocation string `json:"processorLocation"`
	CaptureLocation   string `json:"captureLocation"`
}

// Default returns a Config with factory defaults for A&D HV-C series (HV-60KCWP-K) on Raspberry Pi.
func Default() Config {
	return Config{
		VID:             "0403",
		PID:             "6015",
		Port:            "",
		BaudRate:        2400,
		DataBits:        7,
		Parity:          "even",
		StopBits:        1,
		PrinterName:     "",
		AssetsDir:       "assets",
		ListenAddr:      "0.0.0.0:19800",
		LogLevel:        "INFO",
		EnableWebSocket:   false,
		ProcessorName:     "(株)札幌カネシン水産",
		ProcessorLocation: "北海道訓子府町大町113",
	}
}

// ConfigDir returns the directory where config.json is stored.
func ConfigDir() string {
	return "."
}

// Load reads config.json from the config directory.
// Missing fields retain their default values.
// If the file does not exist, defaults are returned without error.
func Load() (Config, error) {
	cfg := Default()
	path := filepath.Join(ConfigDir(), "config.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(&cfg)
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}

	applyEnvOverrides(&cfg)
	return cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	if v := strings.TrimSpace(os.Getenv("VID")); v != "" {
		cfg.VID = v
	}
	if v := strings.TrimSpace(os.Getenv("PID")); v != "" {
		cfg.PID = v
	}
	if v := strings.TrimSpace(os.Getenv("PORT")); v != "" {
		cfg.Port = v
	}
	if v := strings.TrimSpace(os.Getenv("PARITY")); v != "" {
		cfg.Parity = v
	}
	if v := strings.TrimSpace(os.Getenv("PRINTER_NAME")); v != "" {
		cfg.PrinterName = v
	}
	if v := strings.TrimSpace(os.Getenv("FONT_PATH")); v != "" {
		cfg.FontPath = v
	}
	if v := strings.TrimSpace(os.Getenv("ASSETS_DIR")); v != "" {
		cfg.AssetsDir = v
	}
	if v := strings.TrimSpace(os.Getenv("LISTEN_ADDR")); v != "" {
		cfg.ListenAddr = v
	}
	if v := strings.TrimSpace(os.Getenv("LOG_LEVEL")); v != "" {
		cfg.LogLevel = v
	}
	if v := strings.TrimSpace(os.Getenv("BAUD_RATE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.BaudRate = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("DATA_BITS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DataBits = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("STOP_BITS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.StopBits = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("ENABLE_WEBSOCKET")); v != "" {
		cfg.EnableWebSocket = v == "true" || v == "1"
	}
	if v := strings.TrimSpace(os.Getenv("SCANNER_VID")); v != "" {
		cfg.ScannerVid = v
	}
	if v := strings.TrimSpace(os.Getenv("SCANNER_PID")); v != "" {
		cfg.ScannerPid = v
	}
	if v := strings.TrimSpace(os.Getenv("SCANNER_DEVICE_NAME")); v != "" {
		cfg.ScannerDeviceName = v
	}
	if v := strings.TrimSpace(os.Getenv("PROCESSOR_NAME")); v != "" {
		cfg.ProcessorName = v
	}
	if v := strings.TrimSpace(os.Getenv("PROCESSOR_LOCATION")); v != "" {
		cfg.ProcessorLocation = v
	}
	if v := strings.TrimSpace(os.Getenv("CAPTURE_LOCATION")); v != "" {
		cfg.CaptureLocation = v
	}
}
