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
	"errors"
	"fmt"
	. "github.com/onsi/gomega"
	"reflect"
	"testing"
)

const (
	filePathShort = "zap/zap_test.go"
	loggerName    = "zapTest"
	loggerKey     = "key1"
	loggerValue   = "value1"
	infoMsg       = "info1"
	infoKey       = "key2"
	infoValue     = "value2"
	infoTagUpper  = "INFO"
	infoTagLower  = "info"
	errorTagLower = "error"
	debugTagUpper = "DEBUG"
)

// fakeSyncWriter is a fake zap.SyncerWriter that lets us test if sync was called
type fakeSyncWriter struct {
	item     []byte
	isSynced bool
}

func (w *fakeSyncWriter) Write(p []byte) (int, error) {
	w.item = p
	return len(p), nil
}
func (w *fakeSyncWriter) Sync() error {
	w.isSynced = true
	return nil
}

// Tests if Dev env, ConsoleFriendlyOutputFormat emitting correct logs
func TestDevConsoleFriendlyOutput(t *testing.T) {
	testDevEnvironmentLog(t, ConsoleFriendlyOutputFormat,
		fmt.Sprintf("{\"%s\": \"%s\", \"%s\": \"%s\"}", loggerKey, loggerValue, infoKey, infoValue), false)
}

// Tests if Dev env, JSONOutputFormat emitting correct logs
func TestDevJSONOutput(t *testing.T) {
	testDevEnvironmentLog(t, JSONOutputFormat,
		fmt.Sprintf("\"%s\":\"%s\",\"%s\":\"%s\"}", loggerKey, loggerValue, infoKey, infoValue), false)
}

// Tests if Prod env, ConsoleFriendlyOutputFormat outputs correct logs
func TestProdConsoleFriendlyOutput(t *testing.T) {
	testProdEnvironmentLog(t, ConsoleFriendlyOutputFormat,
		fmt.Sprintf("{\"%s\": \"%s\", \"%s\": \"%s\"}", loggerKey, loggerValue, infoKey, infoValue))
}

// Tests if Prod env, JSONOutputFormat outputs correct logs
func TestProdJSONOutput(t *testing.T) {
	testProdEnvironmentLog(t, JSONOutputFormat,
		fmt.Sprintf("\"%s\":\"%s\",\"%s\":\"%s\"}", loggerKey, loggerValue, infoKey, infoValue))
}

// Tests if Invalid environment type is throwing error
func TestInvalidEnvironment(t *testing.T) {
	g := NewGomegaWithT(t)
	env := Environment(2)
	logger, err := NewLogger(env, ConsoleFriendlyOutputFormat)
	g.Expect(logger).Should(BeNil())
	g.Expect(err).Should(MatchError(fmt.Sprintf("%s enum value not found", reflect.TypeOf(env))))
}

// Tests if Invalid OutputFormat type is throwing error in Dev env.
func TestDevOutputInvalidOutputFormat(t *testing.T) {
	testInvalidOutputFormat(t, DevEnvironment)
}

// Tests if Invalid OutputFormat type is throwing error in Prod env.
func TestProdOutputInvalidOutputFormat(t *testing.T) {
	testInvalidOutputFormat(t, ProdEnvironment)
}

// Tests if Prod env, JSONOutputFormat outputs correctly for ERROR level
func TestProdErrorOutput(t *testing.T) {
	g := NewGomegaWithT(t)
	error := errors.New("from test")
	syncWriter := fakeSyncWriter{}
	logger, err := NewLoggerTo(ProdEnvironment, JSONOutputFormat, &syncWriter, &syncWriter)
	logger.WithName(loggerName).WithValues(loggerKey, loggerValue).Error(error, infoMsg, infoKey, infoValue)
	g.Expect(err).Should(BeNil())
	g.Expect(syncWriter.isSynced).Should(Equal(true))
	g.Expect(syncWriter.item).Should(ContainSubstring(loggerName))
	g.Expect(syncWriter.item).Should(ContainSubstring(infoMsg))
	g.Expect(syncWriter.item).Should(ContainSubstring(fmt.Sprintf("\"%s\":\"%s\",\"%s\":\"%s\"", loggerKey, loggerValue, infoKey, infoValue)))
	g.Expect(syncWriter.item).Should(ContainSubstring(errorTagLower))
	g.Expect(syncWriter.item).Should(ContainSubstring(filePathShort))
}

// Tests if Prod env, is not emitting DEBUG level
func TestProdDebugLogsDisabled(t *testing.T) {
	g := NewGomegaWithT(t)
	syncWriter := fakeSyncWriter{}
	logger, err := NewLoggerTo(ProdEnvironment, JSONOutputFormat, &syncWriter, &syncWriter)
	logger.WithName(loggerName).WithValues(loggerKey, loggerValue).V(1).Info(infoMsg, infoKey, infoValue)
	g.Expect(err).Should(BeNil())
	g.Expect(syncWriter.item).Should(BeNil())
}

// Tests if DEV env, is emitting DEBUG level
func TestDevDebugLogsEnabled(t *testing.T) {
	testDevEnvironmentLog(t, ConsoleFriendlyOutputFormat,
		fmt.Sprintf("{\"%s\": \"%s\", \"%s\": \"%s\"}", loggerKey, loggerValue, infoKey, infoValue), true)
}

func testDevEnvironmentLog(t *testing.T, outputFormat OutputFormat, valueMapFormat string, isDebugLog bool) {
	g := NewGomegaWithT(t)
	syncWriter := fakeSyncWriter{}
	loggerLevel := 0
	logger, err := NewLoggerTo(DevEnvironment, outputFormat, &syncWriter, &syncWriter)
	logger = logger.WithName(loggerName).WithValues(loggerKey, loggerValue)
	if isDebugLog {
		loggerLevel = 1
	}
	logger.V(loggerLevel).Info(infoMsg, infoKey, infoValue)
	g.Expect(err).Should(BeNil())
	g.Expect(syncWriter.isSynced).Should(Equal(true))
	g.Expect(syncWriter.item).Should(ContainSubstring(loggerName))
	g.Expect(syncWriter.item).Should(ContainSubstring(infoMsg))
	g.Expect(syncWriter.item).Should(ContainSubstring(valueMapFormat))
	g.Expect(syncWriter.item).Should(ContainSubstring(filePathShort))
	if isDebugLog {
		g.Expect(syncWriter.item).Should(ContainSubstring(debugTagUpper))
	} else {
		g.Expect(syncWriter.item).Should(ContainSubstring(infoTagUpper))
	}
}

func testProdEnvironmentLog(t *testing.T, outputFormat OutputFormat, valueMapFormat string) {
	g := NewGomegaWithT(t)
	syncWriter := fakeSyncWriter{}
	logger, err := NewLoggerTo(ProdEnvironment, outputFormat, &syncWriter, &syncWriter)
	logger.WithName(loggerName).WithValues(loggerKey, loggerValue).Info(infoMsg, infoKey, infoValue)
	g.Expect(err).Should(BeNil())
	g.Expect(syncWriter.isSynced).Should(Equal(true))
	g.Expect(syncWriter.item).Should(ContainSubstring(loggerName))
	g.Expect(syncWriter.item).Should(ContainSubstring(infoMsg))
	g.Expect(syncWriter.item).Should(ContainSubstring(valueMapFormat))
	g.Expect(syncWriter.item).Should(ContainSubstring(infoTagLower))
	g.Expect(syncWriter.item).Should(ContainSubstring(filePathShort))
}

func testInvalidOutputFormat(t *testing.T, env Environment) {
	g := NewGomegaWithT(t)
	outputFormat := OutputFormat(3)
	logger, err := NewLogger(env, outputFormat)
	g.Expect(logger).Should(BeNil())
	g.Expect(err).Should(MatchError(fmt.Sprintf("%s enum value not found", reflect.TypeOf(outputFormat))))
}
