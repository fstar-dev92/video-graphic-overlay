# Pipeline Architecture: Clean Source Element Implementation

This document explains the cleaned up pipeline architecture for the three different source approaches, with proper separation of concerns and elimination of redundant elements.

## Architecture Overview

### 1. souphttpsrc (Traditional Manual Approach)
```
souphttpsrc → hlsdemux → tsdemux → [video/audio queues] → [processing] → muxer → udpsink
```

**Elements Used:**
- `souphttpsrc`: HTTP source for downloading HLS segments
- `hlsdemux2`/`hlsdemux`: HLS playlist parsing and segment management
- `tsdemux`: MPEG-TS stream demuxing
- Video/audio queues, parsers, decoders, converters, encoders

**Characteristics:**
- Maximum control over each pipeline stage
- Explicit error handling and debugging
- Manual pad management with detailed logging
- Best for production environments requiring fine-tuned control

### 2. urisourcebin (Semi-Automatic Balanced Approach)
```
urisourcebin → [video/audio processing] → muxer → udpsink
```

**Elements Used:**
- `urisourcebin`: Handles source + demuxing internally
- Video/audio converters, encoders (may skip decoders if raw output)
- **No separate demux elements needed**

**Key Features:**
- Internal source selection and demuxing
- Can output either raw or compressed streams
- Automatic format negotiation
- Dynamic pad creation based on stream content

**Linking Logic:**
```go
// urisourcebin can output:
// - Raw streams (video/x-raw, audio/x-raw) → direct to converters
// - Compressed streams (video/x-h264, audio/mpeg) → to decoder chains
```

### 3. playbin3 (High-Level Automatic Approach)
```
playbin3 → [video/audio processing] → muxer → udpsink
```

**Elements Used:**
- `playbin3`: Complete media player with internal source/demux/decode
- Video/audio converters, encoders (typically skips decoders)
- **No separate source, demux, or decode elements needed**

**Key Features:**
- Complete automatic pipeline management
- Typically outputs raw decoded streams
- Built-in adaptive streaming and quality selection
- Minimal configuration required

## Implementation Details

### Source Element Creation

Each source type creates only the elements it needs:

```go
func (p *Pipeline) createSourceElement(cfg *config.Config) error {
    switch cfg.Input.SourceType {
    case "playbin3":
        // Creates only playbin3, sets demux/tsdemux to nil
        return p.createPlaybin3Source(cfg)
    case "urisourcebin":
        // Creates only urisourcebin, sets demux/tsdemux to nil
        return p.createUrisourcebinSource(cfg)
    default:
        // Creates souphttpsrc + hlsdemux + tsdemux
        return p.createSouphttpsrcSource(cfg)
    }
}
```

### Dynamic Linking Strategy

Each source type has its own linking strategy:

#### souphttpsrc Linking
- Manual pad-added callbacks for hlsdemux and tsdemux
- Explicit media type detection and routing
- Fallback mechanisms for unknown media types

#### urisourcebin Linking
- Single pad-added callback on urisourcebin
- Smart detection of raw vs compressed streams
- Direct linking to appropriate processing stage

#### playbin3 Linking
- Pad-added callback for intercepting decoded streams
- Typically handles raw streams directly
- Minimal manual intervention required

### Error Handling and Debugging

Enhanced error handling includes:

1. **Caps Safety**: Always check `caps.GetSize() > 0` before accessing structures
2. **Detailed Logging**: Different log messages for each source type
3. **Fallback Linking**: Automatic retry with alternative linking strategies
4. **Pad State Monitoring**: Track linking success/failure for each pad

### Configuration Properties

#### urisourcebin Specific
```go
p.source.SetProperty("uri", cfg.Input.HLSUrl)
p.source.SetProperty("buffer-duration", int64(cfg.Input.BufferSize)*1000000)
p.source.SetProperty("connection-speed", uint64(cfg.Input.BufferSize/1024))
p.source.SetProperty("download-buffer-size", cfg.Input.BufferSize)
```

#### playbin3 Specific
```go
p.source.SetProperty("uri", cfg.Input.HLSUrl)
p.source.SetProperty("flags", 3) // Video + Audio only
p.source.SetProperty("connection-speed", uint64(cfg.Input.BufferSize/1024))
```

## Benefits of Clean Architecture

### 1. **Reduced Complexity**
- No unnecessary elements in pipeline
- Clear separation between source types
- Simplified debugging and maintenance

### 2. **Better Performance**
- Fewer elements = less overhead
- Direct linking where possible
- Optimized for each source type's strengths

### 3. **Improved Reliability**
- Each source type uses its optimal element set
- Reduced chance of linking conflicts
- Better error isolation

### 4. **Easier Maintenance**
- Clear code organization
- Source-specific logic contained in dedicated methods
- Easier to add new source types

## Troubleshooting Guide

### Common Issues and Solutions

#### "Internal data stream error" / "not-linked"
- **Cause**: Demux pads not properly linked to downstream elements
- **Solution**: Check pad-added callbacks and ensure proper media type detection
- **Debug**: Enable detailed logging to see pad creation and linking attempts

#### Empty Caps Errors
- **Cause**: Accessing caps structure before caps are fully negotiated
- **Solution**: Always check `caps.GetSize() > 0` before accessing structures
- **Prevention**: Use fallback linking based on pad names

#### Element Not Found
- **Cause**: Required GStreamer plugins not installed
- **Solution**: Install appropriate plugin packages
- **Check**: Use `gst-inspect-1.0 <element-name>` to verify availability

### Debugging Commands

```bash
# Enable GStreamer debug logging
export GST_DEBUG=3

# Check element availability
gst-inspect-1.0 urisourcebin
gst-inspect-1.0 playbin3
gst-inspect-1.0 hlsdemux2

# Test pipeline with gst-launch
gst-launch-1.0 urisourcebin uri=<HLS_URL> ! videoconvert ! autovideosink
```

## Performance Recommendations

### For Low Latency
- Use `souphttpsrc` with fine-tuned buffer settings
- Minimize buffer sizes and processing delays

### For Reliability
- Use `urisourcebin` for balanced approach
- Good automatic handling with some control

### For Simplicity
- Use `playbin3` for rapid deployment
- Minimal configuration required
- Built-in adaptive streaming

## Future Enhancements

1. **Dynamic Source Switching**: Runtime switching between source types
2. **Adaptive Quality**: Better integration with adaptive streaming features
3. **Custom Demuxers**: Support for additional container formats
4. **Performance Monitoring**: Real-time pipeline performance metrics
