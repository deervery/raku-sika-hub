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
