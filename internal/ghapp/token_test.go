package ghapp

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnv_AllSet(t *testing.T) {
	t.Setenv("GODARK_APP_ID", "123")
	t.Setenv("GODARK_APP_INSTALLATION_ID", "456")
	t.Setenv("GODARK_APP_PRIVATE_KEY_PATH", "/tmp/key.pem")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.AppID != "123" {
		t.Errorf("AppID = %q, want %q", cfg.AppID, "123")
	}
	if cfg.InstallationID != "456" {
		t.Errorf("InstallationID = %q, want %q", cfg.InstallationID, "456")
	}
}

func TestLoadFromEnv_NoneSet(t *testing.T) {
	t.Setenv("GODARK_APP_ID", "")
	t.Setenv("GODARK_APP_INSTALLATION_ID", "")
	t.Setenv("GODARK_APP_PRIVATE_KEY_PATH", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config, got %+v", cfg)
	}
}

func TestLoadFromEnv_PartialSet(t *testing.T) {
	t.Setenv("GODARK_APP_ID", "123")
	t.Setenv("GODARK_APP_INSTALLATION_ID", "")
	t.Setenv("GODARK_APP_PRIVATE_KEY_PATH", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for partial config, got nil")
	}
}

func TestSignJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}

	jwt, err := signJWT("12345", key)
	if err != nil {
		t.Fatalf("signJWT: %v", err)
	}

	// JWT should have three dot-separated parts.
	parts := 0
	for _, b := range jwt {
		if b == '.' {
			parts++
		}
	}
	if parts != 2 {
		t.Errorf("JWT has %d dots, want 2", parts)
	}
}

func TestExchangeToken_Success(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		resp := installationTokenResponse{Token: "ghs_test_token_123"} //nolint:gosec // test fixture
		data, _ := json.Marshal(resp)
		return data, nil
	}

	token, err := exchangeToken("456", "fake-jwt")
	if err != nil {
		t.Fatalf("exchangeToken: %v", err)
	}
	if token != "ghs_test_token_123" { //nolint:gosec // test fixture
		t.Errorf("token = %q, want %q", token, "ghs_test_token_123")
	}
}

func TestExchangeToken_EmptyToken(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		return []byte(`{"token":""}`), nil
	}

	_, err := exchangeToken("456", "fake-jwt")
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}
}

func TestInstallationToken_EndToEnd(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })

	// Generate a test key and write it to a temp file.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	keyPath := filepath.Join(t.TempDir(), "test.pem")
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("writing key: %v", err)
	}

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		resp := installationTokenResponse{Token: "ghs_end_to_end_token"} //nolint:gosec // test fixture
		data, _ := json.Marshal(resp)
		return data, nil
	}

	cfg := &Config{
		AppID:          "12345",
		InstallationID: "67890",
		PrivateKeyPath: keyPath,
	}

	token, err := cfg.InstallationToken()
	if err != nil {
		t.Fatalf("InstallationToken: %v", err)
	}
	if token != "ghs_end_to_end_token" { //nolint:gosec // test fixture
		t.Errorf("token = %q, want %q", token, "ghs_end_to_end_token")
	}
}

func TestRefreshToken_NoConfig(t *testing.T) {
	old := activeConfig.Load()
	t.Cleanup(func() { activeConfig.Store(old) })

	SetActiveConfig(nil)
	token, err := RefreshToken()
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if token != "" {
		t.Errorf("token = %q, want empty when no config set", token)
	}
}

func TestRefreshToken_WithConfig(t *testing.T) {
	orig := CommandRunner
	t.Cleanup(func() { CommandRunner = orig })

	old := activeConfig.Load()
	t.Cleanup(func() { activeConfig.Store(old) })

	// Generate a test key and write it to a temp file.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})
	keyPath := filepath.Join(t.TempDir(), "test.pem")
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatalf("writing key: %v", err)
	}

	CommandRunner = func(name string, args ...string) ([]byte, error) {
		resp := installationTokenResponse{Token: "ghs_refreshed_token"} //nolint:gosec // test fixture
		data, _ := json.Marshal(resp)
		return data, nil
	}

	SetActiveConfig(&Config{
		AppID:          "12345",
		InstallationID: "67890",
		PrivateKeyPath: keyPath,
	})

	token, err := RefreshToken()
	if err != nil {
		t.Fatalf("RefreshToken: %v", err)
	}
	if token != "ghs_refreshed_token" { //nolint:gosec // test fixture
		t.Errorf("token = %q, want %q", token, "ghs_refreshed_token")
	}
}
