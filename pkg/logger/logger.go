package logger

import (
	"os"

	"github.com/sirupsen/logrus"
)

// Logger wraps logrus.Logger with additional functionality
type Logger struct {
	*logrus.Logger
}

// New creates a new logger instance
func New() *Logger {
	log := logrus.New()
	
	// Set output to stdout
	log.SetOutput(os.Stdout)
	
	// Set log level
	log.SetLevel(logrus.InfoLevel)
	
	// Set formatter
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})
	
	return &Logger{Logger: log}
}

// NewWithLevel creates a new logger with specified level
func NewWithLevel(level logrus.Level) *Logger {
	log := New()
	log.SetLevel(level)
	return log
}

// WithField creates an entry with a single field
func (l *Logger) WithField(key string, value interface{}) *logrus.Entry {
	return l.Logger.WithField(key, value)
}

// WithFields creates an entry with multiple fields
func (l *Logger) WithFields(fields logrus.Fields) *logrus.Entry {
	return l.Logger.WithFields(fields)
}

// WithComponent creates an entry with component field
func (l *Logger) WithComponent(component string) *logrus.Entry {
	return l.WithField("component", component)
}
