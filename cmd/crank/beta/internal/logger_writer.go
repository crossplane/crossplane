package internal

import (
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/logging"
)

// LoggerWriter is an io.Writer implementation that writes to a logger.
type LoggerWriter struct {
	logger logging.Logger
	level  int
}

// NewLoggerWriter creates a new LoggerWriter that sends output to the given logger.
func NewLoggerWriter(logger logging.Logger) *LoggerWriter {
	return &LoggerWriter{
		logger: logger,
		level:  0, // Info level by default
	}
}

// Write implements io.Writer.Write by sending the data to the logger.
func (w *LoggerWriter) Write(p []byte) (n int, err error) {
	// Convert to string and trim trailing newlines
	message := strings.TrimSuffix(string(p), "\n")
	if message != "" {
		w.logger.Debug(message)
	}
	return len(p), nil
}

// WithLevel allows setting a specific logging level.
func (w *LoggerWriter) WithLevel(level int) *LoggerWriter {
	w.level = level
	return w
}
