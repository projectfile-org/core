// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package genlog

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stripANSI removes escape sequences so an assertion reads the text.
func stripANSI(s string) string {
	var b strings.Builder
	for {
		start := strings.Index(s, "\x1b[")
		if start < 0 {
			b.WriteString(s)
			return b.String()
		}
		b.WriteString(s[:start])
		end := strings.Index(s[start:], "m")
		if end < 0 {
			return b.String()
		}
		s = s[start+end+1:]
	}
}

// nonEmptyLines splits a captured stream into its non-blank lines.
func nonEmptyLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// TestSetQuietCrossesTheModuleBoundary pins why SetQuiet exists rather than a value alias.
func TestSetQuietCrossesTheModuleBoundary(t *testing.T) {
	buf := isolateOutput(t)

	Decision("field", "value", "src", "")
	require.NotEmpty(t, buf.String(), "a decision row prints when not quiet")

	SetQuiet(true)
	assert.True(t, Quiet, "the getter must see what the setter set")
	buf.Reset()

	Decision("field", "value", "src", "")
	Plain("result")
	Section("title")
	Debug("hidden")
	assert.Empty(t, buf.String(), "quiet suppresses the four informational surfaces")

	Success("done")
	Warn("careful")
	Error("broken")
	assert.Contains(t, buf.String(), "done")
	assert.Contains(t, buf.String(), "careful")
	assert.Contains(t, buf.String(), "broken")

	SetQuiet(false)
	assert.False(t, Quiet)
	buf.Reset()
	Decision("field", "value", "src", "")
	assert.NotEmpty(t, buf.String(), "un-quitting takes effect immediately")
}

// TestSectionIsVerboseOnly pins why a digest header is verbose-gated.
func TestSectionIsVerboseOnly(t *testing.T) {
	buf := isolateOutput(t)

	Section("Header")
	assert.Empty(t, buf.String(), "a section header is verbose-only")

	SetVerbose(true)
	Section("Header")
	assert.Contains(t, buf.String(), "Header")
	assert.Contains(t, buf.String(), "\n")

	SetVerbose(false)
	buf.Reset()
	Section("Header")
	assert.Empty(t, buf.String())
}

// TestDecisionRowAlignment is the scanability contract.
func TestDecisionRowAlignment(t *testing.T) {
	buf := isolateOutput(t)

	Decision("short", "v", "src", "")
	Decision("a-much-longer-field", "v", "src", "")
	Decision("short", "v", "src", "an override")

	lines := nonEmptyLines(stripANSI(buf.String()))
	require.Len(t, lines, 3)

	wantSourceCol := colField + len(colSeparator) + colValue + len(colSeparator)
	for i, l := range lines {
		assert.Equal(t, wantSourceCol, strings.Index(l, "src"),
			"row %d source column must match the declared widths", i)
	}

	assert.Contains(t, lines[2], "override: an override",
		"a non-empty override is appended as its own trailing column")
}

// TestDecisionWithoutOverrideOmitsTheColumn pins that the trailing column is conditional.
func TestDecisionWithoutOverrideOmitsTheColumn(t *testing.T) {
	buf := isolateOutput(t)
	Decision("f", "v", "src", "")

	line := nonEmptyLines(stripANSI(buf.String()))
	require.Len(t, line, 1)
	assert.NotContains(t, line[0], "override")
}

// TestDebugRowShape pins that both surfaces share one row renderer.
func TestDebugRowShape(t *testing.T) {
	decisionBuf := isolateOutput(t)
	Decision("f", "v", "src", "")
	decision := nonEmptyLines(stripANSI(decisionBuf.String()))

	debugBuf := isolateOutput(t)
	SetVerbose(true)
	DebugRow("f", "v", "src", "")
	debug := nonEmptyLines(stripANSI(debugBuf.String()))

	require.Len(t, decision, 1)
	require.Len(t, debug, 1)
	assert.Equal(t, decision[0], debug[0])
}

// TestDebugRowIsBufferedWithoutVerbose pins the ring a consumer's report reads.
func TestDebugRowIsBufferedWithoutVerbose(t *testing.T) {
	buf := isolateOutput(t)

	DebugRow("f", "v", "src", "")
	assert.Empty(t, buf.String(), "a debug row is not printed by default")

	SetVerbose(true)
	FlushDebug()
	assert.Contains(t, buf.String(), "f")
}

func TestDebugRowDroppedWhenQuiet(t *testing.T) {
	buf := isolateOutput(t)
	SetQuiet(true)
	DebugRow("f", "v", "src", "")
	SetVerbose(true)
	FlushDebug()
	assert.Empty(t, buf.String(), "quiet drops the ring rather than deferring it")
}

// TestDebugRingCapsAtItsBound is the memory contract.
func TestDebugRingCapsAtItsBound(t *testing.T) {
	isolateOutput(t)

	for i := range maxBufferedDebug + 25 {
		Debug("line", "n", i)
	}

	mu.Lock()
	held := len(debugLines)
	oldest := stripANSI(debugLines[0])
	newest := stripANSI(debugLines[len(debugLines)-1])
	mu.Unlock()

	assert.Equal(t, maxBufferedDebug, held, "the ring must not grow past its cap")
	assert.Contains(t, oldest, "n=25", "the oldest lines are the ones dropped")
	assert.Contains(t, newest, "n=524", "the newest line is still retained")
}

// TestSuccessIsNeverSuppressed pins the one line that survives --quiet.
func TestSuccessIsNeverSuppressed(t *testing.T) {
	buf := isolateOutput(t)
	SetQuiet(true)

	Success("LICENSE (created)")
	out := buf.String()
	assert.Contains(t, out, "LICENSE (created)")
	assert.Contains(t, out, "✓")
}

// TestSuccessGoesToTheResultStream pins the two-stream split.
func TestSuccessGoesToTheResultStream(t *testing.T) {
	trace := isolateOutput(t)
	Success("done")
	assert.Contains(t, trace.String(), "done")
}
