package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	loop     *gst.MainLoop
	mutex    sync.RWMutex
	running  bool
}

// New creates a new pipeline instance
func New(cfg *config.Config, logger *logrus.Logger) (*Pipeline, error) {
	// Initialize GStreamer
	gst.Init(nil)

	p := &Pipeline{
		config: cfg,
		logger: logger,
		loop:   gst.NewMainLoop(gst.DefaultMainContext(), false),
	}

	if err := p.buildPipeline(); err != nil {
		return nil, fmt.Errorf("failed to build pipeline: %w", err)
	}

	return p, nil
}

// buildPipeline constructs the GStreamer pipeline
func (p *Pipeline) buildPipeline() error {
	// Create pipeline
	pipeline, err := gst.NewPipeline("video-overlay-pipeline")
	if err != nil {
		return fmt.Errorf("failed to create pipeline: %w", err)
	}
	p.pipeline = pipeline

	// Build pipeline string
	pipelineStr := p.buildPipelineString()
	p.logger.Infof("Pipeline string: %s", pipelineStr)

	// Parse and create pipeline from string
	elements, err := gst.NewPipelineFromString(pipelineStr)
	if err != nil {
		return fmt.Errorf("failed to create pipeline from string: %w", err)
	}

	// Replace the pipeline with the parsed one
	p.pipeline = elements.(*gst.Pipeline)

	// Get bus for message handling
	p.bus = p.pipeline.GetPipelineBus()

	return nil
}

// buildPipelineString constructs the GStreamer pipeline string
func (p *Pipeline) buildPipelineString() string {
	cfg := p.config

	// Create HLS input handler
	hlsInput, err := NewHLSInput(&cfg.Input)
	if err != nil {
		p.logger.Errorf("Failed to create HLS input: %v", err)
		return ""
	}

	// Create overlay manager
	overlayManager := NewOverlayManager(&cfg.Overlay)

	// Create UDP output handler
	udpOutput, err := NewUDPOutput(&cfg.Output)
	if err != nil {
		p.logger.Errorf("Failed to create UDP output: %v", err)
		return ""
	}

	// Build pipeline components
	source := hlsInput.GetPipelineString()

	// Video processing chain
	videoChain := "queue max-size-buffers=100 max-size-time=1000000000 ! " +
		"decodebin ! videoconvert ! videoscale ! " +
		"video/x-raw,format=I420"

	// Add overlay if enabled
	overlayStr := overlayManager.GetPipelineString()
	if overlayStr != "" {
		videoChain += " ! " + overlayStr
	}

	// Audio processing chain
	audioChain := "queue max-size-buffers=100 max-size-time=1000000000 ! " +
		"decodebin ! audioconvert ! audioresample ! " +
		"audio/x-raw,rate=48000,channels=2"

	// Get encoding and output pipeline
	outputPipeline := udpOutput.GetPipelineString()

	// Complete pipeline with proper branching
	return fmt.Sprintf("%s "+
		"demux.video_0 ! %s ! %s "+
		"demux.audio_0 ! %s ! %s",
		source, videoChain, outputPipeline, audioChain, outputPipeline)
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
	ret := p.pipeline.SetState(gst.StatePlaying)
	if ret == gst.StateChangeFailure {
		return fmt.Errorf("failed to set pipeline to playing state")
	}

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
	ret := p.pipeline.SetState(gst.StateNull)
	if ret == gst.StateChangeFailure {
		p.logger.Warn("Failed to set pipeline to null state")
	}

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
			msg := p.bus.TimedPop(100 * time.Millisecond)
			if msg == nil {
				continue
			}

			switch msg.Type() {
			case gst.MessageEOS:
				p.logger.Info("End of stream received")
				return
			case gst.MessageError:
				err, debug := msg.ParseError()
				p.logger.Errorf("Pipeline error: %s (debug: %s)", err.Error(), debug)
				return
			case gst.MessageWarning:
				err, debug := msg.ParseWarning()
				p.logger.Warnf("Pipeline warning: %s (debug: %s)", err.Error(), debug)
			case gst.MessageInfo:
				err, debug := msg.ParseInfo()
				p.logger.Infof("Pipeline info: %s (debug: %s)", err.Error(), debug)
			case gst.MessageStateChanged:
				if msg.Source() == p.pipeline.Element {
					oldState, newState, _ := msg.ParseStateChanged()
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
