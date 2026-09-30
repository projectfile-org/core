// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package genlog

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/colorprofile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func isolateOutput(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	SetOutput(&buf)
	SetResultOutput(&buf)
	Quiet = false
	SetVerbose(false)
	mu.Lock()
	debugLines = nil
	mu.Unlock()
	t.Cleanup(func() {
		SetResultOutput(os.Stdout)
		require.NoError(t, SetColor(ColorAuto))
		Quiet = false
		SetVerbose(false)
		mu.Lock()
		debugLines = nil
		mu.Unlock()
	})
	return &buf
}

func TestDebugBufferedUntilFlush(t *testing.T) {
	buf := isolateOutput(t)
	Debug("hidden op", "k", "v")
	assert.Empty(t, buf.String(), "debug must not print by default")
	SetVerbose(true)
	FlushDebug()
	assert.Contains(t, buf.String(), "hidden op")
	assert.Contains(t, buf.String(), "DEBUG")
	assert.NotContains(t, buf.String(), "DEBU ")
}

func TestDebugImmediateWhenVerbose(t *testing.T) {
	buf := isolateOutput(t)
	SetVerbose(true)
	Debug("loud op")
	assert.Contains(t, buf.String(), "loud op")
	assert.Contains(t, buf.String(), "DEBUG")
}

func TestErrorDoesNotFlushDebug(t *testing.T) {
	buf := isolateOutput(t)
	Debug("context op")
	Error("boom")
	out := buf.String()
	require.Contains(t, out, "boom")
	assert.NotContains(t, out, "context op", "debug context must stay hidden without --verbose")
	SetVerbose(true)
	FlushDebug()
	assert.Contains(t, buf.String(), "context op", "buffered context is still available under --verbose")
}

func TestWarnDoesNotFlushDebug(t *testing.T) {
	buf := isolateOutput(t)
	Debug("context op")
	Warn("heads up")
	assert.Contains(t, buf.String(), "heads up")
	assert.NotContains(t, buf.String(), "context op")
	SetVerbose(true)
	FlushDebug()
	assert.Contains(t, buf.String(), "context op")
}

func TestSuccessShownUnderQuiet(t *testing.T) {
	buf := isolateOutput(t)
	Quiet = true
	Success("all green")
	assert.Contains(t, buf.String(), "✓")
	assert.Contains(t, buf.String(), "all green")
	assert.NotContains(t, buf.String(), "\x1b[", "redirected output must not carry ANSI escapes")
}

func TestDebugRowKeepsTableShape(t *testing.T) {
	buf := isolateOutput(t)
	DebugRow("run_image", "n -> r", "scope.ref", "")
	SetVerbose(true)
	FlushDebug()
	out := buf.String()
	assert.Contains(t, out, "run_image")
	assert.Contains(t, out, "n -> r")
	assert.Contains(t, out, "scope.ref")
	assert.NotContains(t, out, "missing value")
}

func TestDebugRingKeepsNewest(t *testing.T) {
	buf := isolateOutput(t)
	for range maxBufferedDebug + 10 {
		Debug("op")
	}
	SetVerbose(true)
	FlushDebug()
	assert.Equal(t, maxBufferedDebug, strings.Count(buf.String(), "op"))
}

func TestQuietSuppressesDebug(t *testing.T) {
	buf := isolateOutput(t)
	Quiet = true
	Debug("quiet op")
	Info("quiet info")
	DebugRow("f", "v", "s", "")
	SetVerbose(true)
	Debug("loud quiet op")
	FlushDebug()
	assert.Empty(t, buf.String(), "quiet must suppress debug, info and flush even under verbose")
}

func TestFlushGatedOnVerbose(t *testing.T) {
	buf := isolateOutput(t)
	Debug("gated op")
	FlushDebug()
	assert.Empty(t, buf.String(), "flush must stay silent without --verbose")
}

func TestLevelNamesAligned(t *testing.T) {
	buf := isolateOutput(t)
	SetVerbose(true)
	Debug("d")
	Info("i")
	Warn("w")
	Error("e")
	out := buf.String()
	assert.Contains(t, out, "DEBUG")
	assert.Contains(t, out, "INFO ")
	assert.Contains(t, out, "WARN ")
	assert.Contains(t, out, "ERROR")
	assert.NotContains(t, out, "DEBU ")
	assert.NotContains(t, out, "ERRO ")
}

func TestProfileOfNonFileIsNoTTY(t *testing.T) {
	isolateOutput(t)
	assert.Equal(t, colorprofile.NoTTY, Profile(&bytes.Buffer{}))
}

func TestResultsLeaveTheTrace(t *testing.T) {
	isolateOutput(t)
	var trace, results bytes.Buffer
	SetOutput(&trace)
	SetResultOutput(&results)
	Success("done")
	Plain("note")
	Warn("careful")
	assert.Equal(t, "✓ done\nnote\n", results.String())
	assert.Contains(t, trace.String(), "careful")
	assert.NotContains(t, trace.String(), "done")
}

func TestColorModes(t *testing.T) {
	isolateOutput(t)
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	require.NoError(t, SetColor(ColorAlways))
	assert.GreaterOrEqual(t, Profile(&bytes.Buffer{}), colorprofile.ANSI, "always colours a pipe")
	t.Setenv("NO_COLOR", "1")
	assert.GreaterOrEqual(t, Profile(&bytes.Buffer{}), colorprofile.ANSI, "the flag outranks NO_COLOR")
	require.NoError(t, SetColor(ColorAuto))
	assert.Equal(t, colorprofile.NoTTY, Profile(&bytes.Buffer{}))
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")
	assert.GreaterOrEqual(t, Profile(&bytes.Buffer{}), colorprofile.ANSI, "FORCE_COLOR colours a pipe")
	t.Setenv("NO_COLOR", "yes")
	assert.Equal(t, colorprofile.NoTTY, Profile(&bytes.Buffer{}), "any NO_COLOR value outranks FORCE_COLOR")
	require.Error(t, SetColor("sometimes"))
}

func TestForcedColourReachesResults(t *testing.T) {
	buf := isolateOutput(t)
	require.NoError(t, SetColor(ColorAlways))
	Success("green")
	assert.Contains(t, buf.String(), "\x1b[")
}

func TestDumpDebugIgnoresVerbose(t *testing.T) {
	isolateOutput(t)
	Debug("crash context", "k", "v")
	var dump bytes.Buffer
	assert.Equal(t, 1, DumpDebug(&dump))
	assert.Contains(t, dump.String(), "crash context")
	assert.NotContains(t, dump.String(), "\x1b[", "a dump file carries no escapes")
	assert.Zero(t, DumpDebug(&dump), "the dump clears the buffer")
}
