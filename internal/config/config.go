package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Input    InputConfig    `yaml:"input"`
	Output   OutputConfig   `yaml:"output"`
	Overlay  OverlayConfig  `yaml:"overlay"`
	Pipeline PipelineConfig `yaml:"pipeline"`
}

// InputConfig represents HLS input configuration
type InputConfig struct {
	HLSUrl          string `yaml:"hls_url"`
	BufferSize      int    `yaml:"buffer_size"`
	ConnectionRetry int    `yaml:"connection_retry"`
	Timeout         int    `yaml:"timeout"`
	SourceType      string `yaml:"source_type"` // "playbin3"
}

// OutputConfig represents UDP output configuration
type OutputConfig struct {
	Host       string `yaml:"host"`
	Port       int    `yaml:"port"`
	Bitrate    int    `yaml:"bitrate"`
	VideoCodec string `yaml:"video_codec"`
	AudioCodec string `yaml:"audio_codec"`
	Format     string `yaml:"format"`
}

// OverlayConfig represents graphic overlay configuration
type OverlayConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Type     string         `yaml:"type"` // "text", "image", "cairo"
	Text     TextOverlay    `yaml:"text"`
	Image    ImageOverlay   `yaml:"image"`
	Cairo    CairoOverlay   `yaml:"cairo"`
	Position PositionConfig `yaml:"position"`
}

// TextOverlay represents text overlay configuration
type TextOverlay struct {
	Content    string `yaml:"content"`
	FontSize   int    `yaml:"font_size"`
	FontFamily string `yaml:"font_family"`
	Color      string `yaml:"color"`
	Background string `yaml:"background"`
}

// ImageOverlay represents image overlay configuration
type ImageOverlay struct {
	Path  string  `yaml:"path"`
	Scale float64 `yaml:"scale"`
	Alpha float64 `yaml:"alpha"`
}

// CairoOverlay represents cairo overlay configuration
type CairoOverlay struct {
	Script string `yaml:"script"`
	Width  int    `yaml:"width"`
	Height int    `yaml:"height"`
}

// PositionConfig represents overlay position
type PositionConfig struct {
	X      int    `yaml:"x"`
	Y      int    `yaml:"y"`
	Anchor string `yaml:"anchor"` // "top-left", "top-right", "bottom-left", "bottom-right", "center"
}

// PipelineConfig represents GStreamer pipeline configuration
type PipelineConfig struct {
	BufferTime    int  `yaml:"buffer_time"`
	LatencyMs     int  `yaml:"latency_ms"`
	SyncOnClock   bool `yaml:"sync_on_clock"`
	DropOnLatency bool `yaml:"drop_on_latency"`
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	// Set default configuration
	cfg := &Config{
		Input: InputConfig{
			BufferSize:      1024 * 1024, // 1MB
			ConnectionRetry: 3,
			Timeout:         30,
			SourceType:      "playbin3", // Default to playbin3 implementation
		},
		Output: OutputConfig{
			Host:       "127.0.0.1",
			Port:       5000,
			Bitrate:    2000000, // 2Mbps
			VideoCodec: "h264",
			AudioCodec: "aac",
			Format:     "mpegts",
		},
		Overlay: OverlayConfig{
			Enabled: true,
			Type:    "text",
			Text: TextOverlay{
				Content:    "Live Stream",
				FontSize:   24,
				FontFamily: "Arial",
				Color:      "white",
				Background: "black",
			},
			Position: PositionConfig{
				X:      10,
				Y:      10,
				Anchor: "top-left",
			},
		},
		Pipeline: PipelineConfig{
			BufferTime:    200,
			LatencyMs:     100,
			SyncOnClock:   true,
			DropOnLatency: true,
		},
	}

	// Read file if it exists
	if _, err := os.Stat(path); err == nil {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return cfg, nil
}

// Save saves configuration to a YAML file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
