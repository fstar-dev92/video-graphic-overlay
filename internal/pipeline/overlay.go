package pipeline

import (
	"fmt"
	"strings"
	"time"

	"video-graphic-overlay-gstreamer/internal/config"
)

// OverlayManager handles graphic overlays
type OverlayManager struct {
	config *config.OverlayConfig
}

// NewOverlayManager creates a new overlay manager
func NewOverlayManager(cfg *config.OverlayConfig) *OverlayManager {
	return &OverlayManager{
		config: cfg,
	}
}

// GetPipelineString returns the pipeline string for overlay
func (o *OverlayManager) GetPipelineString() string {
	if !o.config.Enabled {
		return ""
	}

	switch o.config.Type {
	case "text":
		return o.getTextOverlayString()
	case "image":
		return o.getImageOverlayString()
	case "cairo":
		return o.getCairoOverlayString()
	default:
		return ""
	}
}

// getTextOverlayString creates text overlay pipeline string
func (o *OverlayManager) getTextOverlayString() string {
	text := o.processTextTemplate(o.config.Text.Content)
	
	// Calculate position based on anchor
	xpos, ypos := o.calculatePosition()
	
	return fmt.Sprintf("textoverlay text=\"%s\" font-desc=\"%s %d\" "+
		"color=0x%s "+
		"xpos=%d ypos=%d "+
		"wrap-mode=word-char "+
		"line-alignment=left",
		text,
		o.config.Text.FontFamily,
		o.config.Text.FontSize,
		o.parseColor(o.config.Text.Color),
		xpos,
		ypos)
}

// getImageOverlayString creates image overlay pipeline string
func (o *OverlayManager) getImageOverlayString() string {
	if o.config.Image.Path == "" {
		return ""
	}

	xpos, ypos := o.calculatePosition()
	
	return fmt.Sprintf("gdkpixbufoverlay location=%s "+
		"offset-x=%d offset-y=%d "+
		"alpha=%f "+
		"relative-x=0 relative-y=0",
		o.config.Image.Path,
		xpos,
		ypos,
		o.config.Image.Alpha)
}

// getCairoOverlayString creates cairo overlay pipeline string
func (o *OverlayManager) getCairoOverlayString() string {
	return "cairooverlay"
}

// processTextTemplate processes text templates with dynamic content
func (o *OverlayManager) processTextTemplate(text string) string {
	// Replace common template variables
	replacements := map[string]string{
		"{{.timestamp}}": time.Now().Format("2006-01-02 15:04:05"),
		"{{.date}}":      time.Now().Format("2006-01-02"),
		"{{.time}}":      time.Now().Format("15:04:05"),
		"{{.unix}}":      fmt.Sprintf("%d", time.Now().Unix()),
	}

	result := text
	for placeholder, value := range replacements {
		result = strings.ReplaceAll(result, placeholder, value)
	}

	return result
}

// calculatePosition calculates overlay position based on anchor
func (o *OverlayManager) calculatePosition() (int, int) {
	x := o.config.Position.X
	y := o.config.Position.Y

	// For now, return absolute positions
	// In a real implementation, you might want to calculate relative positions
	// based on video dimensions and anchor point
	switch o.config.Position.Anchor {
	case "top-left":
		return x, y
	case "top-right":
		// Would need video width to calculate properly
		return x, y
	case "bottom-left":
		// Would need video height to calculate properly
		return x, y
	case "bottom-right":
		// Would need video width and height to calculate properly
		return x, y
	case "center":
		// Would need video width and height to calculate properly
		return x, y
	default:
		return x, y
	}
}

// parseColor converts color string to hex format for GStreamer
func (o *OverlayManager) parseColor(color string) string {
	// Remove any prefix and convert to uppercase
	color = strings.TrimPrefix(color, "#")
	color = strings.TrimPrefix(color, "0x")
	color = strings.ToUpper(color)

	// Handle named colors
	namedColors := map[string]string{
		"WHITE":   "FFFFFF",
		"BLACK":   "000000",
		"RED":     "FF0000",
		"GREEN":   "00FF00",
		"BLUE":    "0000FF",
		"YELLOW":  "FFFF00",
		"CYAN":    "00FFFF",
		"MAGENTA": "FF00FF",
		"GRAY":    "808080",
		"GREY":    "808080",
	}

	if hex, exists := namedColors[color]; exists {
		return hex
	}

	// Validate hex color (should be 6 characters)
	if len(color) == 6 {
		return color
	}

	// Default to white if invalid
	return "FFFFFF"
}

// TextOverlayBuilder helps build complex text overlays
type TextOverlayBuilder struct {
	text       string
	fontSize   int
	fontFamily string
	color      string
	background string
	position   config.PositionConfig
	shadow     bool
	outline    bool
}

// NewTextOverlayBuilder creates a new text overlay builder
func NewTextOverlayBuilder() *TextOverlayBuilder {
	return &TextOverlayBuilder{
		fontSize:   24,
		fontFamily: "Arial",
		color:      "white",
		background: "transparent",
	}
}

// SetText sets the overlay text
func (t *TextOverlayBuilder) SetText(text string) *TextOverlayBuilder {
	t.text = text
	return t
}

// SetFont sets the font family and size
func (t *TextOverlayBuilder) SetFont(family string, size int) *TextOverlayBuilder {
	t.fontFamily = family
	t.fontSize = size
	return t
}

// SetColor sets the text color
func (t *TextOverlayBuilder) SetColor(color string) *TextOverlayBuilder {
	t.color = color
	return t
}

// SetPosition sets the overlay position
func (t *TextOverlayBuilder) SetPosition(x, y int, anchor string) *TextOverlayBuilder {
	t.position = config.PositionConfig{
		X:      x,
		Y:      y,
		Anchor: anchor,
	}
	return t
}

// EnableShadow enables text shadow
func (t *TextOverlayBuilder) EnableShadow() *TextOverlayBuilder {
	t.shadow = true
	return t
}

// EnableOutline enables text outline
func (t *TextOverlayBuilder) EnableOutline() *TextOverlayBuilder {
	t.outline = true
	return t
}

// Build builds the text overlay configuration
func (t *TextOverlayBuilder) Build() config.TextOverlay {
	return config.TextOverlay{
		Content:    t.text,
		FontSize:   t.fontSize,
		FontFamily: t.fontFamily,
		Color:      t.color,
		Background: t.background,
	}
}
