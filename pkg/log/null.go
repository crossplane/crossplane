package log

import "github.com/go-logr/logr"

// NullLogger is a logr.Logger that does nothing.
type NullLogger struct{}

var _ logr.Logger = NullLogger{}

// Info implements logr.InfoLogger
func (NullLogger) Info(_ string, _ ...interface{}) {
	// Do nothing.
}

// Enabled implements logr.InfoLogger
func (NullLogger) Enabled() bool {
	return false
}

// Error implements logr.Logger
func (NullLogger) Error(_ error, _ string, _ ...interface{}) {
	// Do nothing.
}

// V implements logr.Logger
func (log NullLogger) V(_ int) logr.InfoLogger {
	return log
}

// WithName implements logr.Logger
func (log NullLogger) WithName(_ string) logr.Logger {
	return log
}

// WithValues implements logr.Logger
func (log NullLogger) WithValues(_ ...interface{}) logr.Logger {
	return log
}
