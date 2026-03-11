package config

import (
	"os"
	"testing"
)

func TestLoad_UsesEnvOverrideWithoutConfigFile(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	t.Setenv("PRINTER_NAME", "Brother_QL-820NWB")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.PrinterName != "Brother_QL-820NWB" {
		t.Fatalf("expected PRINTER_NAME override, got %q", cfg.PrinterName)
	}
	if cfg.PrinterDriver != "cups_png" {
		t.Fatalf("expected default cups_png driver, got %q", cfg.PrinterDriver)
	}
}

func TestLoad_ParsesMaxClientsAndOriginsFromEnv(t *testing.T) {
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldWD)
	})

	t.Setenv("MAX_CLIENTS", "3")
	t.Setenv("ALLOW_ALL_ORIGINS", "true")
	t.Setenv("ALLOWED_ORIGINS", "localhost:*,127.0.0.1:*,192.168.50.*")
	t.Setenv("SCALE_DRIVER", "mock")
	t.Setenv("PRINTER_DRIVER", "cups_png")
	t.Setenv("PRINTER_ADDRESS", "192.168.50.40:9100")
	t.Setenv("TEMPLATE_MAP_PATH", "custom-template-map.json")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.MaxClients != 3 {
		t.Fatalf("expected MAX_CLIENTS override, got %d", cfg.MaxClients)
	}
	if !cfg.AllowAllOrigins {
		t.Fatal("expected AllowAllOrigins=true")
	}
	if cfg.ScaleDriver != "mock" {
		t.Fatalf("expected SCALE_DRIVER override, got %q", cfg.ScaleDriver)
	}
	if len(cfg.AllowedOrigins) != 3 {
		t.Fatalf("expected 3 origins, got %d", len(cfg.AllowedOrigins))
	}
	if cfg.PrinterDriver != "cups_png" {
		t.Fatalf("expected PRINTER_DRIVER override, got %q", cfg.PrinterDriver)
	}
	if cfg.PrinterAddress != "192.168.50.40:9100" {
		t.Fatalf("expected PRINTER_ADDRESS override, got %q", cfg.PrinterAddress)
	}
	if cfg.TemplateMapPath != "custom-template-map.json" {
		t.Fatalf("expected TEMPLATE_MAP_PATH override, got %q", cfg.TemplateMapPath)
	}
}
