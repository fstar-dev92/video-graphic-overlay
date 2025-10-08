# Migration Guide: Source Element Options

This guide explains the new source element options available in the video-graphic-overlay-gstreamer application and how to migrate between different approaches.

## Overview

The application now supports three different GStreamer source approaches for HLS streaming:

1. **souphttpsrc** (Default) - Traditional manual approach
2. **playbin3** - High-level automatic approach  
3. **urisourcebin** - Mid-level semi-automatic approach

## Configuration Changes

### New Configuration Option

A new `source_type` field has been added to the input configuration:

```yaml
input:
  hls_url: "https://example.com/stream/playlist.m3u8"
  buffer_size: 1048576
  connection_retry: 3
  timeout: 30
  source_type: "souphttpsrc"  # New field
```

### Valid Values

- `"souphttpsrc"` - Uses souphttpsrc + hlsdemux + tsdemux (default)
- `"playbin3"` - Uses playbin3 for automatic handling
- `"urisourcebin"` - Uses urisourcebin for semi-automatic handling

## Source Type Comparison

### souphttpsrc (Default)
```yaml
source_type: "souphttpsrc"
```

**Pipeline**: `souphttpsrc → hlsdemux → tsdemux → processing`

**Advantages:**
- Maximum control over each pipeline element
- Explicit error handling at each stage
- Custom HTTP properties (user-agent, SSL settings, etc.)
- Best for debugging and troubleshooting

**Disadvantages:**
- More complex pipeline setup
- Manual pad management required
- More code to maintain

**Best for:** Production environments requiring maximum control and debugging capability

### playbin3
```yaml
source_type: "playbin3"
```

**Pipeline**: `playbin3 → processing` (internal demuxing)

**Advantages:**
- Simplified pipeline setup
- Automatic adaptive streaming support
- Built-in error recovery
- Automatic source selection
- Less code complexity

**Disadvantages:**
- Less control over individual elements
- Harder to debug specific issues
- Limited customization options

**Best for:** Simple deployments, rapid prototyping, automatic quality adaptation

### urisourcebin
```yaml
source_type: "urisourcebin"
```

**Pipeline**: `urisourcebin → processing` (internal source + demuxing)

**Advantages:**
- Good balance of automation and control
- Automatic source selection
- Easier than manual souphttpsrc setup
- Still provides access to demuxed streams

**Disadvantages:**
- Less control than manual approach
- Some automatic behavior you can't override

**Best for:** Most production use cases requiring reliability with some control

## Migration Steps

### From souphttpsrc to playbin3

1. Update your configuration file:
```yaml
input:
  source_type: "playbin3"  # Add this line
  # Remove any souphttpsrc-specific settings if present
```

2. Test with the new configuration:
```bash
./video-overlay -config your-config.yaml
```

### From souphttpsrc to urisourcebin

1. Update your configuration file:
```yaml
input:
  source_type: "urisourcebin"  # Add this line
```

2. Test with the new configuration:
```bash
./video-overlay -config your-config.yaml
```

### Backward Compatibility

- Existing configurations without `source_type` will continue to work (defaults to `souphttpsrc`)
- All other configuration options remain the same
- No breaking changes to the API

## Example Configurations

### Basic playbin3 Setup
```yaml
input:
  hls_url: "https://example.com/stream.m3u8"
  source_type: "playbin3"
  buffer_size: 1048576
  timeout: 30

output:
  host: "127.0.0.1"
  port: 5000
  bitrate: 2000000
  video_codec: "h264"
  audio_codec: "aac"
  format: "mpegts"

overlay:
  enabled: true
  type: "text"
  text:
    content: "LIVE - {{.time}}"
    font_size: 24
    color: "white"
```

### Basic urisourcebin Setup
```yaml
input:
  hls_url: "https://example.com/stream.m3u8"
  source_type: "urisourcebin"
  buffer_size: 1048576
  timeout: 30
  connection_retry: 3

# ... rest of configuration same as above
```

## Testing

Use the provided example configurations to test each approach:

```bash
# Test playbin3
make run-playbin3

# Test urisourcebin  
make run-urisourcebin

# Test traditional souphttpsrc
make run-basic
```

## Troubleshooting

### playbin3 Issues
- Ensure GStreamer 1.14+ is installed
- Check that `gst-plugins-good` includes playbin3
- Monitor logs for automatic quality switching

### urisourcebin Issues
- Ensure GStreamer 1.12+ is installed
- Verify all required plugins are available
- Check network connectivity for automatic source detection

### General Issues
- Use `GST_DEBUG=3` environment variable for detailed logging
- Test with a known working HLS stream first
- Verify GStreamer plugin availability with `gst-inspect-1.0`

## Performance Considerations

- **playbin3**: May use more CPU due to automatic quality adaptation
- **urisourcebin**: Balanced CPU usage with good reliability
- **souphttpsrc**: Most predictable performance, manual optimization possible

Choose the approach that best fits your performance requirements and operational complexity preferences.
