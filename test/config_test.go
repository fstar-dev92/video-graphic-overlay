package test

import (
	"os"
	"testing"

	"video-graphic-overlay-gstreamer/internal/config"
)

func TestConfigLoad(t *testing.T) {
	// Test loading default configuration
	cfg, err := config.Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("Failed to load default config: %v", err)
	}

	// Verify default values
	if cfg.Input.BufferSize != 1024*1024 {
		t.Errorf("Expected buffer size 1048576, got %d", cfg.Input.BufferSize)
	}

	if cfg.Output.Host != "127.0.0.1" {
		t.Errorf("Expected host 127.0.0.1, got %s", cfg.Output.Host)
	}

	if cfg.Output.Port != 5000 {
		t.Errorf("Expected port 5000, got %d", cfg.Output.Port)
	}

	if !cfg.Overlay.Enabled {
		t.Error("Expected overlay to be enabled by default")
	}
}

func TestConfigSaveLoad(t *testing.T) {
	// Create temporary config file
	tmpFile := "/tmp/test_config.yaml"
	defer os.Remove(tmpFile)

	// Load default config
	cfg, err := config.Load("nonexistent.yaml")
	if err != nil {
		t.Fatalf("Failed to load default config: %v", err)
	}

	// Modify some values
	cfg.Input.HLSUrl = "https://test.example.com/playlist.m3u8"
	cfg.Output.Port = 6000
	cfg.Overlay.Text.Content = "Test Overlay"

	// Save config
	if err := cfg.Save(tmpFile); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load saved config
	loadedCfg, err := config.Load(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	// Verify values
	if loadedCfg.Input.HLSUrl != "https://test.example.com/playlist.m3u8" {
		t.Errorf("Expected HLS URL to be preserved, got %s", loadedCfg.Input.HLSUrl)
	}

	if loadedCfg.Output.Port != 6000 {
		t.Errorf("Expected port 6000, got %d", loadedCfg.Output.Port)
	}

	if loadedCfg.Overlay.Text.Content != "Test Overlay" {
		t.Errorf("Expected overlay text to be preserved, got %s", loadedCfg.Overlay.Text.Content)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg, _ := config.Load("nonexistent.yaml")

	// Test valid configuration
	if cfg.Output.Host == "" {
		t.Error("Host should not be empty in default config")
	}

	if cfg.Output.Port <= 0 || cfg.Output.Port > 65535 {
		t.Error("Port should be in valid range in default config")
	}

	if cfg.Output.Bitrate <= 0 {
		t.Error("Bitrate should be positive in default config")
	}
}
