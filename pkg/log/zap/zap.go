/*
Copyright 2018 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package zap

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Environment represents which env the logger is running on i.e. Dev or Prod
type Environment int8

// OutputFormat represents the format of the log that emits to outputWriter or errorOutputWriter
type OutputFormat int8

// Constants that has enum values for Environment.
// In prod stacktrace is enabled for levels >= ErrorLevel and has sampling so that system resources are less consumed
// In Dev stacktrace is enabled for levels >= WarnLevel and dont have sampling.
const (
	ProdEnvironment = Environment(iota) // Prodcution env where only INFO level logs are displayed but not DEBUG level
	DevEnvironment                      // Development env where both INFO and DEBUG level logs are displayed
)

// Constants that has enum values for OutputFormat.
const (
	ConsoleFriendlyOutputFormat = OutputFormat(iota) // More console fiendly
	JSONOutputFormat                                 // In JSON format
)

// NewLogger returns logger as logr.Logger interface implementation and will emit non-error logs to
// stdout and error logs to stdout
func NewLogger(env Environment, outputFormat OutputFormat) (logr.Logger, error) {
	return NewLoggerTo(env, outputFormat, os.Stdout, os.Stderr)
}

// NewLoggerTo returns logger as logr.Logger interface implementation and emits non-error logs to outputWriter
// and error logs to errorOutputWriter
func NewLoggerTo(env Environment, outputFormat OutputFormat, outputWriter io.Writer,
	errorOutputWriter io.Writer) (logr.Logger, error) {
	var enc zapcore.Encoder
	var lvl zap.AtomicLevel
	var opts []zap.Option
	var encCfg zapcore.EncoderConfig

	if env == DevEnvironment {
		encCfg = zap.NewDevelopmentEncoderConfig()
		lvl = zap.NewAtomicLevelAt(zap.DebugLevel)
		opts = append(opts, zap.Development(), zap.AddStacktrace(zap.ErrorLevel))
	} else if env == ProdEnvironment {
		encCfg = zap.NewProductionEncoderConfig()
		lvl = zap.NewAtomicLevelAt(zap.InfoLevel)
		opts = append(opts, zap.AddStacktrace(zap.WarnLevel),
			zap.WrapCore(func(core zapcore.Core) zapcore.Core {
				return zapcore.NewSampler(core, time.Second, 100, 100)
			}))
	} else {
		return nil, fmt.Errorf("%s enum value not found", reflect.TypeOf(env))
	}

	if outputFormat == ConsoleFriendlyOutputFormat {
		encCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		enc = zapcore.NewConsoleEncoder(encCfg)
	} else if outputFormat == JSONOutputFormat {
		encCfg.EncodeTime = zapcore.EpochNanosTimeEncoder
		enc = zapcore.NewJSONEncoder(encCfg)
	} else {
		return nil, fmt.Errorf("%s enum value not found", reflect.TypeOf(outputFormat))
	}

	opts = append(opts, zap.AddCallerSkip(1), zap.ErrorOutput(zapcore.AddSync(outputWriter)), zap.AddCaller())
	log := zap.New(zapcore.NewCore(enc, zapcore.AddSync(errorOutputWriter), lvl))
	log = log.WithOptions(opts...)

	defer log.Sync()

	return zapr.NewLogger(log), nil
}
