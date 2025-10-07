package pipeline

import (
	"fmt"
	"time"
)

// PipelineError represents a pipeline-specific error
type PipelineError struct {
	Type      ErrorType
	Message   string
	Timestamp time.Time
	Source    string
	Debug     string
}

// ErrorType represents the type of pipeline error
type ErrorType int

const (
	ErrorTypeUnknown ErrorType = iota
	ErrorTypeInput
	ErrorTypeDecoding
	ErrorTypeEncoding
	ErrorTypeOutput
	ErrorTypeNetwork
	ErrorTypeResource
	ErrorTypeConfiguration
)

// Error implements the error interface
func (e *PipelineError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Type.String(), e.Source, e.Message)
}

// String returns a string representation of the error type
func (et ErrorType) String() string {
	switch et {
	case ErrorTypeInput:
		return "INPUT"
	case ErrorTypeDecoding:
		return "DECODING"
	case ErrorTypeEncoding:
		return "ENCODING"
	case ErrorTypeOutput:
		return "OUTPUT"
	case ErrorTypeNetwork:
		return "NETWORK"
	case ErrorTypeResource:
		return "RESOURCE"
	case ErrorTypeConfiguration:
		return "CONFIGURATION"
	default:
		return "UNKNOWN"
	}
}

// NewPipelineError creates a new pipeline error
func NewPipelineError(errorType ErrorType, source, message, debug string) *PipelineError {
	return &PipelineError{
		Type:      errorType,
		Message:   message,
		Timestamp: time.Now(),
		Source:    source,
		Debug:     debug,
	}
}

// ErrorHandler handles pipeline errors with retry logic
type ErrorHandler struct {
	maxRetries    int
	retryDelay    time.Duration
	retryCount    map[ErrorType]int
	lastError     *PipelineError
	errorCallback func(*PipelineError)
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(maxRetries int, retryDelay time.Duration) *ErrorHandler {
	return &ErrorHandler{
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		retryCount: make(map[ErrorType]int),
	}
}

// SetErrorCallback sets a callback function for error notifications
func (eh *ErrorHandler) SetErrorCallback(callback func(*PipelineError)) {
	eh.errorCallback = callback
}

// HandleError processes a pipeline error and determines if retry is needed
func (eh *ErrorHandler) HandleError(err *PipelineError) bool {
	eh.lastError = err
	
	// Call error callback if set
	if eh.errorCallback != nil {
		eh.errorCallback(err)
	}

	// Check if we should retry based on error type
	if !eh.shouldRetry(err.Type) {
		return false
	}

	// Increment retry count for this error type
	eh.retryCount[err.Type]++

	// Check if we've exceeded max retries
	if eh.retryCount[err.Type] > eh.maxRetries {
		return false
	}

	// Wait before retry
	time.Sleep(eh.retryDelay)
	return true
}

// shouldRetry determines if an error type should be retried
func (eh *ErrorHandler) shouldRetry(errorType ErrorType) bool {
	switch errorType {
	case ErrorTypeNetwork, ErrorTypeInput:
		return true
	case ErrorTypeResource:
		return true
	case ErrorTypeConfiguration:
		return false
	default:
		return false
	}
}

// Reset resets the retry counters
func (eh *ErrorHandler) Reset() {
	eh.retryCount = make(map[ErrorType]int)
	eh.lastError = nil
}

// GetLastError returns the last error encountered
func (eh *ErrorHandler) GetLastError() *PipelineError {
	return eh.lastError
}

// GetRetryCount returns the retry count for a specific error type
func (eh *ErrorHandler) GetRetryCount(errorType ErrorType) int {
	return eh.retryCount[errorType]
}

// HealthChecker monitors pipeline health
type HealthChecker struct {
	pipeline         *Pipeline
	checkInterval    time.Duration
	timeoutThreshold time.Duration
	lastActivity     time.Time
	isHealthy        bool
	healthCallback   func(bool)
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(pipeline *Pipeline, checkInterval, timeoutThreshold time.Duration) *HealthChecker {
	return &HealthChecker{
		pipeline:         pipeline,
		checkInterval:    checkInterval,
		timeoutThreshold: timeoutThreshold,
		lastActivity:     time.Now(),
		isHealthy:        true,
	}
}

// SetHealthCallback sets a callback for health status changes
func (hc *HealthChecker) SetHealthCallback(callback func(bool)) {
	hc.healthCallback = callback
}

// Start starts the health monitoring
func (hc *HealthChecker) Start() {
	go hc.monitor()
}

// monitor runs the health monitoring loop
func (hc *HealthChecker) monitor() {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		healthy := hc.checkHealth()
		
		if healthy != hc.isHealthy {
			hc.isHealthy = healthy
			if hc.healthCallback != nil {
				hc.healthCallback(healthy)
			}
		}
	}
}

// checkHealth performs health checks
func (hc *HealthChecker) checkHealth() bool {
	// Check if pipeline is running
	if !hc.pipeline.IsRunning() {
		return false
	}

	// Check for timeout (no activity)
	if time.Since(hc.lastActivity) > hc.timeoutThreshold {
		return false
	}

	// Additional health checks can be added here
	return true
}

// UpdateActivity updates the last activity timestamp
func (hc *HealthChecker) UpdateActivity() {
	hc.lastActivity = time.Now()
}

// IsHealthy returns the current health status
func (hc *HealthChecker) IsHealthy() bool {
	return hc.isHealthy
}

// RecoveryManager handles pipeline recovery
type RecoveryManager struct {
	pipeline      *Pipeline
	errorHandler  *ErrorHandler
	healthChecker *HealthChecker
	autoRestart   bool
	restartDelay  time.Duration
}

// NewRecoveryManager creates a new recovery manager
func NewRecoveryManager(pipeline *Pipeline, autoRestart bool, restartDelay time.Duration) *RecoveryManager {
	return &RecoveryManager{
		pipeline:     pipeline,
		autoRestart:  autoRestart,
		restartDelay: restartDelay,
		errorHandler: NewErrorHandler(3, 5*time.Second),
		healthChecker: NewHealthChecker(pipeline, 10*time.Second, 30*time.Second),
	}
}

// Start starts the recovery manager
func (rm *RecoveryManager) Start() {
	// Set up error callback
	rm.errorHandler.SetErrorCallback(func(err *PipelineError) {
		rm.pipeline.logger.Errorf("Pipeline error: %v", err)
		
		if rm.autoRestart && rm.errorHandler.shouldRetry(err.Type) {
			go rm.attemptRestart()
		}
	})

	// Set up health callback
	rm.healthChecker.SetHealthCallback(func(healthy bool) {
		if !healthy && rm.autoRestart {
			rm.pipeline.logger.Warn("Pipeline unhealthy, attempting restart")
			go rm.attemptRestart()
		}
	})

	// Start health monitoring
	rm.healthChecker.Start()
}

// attemptRestart attempts to restart the pipeline
func (rm *RecoveryManager) attemptRestart() {
	rm.pipeline.logger.Info("Attempting pipeline restart...")
	
	// Stop current pipeline
	if err := rm.pipeline.Stop(); err != nil {
		rm.pipeline.logger.Errorf("Error stopping pipeline: %v", err)
	}

	// Wait before restart
	time.Sleep(rm.restartDelay)

	// Restart pipeline
	if err := rm.pipeline.Start(nil); err != nil {
		rm.pipeline.logger.Errorf("Error restarting pipeline: %v", err)
	} else {
		rm.pipeline.logger.Info("Pipeline restarted successfully")
		rm.errorHandler.Reset()
	}
}
