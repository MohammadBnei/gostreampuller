package main

import (
	"testing"

	"gostreampuller/config"
)

func TestConfigLoading(t *testing.T) {
	// Test local mode setting
	t.Setenv("LOCAL_MODE", "true")
	cfg, err := config.New()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if !cfg.LocalMode {
		t.Error("LocalMode should be true when LOCAL_MODE env var is set to 'true'")
	}
}

func TestConfigWithCredentials(t *testing.T) {
	// Test with credentials set
	t.Setenv("LOCAL_MODE", "false")
	t.Setenv("AUTH_USERNAME", "testuser")
	t.Setenv("AUTH_PASSWORD", "testpass")

	cfg, err := config.New()
	if err != nil {
		t.Fatalf("Failed to load config with credentials: %v", err)
	}

	if cfg.LocalMode {
		t.Error("LocalMode should be false")
	}
	if cfg.AuthUsername != "testuser" {
		t.Errorf("Expected username 'testuser', got '%s'", cfg.AuthUsername)
	}
	if cfg.AuthPassword != "testpass" {
		t.Errorf("Expected password 'testpass', got '%s'", cfg.AuthPassword)
	}
}
