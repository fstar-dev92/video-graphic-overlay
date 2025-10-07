package pipeline

import (
	"context"
	"fmt"
	"strings"
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
	source        *gst.Element // souphttpsrc
	demux         *gst.Element // hlsdemux
	tsdemux       *gst.Element // tsdemux for MPEG-TS streams
	videoQueue    *gst.Element // queue for video
	audioQueue    *gst.Element // queue for audio
	videoDecode   *gst.Element // decodebin for video
	audioDecode   *gst.Element // decodebin for audio
	videoConv     *gst.Element // videoconvert
	audioConv     *gst.Element // audioconvert
	videoScale    *gst.Element // videoscale
	audioResamp   *gst.Element // audioresample
	overlay       *gst.Element // text/image overlay (optional)
	videoEnc      *gst.Element // video encoder
	audioEnc      *gst.Element // audio encoder
	videoEncQueue *gst.Element // queue after video encoder
	audioEncQueue *gst.Element // queue after audio encoder
	mux           *gst.Element // muxer
	sink          *gst.Element // udpsink
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

	// Use improved souphttpsrc + hlsdemux approach (most reliable for streaming)
	p.source, err = gst.NewElement("souphttpsrc")
	if err != nil {
		return fmt.Errorf("failed to create souphttpsrc: %w", err)
	}

	// Configure souphttpsrc with improved settings
	p.source.SetProperty("location", cfg.Input.HLSUrl)
	p.source.SetProperty("timeout", cfg.Input.Timeout)
	p.source.SetProperty("retries", cfg.Input.ConnectionRetry)
	p.source.SetProperty("user-agent", "GStreamer-HLS-Overlay/1.0")
	p.source.SetProperty("automatic-redirect", true)
	p.source.SetProperty("keep-alive", true)
	p.source.SetProperty("compress", false)
	// Add SSL/TLS settings for better HTTPS handling
	p.source.SetProperty("ssl-strict", false)
	p.logger.Info("Using improved souphttpsrc + hlsdemux for HLS streaming")

	// Create demux element (hlsdemux)
	p.demux, err = gst.NewElement("hlsdemux")
	if err != nil {
		return fmt.Errorf("failed to create hlsdemux: %w", err)
	}
	p.demux.SetProperty("connection-speed", uint(cfg.Input.BufferSize/1024))
	// Set additional properties for better HLS handling
	p.demux.SetProperty("start-bitrate", uint(cfg.Output.Bitrate/1000)) // Convert to kbps

	// Create transport stream demux for MPEG-TS streams
	p.tsdemux, err = gst.NewElement("tsdemux")
	if err != nil {
		return fmt.Errorf("failed to create tsdemux: %w", err)
	}

	// Create video processing elements
	p.videoQueue, err = gst.NewElement("queue")
	if err != nil {
		return fmt.Errorf("failed to create video queue: %w", err)
	}
	p.videoQueue.SetProperty("max-size-buffers", 100)
	p.videoQueue.SetProperty("max-size-time", uint64(1000000000)) // 1 second

	p.videoDecode, err = gst.NewElement("decodebin")
	if err != nil {
		return fmt.Errorf("failed to create video decodebin: %w", err)
	}

	p.videoConv, err = gst.NewElement("videoconvert")
	if err != nil {
		return fmt.Errorf("failed to create videoconvert: %w", err)
	}

	p.videoScale, err = gst.NewElement("videoscale")
	if err != nil {
		return fmt.Errorf("failed to create videoscale: %w", err)
	}

	// Create audio processing elements
	p.audioQueue, err = gst.NewElement("queue")
	if err != nil {
		return fmt.Errorf("failed to create audio queue: %w", err)
	}
	p.audioQueue.SetProperty("max-size-buffers", 100)
	p.audioQueue.SetProperty("max-size-time", uint64(1000000000)) // 1 second

	p.audioDecode, err = gst.NewElement("decodebin")
	if err != nil {
		return fmt.Errorf("failed to create audio decodebin: %w", err)
	}

	p.audioConv, err = gst.NewElement("audioconvert")
	if err != nil {
		return fmt.Errorf("failed to create audioconvert: %w", err)
	}

	p.audioResamp, err = gst.NewElement("audioresample")
	if err != nil {
		return fmt.Errorf("failed to create audioresample: %w", err)
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
		case "image":
			p.overlay, err = gst.NewElement("gdkpixbufoverlay")
			if err != nil {
				return fmt.Errorf("failed to create gdkpixbufoverlay: %w", err)
			}
			p.overlay.SetProperty("location", cfg.Overlay.Image.Path)
			p.overlay.SetProperty("alpha", cfg.Overlay.Image.Alpha)
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

	// Create queues after encoders
	p.videoEncQueue, err = gst.NewElement("queue")
	if err != nil {
		return fmt.Errorf("failed to create video encoder queue: %w", err)
	}

	p.audioEncQueue, err = gst.NewElement("queue")
	if err != nil {
		return fmt.Errorf("failed to create audio encoder queue: %w", err)
	}

	// Create muxer
	p.mux, err = p.createMuxer(cfg.Output.Format)
	if err != nil {
		return fmt.Errorf("failed to create muxer: %w", err)
	}

	// Create sink
	p.sink, err = gst.NewElement("udpsink")
	if err != nil {
		return fmt.Errorf("failed to create udpsink: %w", err)
	}
	p.sink.SetProperty("host", cfg.Output.Host)
	p.sink.SetProperty("port", cfg.Output.Port)
	p.sink.SetProperty("sync", false)
	p.sink.SetProperty("async", false)

	// Add all elements to pipeline
	elements := []*gst.Element{
		p.source, p.demux, p.tsdemux, p.videoQueue, p.videoDecode, p.videoConv, p.videoScale,
		p.audioQueue, p.audioDecode, p.audioConv, p.audioResamp,
		p.videoEnc, p.audioEnc, p.videoEncQueue, p.audioEncQueue, p.mux, p.sink,
	}

	if p.overlay != nil {
		elements = append(elements, p.overlay)
	}

	for _, element := range elements {
		if err := p.pipeline.Add(element); err != nil {
			return fmt.Errorf("failed to add element %s to pipeline: %w", element.GetName(), err)
		}
	}

	return nil
}

// linkElements links all GStreamer elements in the pipeline
func (p *Pipeline) linkElements() error {
	// Link souphttpsrc to hlsdemux
	if err := p.source.Link(p.demux); err != nil {
		return fmt.Errorf("failed to link source to demux: %w", err)
	}

	// Set up pad-added callback for demux (HLS demux creates pads dynamically)
	p.demux.Connect("pad-added", func(element *gst.Element, pad *gst.Pad) {
		padName := pad.GetName()
		p.logger.Infof("New pad added: %s", padName)

		// Get pad capabilities to determine media type
		caps := pad.GetCurrentCaps()
		if caps == nil {
			caps = pad.QueryCaps(nil)
		}

		if caps != nil {
			structure := caps.GetStructureAt(0)
			if structure != nil {
				mediaType := structure.Name()
				p.logger.Infof("Pad %s has media type: %s", padName, mediaType)

				if strings.HasPrefix(mediaType, "video/mpegts") || strings.HasPrefix(mediaType, "video/mp2t") {
					// MPEG-TS stream - link to tsdemux for further demuxing
					sinkPad := p.tsdemux.GetStaticPad("sink")
					if sinkPad != nil && !sinkPad.IsLinked() {
						if linkReturn := pad.Link(sinkPad); linkReturn != gst.PadLinkOK {
							p.logger.Errorf("Failed to link demux MPEG-TS pad %s: %v", padName, linkReturn)
						} else {
							p.logger.Infof("Successfully linked MPEG-TS pad %s to tsdemux", padName)
						}
						sinkPad.Unref()
					} else {
						p.logger.Warnf("TS demux sink pad not available or already linked for pad %s", padName)
					}
				} else if strings.HasPrefix(mediaType, "video/") {
					// Direct video stream - link to video queue
					sinkPad := p.videoQueue.GetStaticPad("sink")
					if sinkPad != nil && !sinkPad.IsLinked() {
						if linkReturn := pad.Link(sinkPad); linkReturn != gst.PadLinkOK {
							p.logger.Errorf("Failed to link demux video pad %s: %v", padName, linkReturn)
						} else {
							p.logger.Infof("Successfully linked video pad %s", padName)
						}
						sinkPad.Unref()
					} else {
						p.logger.Warnf("Video sink pad not available or already linked for pad %s", padName)
					}
				} else if strings.HasPrefix(mediaType, "audio/") {
					// Direct audio stream - link to audio queue
					sinkPad := p.audioQueue.GetStaticPad("sink")
					if sinkPad != nil && !sinkPad.IsLinked() {
						if linkReturn := pad.Link(sinkPad); linkReturn != gst.PadLinkOK {
							p.logger.Errorf("Failed to link demux audio pad %s: %v", padName, linkReturn)
						} else {
							p.logger.Infof("Successfully linked audio pad %s", padName)
						}
						sinkPad.Unref()
					} else {
						p.logger.Warnf("Audio sink pad not available or already linked for pad %s", padName)
					}
				}
			}
			caps.Unref()
		} else {
			p.logger.Warnf("Could not get capabilities for pad %s", padName)
		}
	})

	// Set up no-more-pads callback to know when all pads have been created
	p.demux.Connect("no-more-pads", func(element *gst.Element) {
		p.logger.Info("HLS demux finished creating all pads")
	})

	// Set up pad-added callback for tsdemux (handles MPEG-TS demuxing)
	p.tsdemux.Connect("pad-added", func(element *gst.Element, pad *gst.Pad) {
		padName := pad.GetName()
		p.logger.Infof("TS demux new pad added: %s", padName)

		// Get pad capabilities to determine media type
		caps := pad.GetCurrentCaps()
		if caps == nil {
			caps = pad.QueryCaps(nil)
		}

		if caps != nil {
			structure := caps.GetStructureAt(0)
			if structure != nil {
				mediaType := structure.Name()
				p.logger.Infof("TS demux pad %s has media type: %s", padName, mediaType)

				if strings.HasPrefix(mediaType, "video/") {
					// Link video stream to video queue
					sinkPad := p.videoQueue.GetStaticPad("sink")
					if sinkPad != nil && !sinkPad.IsLinked() {
						if linkReturn := pad.Link(sinkPad); linkReturn != gst.PadLinkOK {
							p.logger.Errorf("Failed to link TS demux video pad %s: %v", padName, linkReturn)
						} else {
							p.logger.Infof("Successfully linked TS demux video pad %s", padName)
						}
						sinkPad.Unref()
					} else {
						p.logger.Warnf("Video sink pad not available or already linked for TS demux pad %s", padName)
					}
				} else if strings.HasPrefix(mediaType, "audio/") {
					// Link audio stream to audio queue
					sinkPad := p.audioQueue.GetStaticPad("sink")
					if sinkPad != nil && !sinkPad.IsLinked() {
						if linkReturn := pad.Link(sinkPad); linkReturn != gst.PadLinkOK {
							p.logger.Errorf("Failed to link TS demux audio pad %s: %v", padName, linkReturn)
						} else {
							p.logger.Infof("Successfully linked TS demux audio pad %s", padName)
						}
						sinkPad.Unref()
					} else {
						p.logger.Warnf("Audio sink pad not available or already linked for TS demux pad %s", padName)
					}
				}
			}
			caps.Unref()
		} else {
			p.logger.Warnf("Could not get capabilities for TS demux pad %s", padName)
		}
	})

	// Set up no-more-pads callback for tsdemux
	p.tsdemux.Connect("no-more-pads", func(element *gst.Element) {
		p.logger.Info("TS demux finished creating all pads")
	})

	// Link video processing chain
	if err := p.linkVideoChain(); err != nil {
		return fmt.Errorf("failed to link video chain: %w", err)
	}

	// Link audio processing chain
	if err := p.linkAudioChain(); err != nil {
		return fmt.Errorf("failed to link audio chain: %w", err)
	}

	// Link muxer to sink
	if err := p.mux.Link(p.sink); err != nil {
		return fmt.Errorf("failed to link mux to sink: %w", err)
	}

	return nil
}

// linkVideoChain links the video processing elements
func (p *Pipeline) linkVideoChain() error {
	// Set up pad-added callback for video decodebin
	p.videoDecode.Connect("pad-added", func(element *gst.Element, pad *gst.Pad) {
		padName := pad.GetName()
		p.logger.Infof("Video decoder new pad added: %s", padName)

		caps := pad.GetCurrentCaps()
		if caps != nil {
			structure := caps.GetStructureAt(0)
			if structure != nil {
				mediaType := structure.Name()
				p.logger.Infof("Video decoder pad %s has media type: %s", padName, mediaType)

				if strings.HasPrefix(mediaType, "video/") {
					sinkPad := p.videoConv.GetStaticPad("sink")
					if sinkPad != nil && !sinkPad.IsLinked() {
						if linkReturn := pad.Link(sinkPad); linkReturn != gst.PadLinkOK {
							p.logger.Errorf("Failed to link video decode pad %s: %v", padName, linkReturn)
						} else {
							p.logger.Infof("Successfully linked video decoder pad %s", padName)
						}
						sinkPad.Unref()
					} else {
						p.logger.Warnf("Video converter sink pad not available or already linked for pad %s", padName)
					}
				}
			}
			caps.Unref()
		} else {
			p.logger.Warnf("Could not get capabilities for video decoder pad %s", padName)
		}
	})

	// Link video queue to decode
	if err := p.videoQueue.Link(p.videoDecode); err != nil {
		return fmt.Errorf("failed to link video queue to decode: %w", err)
	}

	// Link video processing elements
	elements := []*gst.Element{p.videoConv, p.videoScale}
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

	// Link video encoder queue to muxer
	if err := p.videoEncQueue.Link(p.mux); err != nil {
		return fmt.Errorf("failed to link video encoder queue to muxer: %w", err)
	}

	return nil
}

// linkAudioChain links the audio processing elements
func (p *Pipeline) linkAudioChain() error {
	// Set up pad-added callback for audio decodebin
	p.audioDecode.Connect("pad-added", func(element *gst.Element, pad *gst.Pad) {
		padName := pad.GetName()
		p.logger.Infof("Audio decoder new pad added: %s", padName)

		caps := pad.GetCurrentCaps()
		if caps != nil {
			structure := caps.GetStructureAt(0)
			if structure != nil {
				mediaType := structure.Name()
				p.logger.Infof("Audio decoder pad %s has media type: %s", padName, mediaType)

				if strings.HasPrefix(mediaType, "audio/") {
					sinkPad := p.audioConv.GetStaticPad("sink")
					if sinkPad != nil && !sinkPad.IsLinked() {
						if linkReturn := pad.Link(sinkPad); linkReturn != gst.PadLinkOK {
							p.logger.Errorf("Failed to link audio decode pad %s: %v", padName, linkReturn)
						} else {
							p.logger.Infof("Successfully linked audio decoder pad %s", padName)
						}
						sinkPad.Unref()
					} else {
						p.logger.Warnf("Audio converter sink pad not available or already linked for pad %s", padName)
					}
				}
			}
			caps.Unref()
		} else {
			p.logger.Warnf("Could not get capabilities for audio decoder pad %s", padName)
		}
	})

	// Link audio queue to decode
	if err := p.audioQueue.Link(p.audioDecode); err != nil {
		return fmt.Errorf("failed to link audio queue to decode: %w", err)
	}

	// Link audio processing elements
	elements := []*gst.Element{p.audioConv, p.audioResamp, p.audioEnc, p.audioEncQueue}

	for i := 0; i < len(elements)-1; i++ {
		if err := elements[i].Link(elements[i+1]); err != nil {
			return fmt.Errorf("failed to link audio elements %s to %s: %w",
				elements[i].GetName(), elements[i+1].GetName(), err)
		}
	}

	// Link audio encoder queue to muxer
	if err := p.audioEncQueue.Link(p.mux); err != nil {
		return fmt.Errorf("failed to link audio encoder queue to muxer: %w", err)
	}

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
