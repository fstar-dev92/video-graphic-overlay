package pipeline

import (
	"fmt"
	"net"
	"strconv"

	"video-graphic-overlay-gstreamer/internal/config"
)

// UDPOutput handles UDP stream output
type UDPOutput struct {
	config *config.OutputConfig
}

// NewUDPOutput creates a new UDP output handler
func NewUDPOutput(cfg *config.OutputConfig) (*UDPOutput, error) {
	// Validate configuration
	if err := validateUDPConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid UDP configuration: %w", err)
	}

	return &UDPOutput{
		config: cfg,
	}, nil
}

// GetPipelineString returns the pipeline string for UDP output
func (u *UDPOutput) GetPipelineString() string {
	// Video encoding pipeline
	videoEncode := u.getVideoEncodingString()
	
	// Audio encoding pipeline
	audioEncode := u.getAudioEncodingString()
	
	// Muxer
	muxer := u.getMuxerString()
	
	// UDP sink
	udpSink := u.getUDPSinkString()

	return fmt.Sprintf("%s ! queue name=video_queue ! mux.video_0 "+
		"%s ! queue name=audio_queue ! mux.audio_0 "+
		"%s name=mux ! %s",
		videoEncode, audioEncode, muxer, udpSink)
}

// getVideoEncodingString returns video encoding pipeline string
func (u *UDPOutput) getVideoEncodingString() string {
	switch u.config.VideoCodec {
	case "h264":
		return fmt.Sprintf("x264enc bitrate=%d tune=zerolatency speed-preset=ultrafast "+
			"key-int-max=30 bframes=0 "+
			"option-string=\"slice-max-size=1200:vbv-maxrate=%d:vbv-bufsize=%d\"",
			u.config.Bitrate/1000,
			u.config.Bitrate/1000,
			u.config.Bitrate/1000*2)
	case "h265":
		return fmt.Sprintf("x265enc bitrate=%d tune=zerolatency speed-preset=ultrafast "+
			"key-int-max=30",
			u.config.Bitrate/1000)
	case "vp8":
		return fmt.Sprintf("vp8enc target-bitrate=%d deadline=1 cpu-used=16 "+
			"keyframe-max-dist=30",
			u.config.Bitrate)
	case "vp9":
		return fmt.Sprintf("vp9enc target-bitrate=%d deadline=1 cpu-used=8 "+
			"keyframe-max-dist=30",
			u.config.Bitrate)
	default:
		// Default to H.264
		return fmt.Sprintf("x264enc bitrate=%d tune=zerolatency speed-preset=ultrafast",
			u.config.Bitrate/1000)
	}
}

// getAudioEncodingString returns audio encoding pipeline string
func (u *UDPOutput) getAudioEncodingString() string {
	switch u.config.AudioCodec {
	case "aac":
		return "avenc_aac bitrate=128000"
	case "mp3":
		return "lamemp3enc bitrate=128"
	case "opus":
		return "opusenc bitrate=128000"
	case "vorbis":
		return "vorbisenc bitrate=128000"
	default:
		// Default to AAC
		return "avenc_aac bitrate=128000"
	}
}

// getMuxerString returns muxer pipeline string
func (u *UDPOutput) getMuxerString() string {
	switch u.config.Format {
	case "mpegts":
		return "mpegtsmux"
	case "mp4":
		return "mp4mux"
	case "webm":
		return "webmmux"
	case "mkv":
		return "matroskamux"
	case "flv":
		return "flvmux"
	default:
		// Default to MPEG-TS for UDP streaming
		return "mpegtsmux"
	}
}

// getUDPSinkString returns UDP sink pipeline string
func (u *UDPOutput) getUDPSinkString() string {
	return fmt.Sprintf("udpsink host=%s port=%d sync=false async=false",
		u.config.Host, u.config.Port)
}

// validateUDPConfig validates UDP output configuration
func validateUDPConfig(cfg *config.OutputConfig) error {
	// Validate host
	if cfg.Host == "" {
		return fmt.Errorf("host cannot be empty")
	}

	// Validate IP address
	if net.ParseIP(cfg.Host) == nil {
		// Try to resolve hostname
		if _, err := net.LookupHost(cfg.Host); err != nil {
			return fmt.Errorf("invalid host: %w", err)
		}
	}

	// Validate port
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	// Validate bitrate
	if cfg.Bitrate < 100000 || cfg.Bitrate > 50000000 {
		return fmt.Errorf("bitrate must be between 100kbps and 50Mbps")
	}

	return nil
}

// MulticastUDPOutput handles multicast UDP output
type MulticastUDPOutput struct {
	*UDPOutput
	multicastGroup string
	ttl            int
}

// NewMulticastUDPOutput creates a new multicast UDP output handler
func NewMulticastUDPOutput(cfg *config.OutputConfig, multicastGroup string, ttl int) (*MulticastUDPOutput, error) {
	base, err := NewUDPOutput(cfg)
	if err != nil {
		return nil, err
	}

	// Validate multicast group
	if !isMulticastIP(multicastGroup) {
		return nil, fmt.Errorf("invalid multicast group: %s", multicastGroup)
	}

	return &MulticastUDPOutput{
		UDPOutput:      base,
		multicastGroup: multicastGroup,
		ttl:            ttl,
	}, nil
}

// GetPipelineString returns the pipeline string for multicast UDP output
func (m *MulticastUDPOutput) GetPipelineString() string {
	// Get base pipeline string
	basePipeline := m.UDPOutput.GetPipelineString()
	
	// Replace udpsink with multicast-specific settings
	udpSink := fmt.Sprintf("udpsink host=%s port=%d multicast-iface=eth0 "+
		"ttl-mc=%d auto-multicast=true sync=false async=false",
		m.multicastGroup, m.config.Port, m.ttl)
	
	// Replace the last udpsink in the pipeline
	return replaceLast(basePipeline, "udpsink host="+m.config.Host+" port="+strconv.Itoa(m.config.Port)+" sync=false async=false", udpSink)
}

// isMulticastIP checks if an IP address is in the multicast range
func isMulticastIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// IPv4 multicast range: 224.0.0.0 to 239.255.255.255
	if parsedIP.To4() != nil {
		return parsedIP[0] >= 224 && parsedIP[0] <= 239
	}

	// IPv6 multicast range: ff00::/8
	if parsedIP.To16() != nil {
		return parsedIP[0] == 0xff
	}

	return false
}

// replaceLast replaces the last occurrence of old with new in s
func replaceLast(s, old, new string) string {
	i := len(s) - len(old)
	for i >= 0 {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
		i--
	}
	return s
}

// RTMPOutput handles RTMP output (alternative to UDP)
type RTMPOutput struct {
	config  *config.OutputConfig
	rtmpURL string
}

// NewRTMPOutput creates a new RTMP output handler
func NewRTMPOutput(cfg *config.OutputConfig, rtmpURL string) (*RTMPOutput, error) {
	return &RTMPOutput{
		config:  cfg,
		rtmpURL: rtmpURL,
	}, nil
}

// GetPipelineString returns the pipeline string for RTMP output
func (r *RTMPOutput) GetPipelineString() string {
	// Video encoding (optimized for RTMP)
	videoEncode := fmt.Sprintf("x264enc bitrate=%d tune=zerolatency speed-preset=veryfast "+
		"key-int-max=60 bframes=0",
		r.config.Bitrate/1000)
	
	// Audio encoding
	audioEncode := "avenc_aac bitrate=128000"
	
	// FLV muxer for RTMP
	muxer := "flvmux streamable=true"
	
	// RTMP sink
	rtmpSink := fmt.Sprintf("rtmpsink location=%s sync=false", r.rtmpURL)

	return fmt.Sprintf("%s ! queue ! mux.video "+
		"%s ! queue ! mux.audio "+
		"%s name=mux ! %s",
		videoEncode, audioEncode, muxer, rtmpSink)
}
