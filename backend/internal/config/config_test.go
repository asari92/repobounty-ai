package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestLoadUsesOneMinuteAutoFinalizeIntervalByDefault(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("AUTO_FINALIZE_INTERVAL_SECONDS", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AutoFinalizeIntervalSeconds != 60 {
		t.Fatalf("AutoFinalizeIntervalSeconds = %d, want %d", cfg.AutoFinalizeIntervalSeconds, 60)
	}
}

func TestLoadUsesServicePrivateKey(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("SERVICE_PRIVATE_KEY", "service-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.ServicePrivateKey != "service-key" {
		t.Fatalf("ServicePrivateKey = %q, want %q", cfg.ServicePrivateKey, "service-key")
	}
}

func TestResolveDatabasePathUsesStableBackendRoot(t *testing.T) {
	currentFile := testFilePath(t)
	backendRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	repoRoot := filepath.Dir(backendRoot)
	want := filepath.Join(backendRoot, "repobounty.db")

	for _, cwd := range []string{backendRoot, repoRoot} {
		t.Run(cwd, func(t *testing.T) {
			oldWD, err := os.Getwd()
			if err != nil {
				t.Fatalf("Getwd: %v", err)
			}
			defer func() {
				if chdirErr := os.Chdir(oldWD); chdirErr != nil {
					t.Fatalf("restore cwd: %v", chdirErr)
				}
			}()

			if err := os.Chdir(cwd); err != nil {
				t.Fatalf("Chdir(%s): %v", cwd, err)
			}

			got, err := resolveDatabasePath("repobounty.db")
			if err != nil {
				t.Fatalf("resolveDatabasePath: %v", err)
			}
			if got != want {
				t.Fatalf("resolveDatabasePath() = %q, want %q", got, want)
			}
		})
	}
}

func testFilePath(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return file
}
