package config

import "testing"

func TestLoadServicePrivateKeyEnvFallback(t *testing.T) {
	t.Run("loads service private key", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv("SERVICE_PRIVATE_KEY", "service-key")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.ServicePrivateKey != "service-key" {
			t.Fatalf("ServicePrivateKey = %q, want %q", cfg.ServicePrivateKey, "service-key")
		}
	})

	t.Run("does not load legacy solana private key", func(t *testing.T) {
		t.Chdir(t.TempDir())
		t.Setenv("SERVICE_PRIVATE_KEY", "")
		t.Setenv("SOLANA_PRIVATE_KEY", "legacy-key")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.ServicePrivateKey != "" {
			t.Fatalf("ServicePrivateKey = %q, want empty string", cfg.ServicePrivateKey)
		}
	})
}
