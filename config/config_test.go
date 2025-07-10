package config

import (
	"os"
	"testing"
)

func TestLocalMode(t *testing.T) {
	// Save original env vars to restore later
	originalLocalMode := os.Getenv("LOCAL_MODE")
	originalUsername := os.Getenv("AUTH_USERNAME")
	originalPassword := os.Getenv("AUTH_PASSWORD")
	originalMaxRetries := os.Getenv("MAX_RETRIES")
	originalRetryBackoff := os.Getenv("RETRY_BACKOFF")

	defer func() {
		os.Setenv("LOCAL_MODE", originalLocalMode)
		os.Setenv("AUTH_USERNAME", originalUsername)
		os.Setenv("AUTH_PASSWORD", originalPassword)
		os.Setenv("MAX_RETRIES", originalMaxRetries)
		os.Setenv("RETRY_BACKOFF", originalRetryBackoff)
	}()

	// Set auth credentials for non-local mode tests
	os.Setenv("AUTH_USERNAME", "testuser")
	os.Setenv("AUTH_PASSWORD", "testpass")

	// Test when LOCAL_MODE is not set
	os.Unsetenv("LOCAL_MODE")
	cfg, err := New()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if cfg.LocalMode {
		t.Error("LocalMode should be false when LOCAL_MODE env var is not set")
	}

	// Test when LOCAL_MODE is set to true
	os.Setenv("LOCAL_MODE", "true")
	cfg, err = New()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !cfg.LocalMode {
		t.Error("LocalMode should be true when LOCAL_MODE env var is set to 'true'")
	}

	// Test when LOCAL_MODE is set to something else
	os.Setenv("LOCAL_MODE", "yes")
	cfg, err = New()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if cfg.LocalMode {
		t.Error("LocalMode should be false when LOCAL_MODE env var is not 'true'")
	}
}

func TestAuthCredentials(t *testing.T) {
	// Clear environment variables for this test
	t.Setenv("AUTH_USERNAME", "")
	t.Setenv("AUTH_PASSWORD", "")
	t.Setenv("LOCAL_MODE", "")

	// Test missing username in non-local mode
	t.Setenv("AUTH_PASSWORD", "testpass")

	_, err := New()
	if err == nil {
		t.Error("Expected error for missing username in non-local mode")
	}

	// Test missing password in non-local mode
	t.Setenv("AUTH_USERNAME", "testuser")
	t.Setenv("AUTH_PASSWORD", "")

	_, err = New()
	if err == nil {
		t.Error("Expected error for missing password in non-local mode")
	}

	// Test local mode with missing credentials (should not error)
	t.Setenv("LOCAL_MODE", "true")

	cfg, err := New()
	if err != nil {
		t.Errorf("Unexpected error in local mode: %v", err)
	}
	if !cfg.LocalMode {
		t.Error("LocalMode should be true")
	}
}
func TestRetryConfig(t *testing.T) {
	// Save original env vars to restore later
	origMaxRetries := os.Getenv("MAX_RETRIES")
	origRetryBackoff := os.Getenv("RETRY_BACKOFF")
	origLocalMode := os.Getenv("LOCAL_MODE")

	defer func() {
		os.Setenv("MAX_RETRIES", origMaxRetries)
		os.Setenv("RETRY_BACKOFF", origRetryBackoff)
		os.Setenv("LOCAL_MODE", origLocalMode)
	}()

	// Test default values
	t.Run("DefaultValues", func(t *testing.T) {
		os.Unsetenv("MAX_RETRIES")
		os.Unsetenv("RETRY_BACKOFF")
		os.Setenv("LOCAL_MODE", "true") // To bypass auth requirements

		cfg, err := New()
		if err != nil {
			t.Fatalf("Failed to create config: %v", err)
		}

		if cfg.MaxRetries != 3 {
			t.Errorf("Expected default MaxRetries to be 3, got %d", cfg.MaxRetries)
		}

		if cfg.RetryBackoff != 500 {
			t.Errorf("Expected default RetryBackoff to be 500, got %d", cfg.RetryBackoff)
		}
	})

	// Test custom values
	t.Run("CustomValues", func(t *testing.T) {
		os.Setenv("MAX_RETRIES", "5")
		os.Setenv("RETRY_BACKOFF", "200")
		os.Setenv("LOCAL_MODE", "true") // To bypass auth requirements

		cfg, err := New()
		if err != nil {
			t.Fatalf("Failed to create config: %v", err)
		}

		if cfg.MaxRetries != 5 {
			t.Errorf("Expected MaxRetries to be 5, got %d", cfg.MaxRetries)
		}

		if cfg.RetryBackoff != 200 {
			t.Errorf("Expected RetryBackoff to be 200, got %d", cfg.RetryBackoff)
		}
	})

	// Test invalid values
	t.Run("InvalidValues", func(t *testing.T) {
		os.Setenv("MAX_RETRIES", "invalid")
		os.Setenv("RETRY_BACKOFF", "invalid")
		os.Setenv("LOCAL_MODE", "true") // To bypass auth requirements

		cfg, err := New()
		if err != nil {
			t.Fatalf("Failed to create config: %v", err)
		}

		// Should fall back to defaults
		if cfg.MaxRetries != 3 {
			t.Errorf("Expected MaxRetries to fall back to 3, got %d", cfg.MaxRetries)
		}

		if cfg.RetryBackoff != 500 {
			t.Errorf("Expected RetryBackoff to fall back to 500, got %d", cfg.RetryBackoff)
		}
	})

	// Test negative values
	t.Run("NegativeValues", func(t *testing.T) {
		os.Setenv("MAX_RETRIES", "-1")
		os.Setenv("RETRY_BACKOFF", "-100")
		os.Setenv("LOCAL_MODE", "true") // To bypass auth requirements

		cfg, err := New()
		if err != nil {
			t.Fatalf("Failed to create config: %v", err)
		}

		// MaxRetries can be negative (meaning no retries)
		if cfg.MaxRetries != -1 {
			t.Errorf("Expected MaxRetries to be -1, got %d", cfg.MaxRetries)
		}

		// RetryBackoff should not be negative, should fall back to default
		if cfg.RetryBackoff != 500 {
			t.Errorf("Expected RetryBackoff to fall back to 500 for negative value, got %d", cfg.RetryBackoff)
		}
	})
}
