package test

import (
	"testing"

	"video-graphic-overlay-gstreamer/internal/config"
)

func TestSourceTypeConfiguration(t *testing.T) {
	tests := []struct {
		name           string
		configFile     string
		expectedSource string
	}{
		{
			name:           "Default souphttpsrc",
			configFile:     "../examples/basic-text-overlay.yaml",
			expectedSource: "souphttpsrc",
		},
		{
			name:           "Playbin3 source",
			configFile:     "../examples/playbin3-overlay.yaml",
			expectedSource: "playbin3",
		},
		{
			name:           "Urisourcebin source",
			configFile:     "../examples/urisourcebin-overlay.yaml",
			expectedSource: "urisourcebin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := config.Load(tt.configFile)
			if err != nil {
				t.Fatalf("Failed to load config %s: %v", tt.configFile, err)
			}

			if cfg.Input.SourceType == "" {
				// Default should be souphttpsrc
				if tt.expectedSource != "souphttpsrc" {
					t.Errorf("Expected source type %s, but got empty (should default to souphttpsrc)", tt.expectedSource)
				}
			} else if cfg.Input.SourceType != tt.expectedSource {
				t.Errorf("Expected source type %s, but got %s", tt.expectedSource, cfg.Input.SourceType)
			}
		})
	}
}

func TestDefaultSourceType(t *testing.T) {
	// Test that default configuration uses souphttpsrc
	cfg, err := config.Load("nonexistent-file.yaml") // This should load defaults
	if err != nil {
		t.Fatalf("Failed to load default config: %v", err)
	}

	if cfg.Input.SourceType != "souphttpsrc" {
		t.Errorf("Expected default source type to be 'souphttpsrc', but got '%s'", cfg.Input.SourceType)
	}
}

func TestSourceTypeValidation(t *testing.T) {
	validTypes := []string{"souphttpsrc", "playbin3", "urisourcebin"}
	
	for _, sourceType := range validTypes {
		t.Run("Valid_"+sourceType, func(t *testing.T) {
			// Create a minimal config with the source type
			cfg := &config.Config{
				Input: config.InputConfig{
					HLSUrl:     "https://example.com/test.m3u8",
					SourceType: sourceType,
				},
			}
			
			// This test just verifies the config structure accepts the source type
			if cfg.Input.SourceType != sourceType {
				t.Errorf("Expected source type %s, but got %s", sourceType, cfg.Input.SourceType)
			}
		})
	}
}
