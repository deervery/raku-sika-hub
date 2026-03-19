package printer

import (
	"os"
	"path/filepath"
)

// DefaultLogoFile returns a logo filename that exists under assetsDir (relative).
func DefaultLogoFile(assetsDir string) string {
	if assetsDir == "" {
		assetsDir = "."
	}
	candidates := []string{"logo.jpg", "logo.jpeg", "logo.png", "logo.bmp"}
	for _, name := range candidates {
		if _, err := os.Stat(filepath.Join(assetsDir, name)); err == nil {
			return name
		}
	}
	return ""
}
