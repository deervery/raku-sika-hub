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
}
