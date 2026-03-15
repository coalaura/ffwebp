package headless

import (
	"os"
	"runtime"
)

func init() {
	// Ebitengine (specifically GLFW) panics during package initialization if
	// the DISPLAY environment variable is missing on Linux. This prevents the
	// CLI from running on headless servers even if --play is not used.
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" {
		os.Setenv("DISPLAY", ":99.0")
	}
}
