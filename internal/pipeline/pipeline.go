package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-gst/go-glib/glib"
	"github.com/go-gst/go-gst/gst"
	"github.com/sirupsen/logrus"

	"video-graphic-overlay-gstreamer/internal/config"
)

// Pipeline represents a GStreamer pipeline for HLS input with graphic overlay and UDP output
type Pipeline struct {
	config   *config.Config
	logger   *logrus.Logger
	pipeline *gst.Pipeline
	bus      *gst.Bus
	loop     *glib.MainLoop
	mutex    sync.RWMutex
	running  bool

	// Pipeline elements
	source         *gst.Element // playbin3
	videoConv      *gst.Element // videoconvert
	audioConv      *gst.Element // audioconvert
	videoScale     *gst.Element // videoscale
	videoScaleCaps *gst.Element // caps filter after videoscale
	audioResamp    *gst.Element // audioresample
	audioRate      *gst.Element // audiorate for consistent timing
	overlay        *gst.Element // text/image overlay (optional)
	videoEnc       *gst.Element // video encoder
	audioEnc       *gst.Element // audio encoder
	videoEncQueue  *gst.Element // queue after video encoder
	audioEncQueue  *gst.Element // queue after audio encoder
	videoCaps      *gst.Element // caps filter for video
	audioCaps      *gst.Element // caps filter for audio
	mux            *gst.Element // muxer
	sink           *gst.Element // udpsink
}

// New creates a new pipeline instance
func New(cfg *config.Config, logger *logrus.Logger) (*Pipeline, error) {
	// Initialize GStreamer
	gst.Init(nil)

	p := &Pipeline{
		config: cfg,
		logger: logger,
		loop:   glib.NewMainLoop(glib.MainContextDefault(), false),
	}

	if err := p.buildPipeline(); err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %w", err)
	}

	return p, nil
}

// buildPipeline constructs the GStreamer pipeline programmatically
func (p *Pipeline) buildPipeline() error {
	// Create pipeline
	pipeline, err := gst.NewPipeline("video-overlay-pipeline")
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}
	p.pipeline = pipeline

	// Create and add all elements
	if err := p.createElements(); err != nil {
		return fmt.Errorf("failed to create elements: %w", err)
	}

	// Link all elements
	if err := p.linkElements(); err != nil {
		return fmt.Errorf("failed to link elements: %w", err)
	}

	// Get bus for message handling
	p.bus = p.pipeline.GetPipelineBus()

	return nil
}

// createElements creates all GStreamer elements and adds them to the pipeline
func (p *Pipeline) createElements() error {
	var err error
	cfg := p.config

	// Create playbin3 source element
	if err := p.createPlaybin3Source(cfg); err != nil {
		return fmt.Errorf("failed to create playbin3 source element: %w", err)
	}

	// Create video processing elements
	p.videoConv, err = gst.NewElement("videoconvert")
	if err != nil {
		return fmt.Errorf("failed to create videoconvert: %w", err)
	}

	p.videoScale, err = gst.NewElement("videoscale")
	if err != nil {
		return fmt.Errorf("failed to create videoscale: %w", err)
	}

	// Create caps filter for video scaling to ensure proper output resolution
	p.videoScaleCaps, err = gst.NewElement("capsfilter")
	if err != nil {
		return fmt.Errorf("failed to create video scale caps filter: %w", err)
	}

	// Set output video caps based on configuration or default to 1920x1080
	outputWidth := 1920
	outputHeight := 1080

	// Priority: Output config > Input preferred > Default
	if cfg.Output.Width > 0 && cfg.Output.Height > 0 {
		outputWidth = cfg.Output.Width
		outputHeight = cfg.Output.Height
		p.logger.Infof("Using output resolution from config: %dx%d", outputWidth, outputHeight)
	} else if cfg.Input.PreferredWidth > 0 && cfg.Input.PreferredHeight > 0 {
		outputWidth = cfg.Input.PreferredWidth
		outputHeight = cfg.Input.PreferredHeight
		p.logger.Infof("Using input preferred resolution: %dx%d", outputWidth, outputHeight)
	} else {
		p.logger.Infof("Using default output resolution: %dx%d", outputWidth, outputHeight)
	}

	videoCapsStr := fmt.Sprintf("video/x-raw,width=%d,height=%d", outputWidth, outputHeight)
	videoScaleCaps := gst.NewCapsFromString(videoCapsStr)
	if videoScaleCaps != nil {
		p.videoScaleCaps.SetProperty("caps", videoScaleCaps)
		p.logger.Infof("Setting video output resolution to %dx%d", outputWidth, outputHeight)
	}

	// Create audio processing elements
	p.audioConv, err = gst.NewElement("audioconvert")
	if err != nil {
		return fmt.Errorf("failed to create audioconvert: %w", err)
	}

	p.audioResamp, err = gst.NewElement("audioresample")
	if err != nil {
		return fmt.Errorf("failed to create audioresample: %w", err)
	}
	// Configure audio resampler for consistent output
	p.audioResamp.SetProperty("quality", 4) // Good quality resampling

	p.audioRate, err = gst.NewElement("audiorate")
	if err != nil {
		return fmt.Errorf("failed to create audiorate: %w", err)
	}

	// Create overlay element if enabled
	if cfg.Overlay.Enabled {
		switch cfg.Overlay.Type {
		case "text":
			p.overlay, err = gst.NewElement("textoverlay")
			if err != nil {
				return fmt.Errorf("failed to create textoverlay: %w", err)
			}
			// Configure text overlay
			p.overlay.SetProperty("text", cfg.Overlay.Text.Content)
			p.overlay.SetProperty("font-desc", fmt.Sprintf("%s %d", cfg.Overlay.Text.FontFamily, cfg.Overlay.Text.FontSize))
			p.overlay.SetProperty("color", parseColor(cfg.Overlay.Text.Color))
			p.overlay.SetProperty("halignment", "left")
			p.overlay.SetProperty("valignment", "top")
			p.overlay.SetProperty("xpad", cfg.Overlay.Position.X)
			p.overlay.SetProperty("ypad", cfg.Overlay.Position.Y)
			p.logger.Info("Text overlay configured successfully")
		case "image":
			p.overlay, err = gst.NewElement("gdkpixbufoverlay")
			if err != nil {
				return fmt.Errorf("failed to create gdkpixbufoverlay: %w", err)
			}
			p.overlay.SetProperty("location", cfg.Overlay.Image.Path)
			p.overlay.SetProperty("alpha", cfg.Overlay.Image.Alpha)
			p.overlay.SetProperty("offset-x", cfg.Overlay.Position.X)
			p.overlay.SetProperty("offset-y", cfg.Overlay.Position.Y)
			p.logger.Info("Image overlay configured successfully")
		}
	}

	// Create encoding elements
	p.videoEnc, err = p.createVideoEncoder(cfg.Output.VideoCodec, cfg.Output.Bitrate)
	if err != nil {
		return fmt.Errorf("failed to create video encoder: %w", err)
	}

	p.audioEnc, err = p.createAudioEncoder(cfg.Output.AudioCodec)
	if err != nil {
		return fmt.Errorf("failed to create audio encoder: %w", err)
	}

	// Create queues after encoders with increased buffering
	p.videoEncQueue, err = gst.NewElement("queue")
	if err != nil {
		return fmt.Errorf("failed to create video encoder queue: %w", err)
	}
	p.videoEncQueue.SetProperty("max-size-buffers", 300)
	p.videoEncQueue.SetProperty("max-size-time", uint64(3000000000)) // 3 seconds
	p.videoEncQueue.SetProperty("leaky", 2)                          // Drop old buffers when full

	p.audioEncQueue, err = gst.NewElement("queue")
	if err != nil {
		return fmt.Errorf("failed to create audio encoder queue: %w", err)
	}
	p.audioEncQueue.SetProperty("max-size-buffers", 300)
	p.audioEncQueue.SetProperty("max-size-time", uint64(3000000000)) // 3 seconds
	p.audioEncQueue.SetProperty("leaky", 2)                          // Drop old buffers when full

	// Create caps filters for proper format negotiation
	p.videoCaps, err = gst.NewElement("capsfilter")
	if err != nil {
		return fmt.Errorf("failed to create video caps filter: %w", err)
	}
	// Set video caps for H.264
	videoCaps := gst.NewCapsFromString("video/x-h264,stream-format=avc,alignment=au")
	if videoCaps != nil {
		p.videoCaps.SetProperty("caps", videoCaps)
	}

	p.audioCaps, err = gst.NewElement("capsfilter")
	if err != nil {
		return fmt.Errorf("failed to create audio caps filter: %w", err)
	}
	// Set audio caps for AAC
	audioCaps := gst.NewCapsFromString("audio/mpeg,mpegversion=4,stream-format=raw")
	if audioCaps != nil {
		p.audioCaps.SetProperty("caps", audioCaps)
	}

	// Create muxer
	p.mux, err = p.createMuxer(cfg.Output.Format)
	if err != nil {
		return fmt.Errorf("failed to create muxer: %w", err)
	}

	// Configure muxer for better streaming
	if cfg.Output.Format == "mpegts" {
		// Set properties for MPEG-TS muxer to improve streaming
		p.mux.SetProperty("alignment", 7)                // Align to 188 bytes (TS packet size)
		p.mux.SetProperty("latency", uint64(3000000000)) // 3 seconds latency to accommodate buffering
		p.mux.SetProperty("min-upstream-latency", uint64(0))
		// Ensure both video and audio are included in the program
		p.mux.SetProperty("prog-map", "program_map,video_0=0,audio_0=0")
	}

	// Create sink
	p.sink, err = gst.NewElement("udpsink")
	if err != nil {
		return fmt.Errorf("failed to create udpsink: %w", err)
	}
	p.sink.SetProperty("host", cfg.Output.Host)
	p.sink.SetProperty("port", cfg.Output.Port)
	// p.sink.SetProperty("sync", false)
	// p.sink.SetProperty("async", false)
	p.sink.SetProperty("buffer-size", 65536) // 64KB buffer for UDP

	// Add all elements to pipeline
	elements := []*gst.Element{
		p.source, p.videoConv, p.videoScale, p.videoScaleCaps,
		p.audioConv, p.audioResamp, p.audioRate,
		p.videoEnc, p.audioEnc, p.videoEncQueue, p.audioEncQueue, p.mux, p.sink,
	}

	if p.overlay != nil {
		elements = append(elements, p.overlay)
	}

	for _, element := range elements {
		if element != nil {
			if err := p.pipeline.Add(element); err != nil {
				return fmt.Errorf("failed to add element %s to pipeline: %w", element.GetName(), err)
			}
		}
	}

	return nil
}

// createPlaybin3Source creates a playbin3 element with external sinks for processing
func (p *Pipeline) createPlaybin3Source(cfg *config.Config) error {
	var err error

	// Parse master playlist if enabled
	finalURL := cfg.Input.HLSUrl
	if cfg.Input.ParseMasterPlaylist {
		playlist, err := ParseHLSMasterPlaylist(cfg.Input.HLSUrl, p.logger)
		if err != nil {
			p.logger.Warnf("Failed to parse master playlist, using original URL: %v", err)
		} else {
			// Select best stream based on configuration
			selection := cfg.Input.StreamSelection
			if selection == "" {
				selection = "highest"
			}

			bestStream := playlist.SelectBestStream(selection)
			if bestStream != nil {
				finalURL = bestStream.URL
				p.logger.Infof("Selected stream: %dx%d, %d bps (%s)",
					bestStream.Width, bestStream.Height,
					bestStream.Bandwidth, selection)

				// Update preferred resolution if not set
				if cfg.Input.PreferredWidth == 0 && cfg.Input.PreferredHeight == 0 {
					cfg.Input.PreferredWidth = bestStream.Width
					cfg.Input.PreferredHeight = bestStream.Height
					p.logger.Infof("Updated preferred resolution to %dx%d",
						bestStream.Width, bestStream.Height)
				}
			} else {
				p.logger.Warnf("No suitable stream found, using original URL")
			}
		}
	}

	// Create playbin3 element - it handles source, demuxing, and decoding internally
	p.source, err = gst.NewElement("playbin3")
	if err != nil {
		return fmt.Errorf("failed to create playbin3: %w", err)
	}

	// Configure playbin3
	p.source.SetProperty("uri", finalURL)

	// Set flags to enable video and audio, disable text/subtitles
	// GST_PLAY_FLAG_VIDEO (1) + GST_PLAY_FLAG_AUDIO (2) + GST_PLAY_FLAG_BUFFERING (16) = 19
	// Removed native flags to improve compatibility with adaptive streams
	p.source.SetProperty("flags", 19)

	// Configure buffering for better streaming performance with increased latency tolerance
	p.source.SetProperty("buffer-duration", int64(5000000000))                  // 5 seconds buffer duration
	p.source.SetProperty("buffer-size", cfg.Input.BufferSize*2)                 // Double the buffer size
	p.source.SetProperty("connection-speed", uint64(cfg.Input.BufferSize/1024)) // Connection speed in kbps

	// Create intervideosink and interaudiosink for external processing
	videoSink, err := gst.NewElement("intervideosink")
	if err != nil {
		return fmt.Errorf("failed to create intervideosink: %w", err)
	}
	videoSink.SetProperty("channel", "video-channel")
	videoSink.SetProperty("max-lateness", int64(3000000000)) // 3 seconds max lateness

	audioSink, err := gst.NewElement("interaudiosink")
	if err != nil {
		return fmt.Errorf("failed to create interaudiosink: %w", err)
	}
	audioSink.SetProperty("channel", "audio-channel")
	audioSink.SetProperty("max-lateness", int64(3000000000)) // 3 seconds max lateness

	// Set the external sinks on playbin3
	p.source.SetProperty("video-sink", videoSink)
	p.source.SetProperty("audio-sink", audioSink)

	// Add stream selection callback to handle adaptive streams
	p.source.Connect("stream-notify::stream-collection", func(element *gst.Element, pspec *glib.ParamSpec) {
		p.logger.Info("Stream collection updated, selecting streams")
		p.selectStreams()
	})

	p.logger.Info("Using playbin3 with external sinks for HLS streaming and processing")

	return nil
}

// linkElements links all GStreamer elements in the pipeline
func (p *Pipeline) linkElements() error {
	return p.linkPlaybin3Elements()
}

// linkPlaybin3Elements links elements for the playbin3 approach using intervideo/interaudio
func (p *Pipeline) linkPlaybin3Elements() error {
	// Create intervideo and interaudio sources to receive data from playbin3 sinks
	videoSrc, err := gst.NewElement("intervideosrc")
	if err != nil {
		return fmt.Errorf("failed to create intervideosrc: %w", err)
	}
	videoSrc.SetProperty("channel", "video-channel")
	videoSrc.SetProperty("timeout", uint64(3000000000)) // 3 seconds timeout

	audioSrc, err := gst.NewElement("interaudiosrc")
	if err != nil {
		return fmt.Errorf("failed to create interaudiosrc: %w", err)
	}
	audioSrc.SetProperty("channel", "audio-channel")
	audioSrc.SetProperty("timeout", uint64(3000000000)) // 3 seconds timeout

	// Add inter sources to pipeline
	if err := p.pipeline.Add(videoSrc); err != nil {
		return fmt.Errorf("failed to add intervideosrc to pipeline: %w", err)
	}
	if err := p.pipeline.Add(audioSrc); err != nil {
		return fmt.Errorf("failed to add interaudiosrc to pipeline: %w", err)
	}

	// Link video processing chain: intervideosrc -> videoConv -> videoScale -> overlay -> videoEnc -> videoEncQueue -> mux
	if err := videoSrc.Link(p.videoConv); err != nil {
		return fmt.Errorf("failed to link intervideosrc to video converter: %w", err)
	}

	// Link audio processing chain: interaudiosrc -> audioConv -> audioResamp -> audioRate -> audioEnc -> audioEncQueue -> mux
	if err := audioSrc.Link(p.audioConv); err != nil {
		return fmt.Errorf("failed to link interaudiosrc to audio converter: %w", err)
	}

	// Link video processing elements
	elements := []*gst.Element{p.videoConv, p.videoScale, p.videoScaleCaps}
	if p.overlay != nil {
		elements = append(elements, p.overlay)
	}
	elements = append(elements, p.videoEnc, p.videoEncQueue)

	for i := 0; i < len(elements)-1; i++ {
		if err := elements[i].Link(elements[i+1]); err != nil {
			return fmt.Errorf("failed to link video elements %s to %s: %w",
				elements[i].GetName(), elements[i+1].GetName(), err)
		}
	}

	// Link audio processing elements
	audioElements := []*gst.Element{p.audioConv, p.audioResamp, p.audioRate, p.audioEnc, p.audioEncQueue}
	for i := 0; i < len(audioElements)-1; i++ {
		if err := audioElements[i].Link(audioElements[i+1]); err != nil {
			return fmt.Errorf("failed to link audio elements %s to %s: %w",
				audioElements[i].GetName(), audioElements[i+1].GetName(), err)
		}
	}

	// Link video encoder queue to muxer
	if err := p.videoEncQueue.Link(p.mux); err != nil {
		return fmt.Errorf("failed to link video encoder queue to muxer: %w", err)
	}

	// Link audio encoder queue to muxer
	if err := p.audioEncQueue.Link(p.mux); err != nil {
		return fmt.Errorf("failed to link audio encoder queue to muxer: %w", err)
	}

	// Link muxer to sink
	if err := p.mux.Link(p.sink); err != nil {
		return fmt.Errorf("failed to link mux to sink: %w", err)
	}

	p.logger.Info("Playbin3 with intervideo/interaudio linking completed successfully")

	return nil
}

// createVideoEncoder creates a video encoder based on codec type
func (p *Pipeline) createVideoEncoder(codec string, bitrate int) (*gst.Element, error) {
	switch codec {
	case "h264":
		enc, err := gst.NewElement("x264enc")
		if err != nil {
			return nil, err
		}
		enc.SetProperty("bitrate", bitrate/1000) // x264enc expects kbps
		enc.SetProperty("tune", "zerolatency")
		enc.SetProperty("speed-preset", "ultrafast")
		enc.SetProperty("key-int-max", 30)
		enc.SetProperty("bframes", 0)
		return enc, nil
	case "h265":
		enc, err := gst.NewElement("x265enc")
		if err != nil {
			return nil, err
		}
		enc.SetProperty("bitrate", bitrate/1000)
		enc.SetProperty("tune", "zerolatency")
		enc.SetProperty("speed-preset", "ultrafast")
		enc.SetProperty("key-int-max", 30)
		return enc, nil
	case "vp8":
		enc, err := gst.NewElement("vp8enc")
		if err != nil {
			return nil, err
		}
		enc.SetProperty("target-bitrate", bitrate)
		enc.SetProperty("deadline", 1)
		enc.SetProperty("cpu-used", 16)
		enc.SetProperty("keyframe-max-dist", 30)
		return enc, nil
	case "vp9":
		enc, err := gst.NewElement("vp9enc")
		if err != nil {
			return nil, err
		}
		enc.SetProperty("target-bitrate", bitrate)
		enc.SetProperty("deadline", 1)
		enc.SetProperty("cpu-used", 8)
		enc.SetProperty("keyframe-max-dist", 30)
		return enc, nil
	default:
		// Default to H.264
		enc, err := gst.NewElement("x264enc")
		if err != nil {
			return nil, err
		}
		enc.SetProperty("bitrate", bitrate/1000)
		enc.SetProperty("tune", "zerolatency")
		enc.SetProperty("speed-preset", "ultrafast")
		return enc, nil
	}
}

// createAudioEncoder creates an audio encoder based on codec type
func (p *Pipeline) createAudioEncoder(codec string) (*gst.Element, error) {
	switch codec {
	case "aac":
		enc, err := gst.NewElement("avenc_aac")
		if err != nil {
			return nil, err
		}
		enc.SetProperty("bitrate", 128000)
		enc.SetProperty("compliance", -2) // Allow experimental features
		return enc, nil
	case "mp3":
		enc, err := gst.NewElement("lamemp3enc")
		if err != nil {
			return nil, err
		}
		enc.SetProperty("bitrate", 128)
		return enc, nil
	case "opus":
		enc, err := gst.NewElement("opusenc")
		if err != nil {
			return nil, err
		}
		enc.SetProperty("bitrate", 128000)
		return enc, nil
	case "vorbis":
		enc, err := gst.NewElement("vorbisenc")
		if err != nil {
			return nil, err
		}
		enc.SetProperty("bitrate", 128000)
		return enc, nil
	default:
		// Default to AAC
		enc, err := gst.NewElement("avenc_aac")
		if err != nil {
			return nil, err
		}
		enc.SetProperty("bitrate", 128000)
		return enc, nil
	}
}

// createMuxer creates a muxer based on format type
func (p *Pipeline) createMuxer(format string) (*gst.Element, error) {
	switch format {
	case "mpegts":
		return gst.NewElement("mpegtsmux")
	case "mp4":
		return gst.NewElement("mp4mux")
	case "webm":
		return gst.NewElement("webmmux")
	case "mkv":
		return gst.NewElement("matroskamux")
	case "flv":
		return gst.NewElement("flvmux")
	default:
		// Default to MPEG-TS for UDP streaming
		return gst.NewElement("mpegtsmux")
	}
}

// parseColor converts color string to uint32 (simplified implementation)
func parseColor(colorStr string) uint32 {
	// Simple color mapping - in a real implementation you'd parse hex/rgb values
	switch colorStr {
	case "white":
		return 0xFFFFFFFF
	case "black":
		return 0xFF000000
	case "red":
		return 0xFFFF0000
	case "green":
		return 0xFF00FF00
	case "blue":
		return 0xFF0000FF
	case "yellow":
		return 0xFFFFFF00
	default:
		return 0xFFFFFFFF // Default to white
	}
}

// Start starts the pipeline
func (p *Pipeline) Start(ctx context.Context) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		return fmt.Errorf("pipeline is already running")
	}

	p.logger.Info("Starting pipeline...")

	// Set pipeline to playing state
	p.pipeline.SetState(gst.StatePlaying)

	p.running = true

	// Start message handling in a separate goroutine
	go p.handleMessages(ctx)

	// Run main loop in a separate goroutine
	go func() {
		p.loop.Run()
	}()

	p.logger.Info("Pipeline started successfully")
	return nil
}

// Stop stops the pipeline
func (p *Pipeline) Stop() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.running {
		return nil
	}

	p.logger.Info("Stopping pipeline...")

	// Set pipeline to null state
	p.pipeline.SetState(gst.StateNull)

	// Quit main loop
	p.loop.Quit()

	p.running = false
	p.logger.Info("Pipeline stopped")

	return nil
}

// handleMessages handles GStreamer bus messages
func (p *Pipeline) handleMessages(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg := p.bus.TimedPop(gst.ClockTime(100 * time.Millisecond))
			if msg == nil {
				continue
			}

			switch msg.Type() {
			case gst.MessageEOS:
				p.logger.Info("End of stream received")
				return
			case gst.MessageError:
				err := msg.ParseError()
				p.logger.Errorf("Pipeline error: %s", err.Error())
				if debug := err.DebugString(); debug != "" {
					p.logger.Errorf("Debug: %s", debug)
				}
				return
			case gst.MessageWarning:
				err := msg.ParseWarning()
				p.logger.Warnf("Pipeline warning: %s", err.Error())
				if debug := err.DebugString(); debug != "" {
					p.logger.Warnf("Debug: %s", debug)
				}
			case gst.MessageInfo:
				err := msg.ParseInfo()
				p.logger.Infof("Pipeline info: %s", err.Error())
				if debug := err.DebugString(); debug != "" {
					p.logger.Infof("Debug: %s", debug)
				}
			case gst.MessageStateChanged:
				if msg.Source() == p.pipeline.GetName() {
					oldState, newState := msg.ParseStateChanged()
					p.logger.Debugf("Pipeline state changed from %s to %s",
						oldState.String(), newState.String())
				}
			case gst.MessageStreamCollection:
				p.logger.Info("Stream collection message received")
				// Handle stream collection updates for adaptive streaming
				p.selectStreams()
			case gst.MessageStreamsSelected:
				p.logger.Info("Streams selected message received")
			}

			msg.Unref()
		}
	}
}

// IsRunning returns whether the pipeline is currently running
func (p *Pipeline) IsRunning() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.running
}

// selectStreams handles stream selection for adaptive streaming
func (p *Pipeline) selectStreams() {
	// Get the stream collection from playbin3
	streamCollection, err := p.source.GetProperty("stream-collection")
	if err != nil || streamCollection == nil {
		p.logger.Warn("No stream collection available")
		return
	}

	p.logger.Info("Selecting best video and audio streams from collection")

	// For now, let playbin3 auto-select streams
	// In a more advanced implementation, you could iterate through streams
	// and select based on bitrate, resolution, etc.
	p.source.SetProperty("current-video", -1) // Auto-select video
	p.source.SetProperty("current-audio", -1) // Auto-select audio
}
