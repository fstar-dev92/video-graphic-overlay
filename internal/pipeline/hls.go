package pipeline

import (
	"fmt"
	"net/url"
	"strings"

	"video-graphic-overlay-gstreamer/internal/config"

	"github.com/go-gst/go-gst/gst"
)

// HLSInput handles HLS stream input
type HLSInput struct {
	config *config.InputConfig
	source *gst.Element
	demux  *gst.Element
}

// NewHLSInput creates a new HLS input handler
func NewHLSInput(cfg *config.InputConfig) (*HLSInput, error) {
	// Validate HLS URL
	if err := validateHLSURL(cfg.HLSUrl); err != nil {
		return nil, fmt.Errorf("invalid HLS URL: %w", err)
	}

	return &HLSInput{
		config: cfg,
	}, nil
}

// CreateElements creates the GStreamer elements for HLS input
func (h *HLSInput) CreateElements() ([]*gst.Element, error) {
	var elements []*gst.Element

	// Create souphttpsrc element
	source, err := gst.NewElement("souphttpsrc")
	if err != nil {
		return nil, fmt.Errorf("failed to create souphttpsrc: %w", err)
	}

	// Configure souphttpsrc
	source.SetProperty("location", h.config.HLSUrl)
	source.SetProperty("timeout", h.config.Timeout)
	source.SetProperty("retries", h.config.ConnectionRetry)

	// Set user agent for better compatibility
	source.SetProperty("user-agent", "GStreamer-HLS-Overlay/1.0")

	// Enable automatic retries
	source.SetProperty("automatic-redirect", true)

	h.source = source
	elements = append(elements, source)

	// Create hlsdemux element
	demux, err := gst.NewElement("hlsdemux")
	if err != nil {
		return nil, fmt.Errorf("failed to create hlsdemux: %w", err)
	}

	// Configure hlsdemux for low latency
	// Note: max-buffering-time property doesn't exist in newer GStreamer versions
	// Use connection-speed instead for better performance
	demux.SetProperty("connection-speed", uint(h.config.BufferSize/1024)) // Convert to kbps

	h.demux = demux
	elements = append(elements, demux)

	return elements, nil
}

// GetPipelineString returns the pipeline string for HLS input
func (h *HLSInput) GetPipelineString() string {
	return fmt.Sprintf("souphttpsrc location=%s timeout=%d retries=%d "+
		"user-agent=\"GStreamer-HLS-Overlay/1.0\" automatic-redirect=true ! "+
		"hlsdemux connection-speed=%d name=demux",
		h.config.HLSUrl,
		h.config.Timeout,
		h.config.ConnectionRetry,
		h.config.BufferSize/1024) // Convert to kbps
}

// validateHLSURL validates the HLS URL format
func validateHLSURL(hlsURL string) error {
	if hlsURL == "" {
		return fmt.Errorf("HLS URL cannot be empty")
	}

	// Parse URL
	u, err := url.Parse(hlsURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Check scheme
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("HLS URL must use http or https scheme")
	}

	// Check if it looks like an HLS playlist
	if !strings.HasSuffix(strings.ToLower(u.Path), ".m3u8") &&
		!strings.HasSuffix(strings.ToLower(u.Path), ".m3u") {
		return fmt.Errorf("HLS URL should point to a .m3u8 or .m3u playlist file")
	}

	return nil
}

// AdaptiveHLSInput handles adaptive HLS streams with multiple quality levels
type AdaptiveHLSInput struct {
	*HLSInput
	maxBitrate int
	minBitrate int
}

// NewAdaptiveHLSInput creates a new adaptive HLS input handler
func NewAdaptiveHLSInput(cfg *config.InputConfig, maxBitrate, minBitrate int) (*AdaptiveHLSInput, error) {
	base, err := NewHLSInput(cfg)
	if err != nil {
		return nil, err
	}

	return &AdaptiveHLSInput{
		HLSInput:   base,
		maxBitrate: maxBitrate,
		minBitrate: minBitrate,
	}, nil
}

// GetPipelineString returns the pipeline string for adaptive HLS input
func (a *AdaptiveHLSInput) GetPipelineString() string {
	return fmt.Sprintf("souphttpsrc location=%s timeout=%d retries=%d "+
		"user-agent=\"GStreamer-HLS-Overlay/1.0\" automatic-redirect=true ! "+
		"hlsdemux connection-speed=%d bitrate-limit=%.1f name=demux",
		a.config.HLSUrl,
		a.config.Timeout,
		a.config.ConnectionRetry,
		a.config.BufferSize/1024, // Convert to kbps
		float64(a.maxBitrate)/float64(a.config.BufferSize)) // Bitrate limit ratio
}
