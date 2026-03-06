// Copyright 2024 Upbound Inc.
// All rights reserved

package logging

import (
	"flag"
	"os"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"
)

// SetFilteredKlogLogger sets log as the logger backend of klog, but filtering
// aggressively to avoid noise.
func SetFilteredKlogLogger(log logr.Logger) {
	// initialize klog at verbosity level 3, dropping everything higher.
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	klog.InitFlags(fs)
	fs.Parse([]string{"--v=3"}) //nolint:errcheck // we couldn't do anything here anyway

	klogr := logr.New(&requestThrottlingFilter{log.GetSink()})
	klog.SetLogger(klogr)
}

// requestThrottlingFilter drops everything that is not a client-go throttling
// message, compare:
// https://github.com/kubernetes/client-go/blob/8c4efe8d079e405329f314fb789a41ac6af101dc/rest/request.go#L621
type requestThrottlingFilter struct {
	logr.LogSink
}

func (l *requestThrottlingFilter) Info(level int, msg string, keysAndValues ...any) {
	if !strings.Contains(msg, "Waited for ") || !strings.Contains(msg, "  request: ") {
		return
	}

	l.LogSink.Info(l.klogToLogrLevel(level), msg, keysAndValues...)
}

func (l *requestThrottlingFilter) Enabled(level int) bool {
	return l.LogSink.Enabled(l.klogToLogrLevel(level))
}

func (l *requestThrottlingFilter) klogToLogrLevel(klogLvl int) int {
	// we want a default klog level of 3 for info, 4 for debug, corresponding to
	// logr levels of 0 and 1.
	if klogLvl >= 3 {
		return klogLvl - 3
	}

	return 0
}

func (l *requestThrottlingFilter) WithCallDepth(depth int) logr.LogSink {
	if delegate, ok := l.LogSink.(logr.CallDepthLogSink); ok {
		return &requestThrottlingFilter{LogSink: delegate.WithCallDepth(depth)}
	}

	return l
}
