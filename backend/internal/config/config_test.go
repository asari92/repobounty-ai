package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

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
