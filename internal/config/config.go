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
	VID             string   `json:"vid"`
	PID             string   `json:"pid"`
	Port            string   `json:"port"`
	BaudRate        int      `json:"baudRate"`
	DataBits        int      `json:"dataBits"`
	Parity          string   `json:"parity"`
	StopBits        int      `json:"stopBits"`
	PrinterName     string   `json:"printerName"`
	PrinterDriver   string   `json:"printerDriver"`
	PrinterAddress  string   `json:"printerAddress"`
	TemplateMapPath string   `json:"templateMapPath"`
	FontPath        string   `json:"fontPath"`
	MaxClients      int      `json:"maxClients"`
	ListenAddr      string   `json:"listenAddr"`
	LogLevel        string   `json:"logLevel"`
	ScaleDriver     string   `json:"scaleDriver"`
	AllowAllOrigins bool     `json:"allowAllOrigins"`
	AllowedOrigins  []string `json:"allowedOrigins"`
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
		PrinterName:     "Brother_QL-820NWB",
		PrinterDriver:   "ptouch_template",
		PrinterAddress:  "",
		TemplateMapPath: "templates/siknue/template-map.json",
		MaxClients:      1,
		ListenAddr:      "0.0.0.0:19800",
		LogLevel:        "INFO",
		ScaleDriver:     strings.TrimSpace(os.Getenv("SCALE_DRIVER")),
		AllowedOrigins: []string{
			"localhost:*",
			"127.0.0.1:*",
			"192.168.50.*",
			"preview.rakusika.com",
			"rakusika.com",
			"*.rakusika.com",
		},
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
	if v := strings.TrimSpace(os.Getenv("PRINTER_DRIVER")); v != "" {
		cfg.PrinterDriver = v
	}
	if v := strings.TrimSpace(os.Getenv("PRINTER_ADDRESS")); v != "" {
		cfg.PrinterAddress = v
	}
	if v := strings.TrimSpace(os.Getenv("TEMPLATE_MAP_PATH")); v != "" {
		cfg.TemplateMapPath = v
	}
	if v := strings.TrimSpace(os.Getenv("FONT_PATH")); v != "" {
		cfg.FontPath = v
	}
	if v := strings.TrimSpace(os.Getenv("LISTEN_ADDR")); v != "" {
		cfg.ListenAddr = v
	}
	if v := strings.TrimSpace(os.Getenv("LOG_LEVEL")); v != "" {
		cfg.LogLevel = v
	}
	if v := strings.TrimSpace(os.Getenv("SCALE_DRIVER")); v != "" {
		cfg.ScaleDriver = v
	}
	if v := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS")); v != "" {
		cfg.AllowedOrigins = splitCSV(v)
	}
	if v := strings.TrimSpace(os.Getenv("ALLOW_ALL_ORIGINS")); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			cfg.AllowAllOrigins = b
		}
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
	if v := strings.TrimSpace(os.Getenv("MAX_CLIENTS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxClients = n
		}
	}
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}
