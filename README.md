# Video Graphic Overlay GStreamer

A Go application that processes HLS (HTTP Live Streaming) input streams, adds graphic overlays, and outputs to UDP streams using GStreamer.

## Features

- **HLS Input**: Support for HTTP Live Streaming (HLS) input with automatic retry and buffering
- **Graphic Overlays**: Text, image, and Cairo-based overlays with customizable positioning
- **UDP Output**: High-performance UDP streaming with configurable encoding
- **Real-time Processing**: Low-latency video processing optimized for live streaming
- **Error Handling**: Comprehensive error handling with automatic recovery
- **Configuration**: YAML-based configuration with sensible defaults

## Prerequisites

### System Dependencies

1. **GStreamer**: Install GStreamer development libraries
   ```bash
   # Ubuntu/Debian
   sudo apt-get install libgstreamer1.0-dev libgstreamer-plugins-base1.0-dev \
                        libgstreamer-plugins-good1.0-dev libgstreamer-plugins-bad1.0-dev

   # CentOS/RHEL/Fedora
   sudo yum install gstreamer1-devel gstreamer1-plugins-base-devel \
                    gstreamer1-plugins-good gstreamer1-plugins-bad-free

   # macOS
   brew install gstreamer gst-plugins-base gst-plugins-good gst-plugins-bad
   ```

2. **Go**: Go 1.21 or later
   ```bash
   # Download from https://golang.org/dl/
   ```

### GStreamer Plugins

Ensure the following GStreamer plugins are installed:
- `gst-plugins-good` (for hlsdemux, udpsink)
- `gst-plugins-bad` (for additional codecs)
- `gst-plugins-ugly` (for x264enc)
- `gst-libav` (for avenc_aac)

## Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd video-graphic-overlay-gstreamer
   ```

2. Install Go dependencies:
   ```bash
   go mod download
   ```

3. Build the application:
   ```bash
   go build -o video-overlay ./main.go
   ```

## Configuration

Create a `config.yaml` file (see `config.yaml` for example):

```yaml
input:
  hls_url: "https://example.com/stream/playlist.m3u8"
  buffer_size: 1048576  # 1MB
  connection_retry: 3
  timeout: 30

output:
  host: "127.0.0.1"
  port: 5000
  bitrate: 2000000  # 2Mbps
  video_codec: "h264"
  audio_codec: "aac"
  format: "mpegts"

overlay:
  enabled: true
  type: "text"  # "text", "image", "cairo"
  text:
    content: "Live Stream - {{.timestamp}}"
    font_size: 24
    font_family: "Arial"
    color: "white"
    background: "rgba(0,0,0,0.5)"
  position:
    x: 10
    y: 10
    anchor: "top-left"
```

## Usage

### Basic Usage

```bash
./video-overlay -config config.yaml
```

### Command Line Options

- `-config`: Path to configuration file (default: `config.yaml`)

### Environment Variables

You can override configuration values using environment variables:

```bash
export HLS_URL="https://your-stream.com/playlist.m3u8"
export UDP_HOST="192.168.1.100"
export UDP_PORT="5001"
./video-overlay
```

## Examples

### Example 1: Basic Text Overlay

```yaml
overlay:
  enabled: true
  type: "text"
  text:
    content: "LIVE"
    font_size: 32
    font_family: "Arial Bold"
    color: "red"
  position:
    x: 20
    y: 20
    anchor: "top-left"
```

### Example 2: Dynamic Timestamp Overlay

```yaml
overlay:
  enabled: true
  type: "text"
  text:
    content: "{{.timestamp}} - Channel 1"
    font_size: 24
    color: "white"
    background: "rgba(0,0,0,0.7)"
  position:
    x: 10
    y: 10
    anchor: "top-left"
```

### Example 3: Logo Overlay

```yaml
overlay:
  enabled: true
  type: "image"
  image:
    path: "/path/to/logo.png"
    scale: 0.5
    alpha: 0.8
  position:
    x: 20
    y: 20
    anchor: "top-right"
```

## API Reference

### Configuration Structure

- `input`: HLS input configuration
  - `hls_url`: HLS stream URL
  - `buffer_size`: Buffer size in bytes
  - `connection_retry`: Number of connection retries
  - `timeout`: Connection timeout in seconds

- `output`: UDP output configuration
  - `host`: Target host/IP address
  - `port`: Target port
  - `bitrate`: Video bitrate in bps
  - `video_codec`: Video codec (h264, h265, vp8, vp9)
  - `audio_codec`: Audio codec (aac, mp3, opus)
  - `format`: Container format (mpegts, mp4, webm)

- `overlay`: Graphic overlay configuration
  - `enabled`: Enable/disable overlay
  - `type`: Overlay type (text, image, cairo)
  - `text`: Text overlay settings
  - `image`: Image overlay settings
  - `position`: Overlay position settings

### Template Variables

Text overlays support template variables:
- `{{.timestamp}}`: Current timestamp (YYYY-MM-DD HH:MM:SS)
- `{{.date}}`: Current date (YYYY-MM-DD)
- `{{.time}}`: Current time (HH:MM:SS)
- `{{.unix}}`: Unix timestamp

## Troubleshooting

### Common Issues

1. **GStreamer not found**
   ```
   Error: failed to initialize GStreamer
   ```
   Solution: Install GStreamer development libraries

2. **Plugin not found**
   ```
   Error: no element "hlsdemux"
   ```
   Solution: Install gst-plugins-good

3. **Network connection failed**
   ```
   Error: failed to connect to HLS stream
   ```
   Solution: Check HLS URL and network connectivity

4. **UDP output not working**
   ```
   Error: failed to bind UDP socket
   ```
   Solution: Check if port is available and not blocked by firewall

### Debug Mode

Enable debug logging by setting log level:

```bash
export LOG_LEVEL=debug
./video-overlay -config config.yaml
```

### Testing UDP Output

Test UDP output with VLC or ffplay:

```bash
# VLC
vlc udp://@127.0.0.1:5000

# ffplay
ffplay udp://127.0.0.1:5000
```

## Performance Tuning

### Low Latency Configuration

```yaml
pipeline:
  buffer_time: 100
  latency_ms: 50
  sync_on_clock: true
  drop_on_latency: true

output:
  bitrate: 1000000  # Lower bitrate
  video_codec: "h264"
```

### High Quality Configuration

```yaml
output:
  bitrate: 5000000  # Higher bitrate
  video_codec: "h265"  # Better compression
```

## License

[Add your license information here]

## Contributing

[Add contributing guidelines here]
