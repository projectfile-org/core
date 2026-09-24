// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package genlog is core's OTEL-aligned log surface: DEBUG/INFO/WARN/ERROR levels, Debug shown only under Verbose, Success always shown.
package genlog

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

// Severity numbers are the OTEL log severity numbers for genlog's levels (DEBUG=5, INFO=9, WARN=13, ERROR=17).
const (
	SeverityDebug = 5
	SeverityInfo  = 9
	SeverityWarn  = 13
	SeverityError = 17
)

var (
	mu     sync.Mutex
	logger *log.Logger
	output io.Writer = os.Stderr
	// Quiet suppresses Decision/Section/Plain/Info/Debug output; warnings, errors and Success are NEVER suppressed.
	Quiet bool
	// Verbose writes Info and Debug straight through. Off by default; enabled by --verbose or PF_CLI_VERBOSE=1.
	Verbose bool
	// debugLines buffers Debug output while neither Verbose nor Quiet applies; FlushDebug dumps it only under Verbose.
	debugLines []string
)

// maxBufferedDebug caps the failure-context ring; beyond it the oldest line drops.
const maxBufferedDebug = 500

// SetQuiet drives Quiet across the module boundary (a value alias would copy the var).
func SetQuiet(b bool) { Quiet = b }

// SetVerbose drives Verbose across the module boundary and opens the logger level for Debug.
func SetVerbose(b bool) {
	Verbose = b
	if b {
		L().SetLevel(log.DebugLevel)
	} else {
		L().SetLevel(log.InfoLevel)
	}
}

// Field-column widths for Decision rows. Set so the most common decision
// types align cleanly without wrapping in a 100-col terminal.
const (
	colField     = 22
	colValue     = 40
	colSeparator = " "
)

// Styles for the decision trace. Subdued by design — generators emit one
// line per assembled field and we want them to read as a digest, not a
// celebration.
var (
	styleField    = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
	styleSource   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	styleOverride = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	styleSuccess  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
)

// levelStyles returns the default text styles with 5-column level names
// (DEBUG, "INFO ", "WARN ", ERROR, FATAL) so every line aligns without truncation.
func levelStyles() *log.Styles {
	st := log.DefaultStyles()
	st.Levels[log.DebugLevel] = lipgloss.NewStyle().SetString("DEBUG").Bold(true).MaxWidth(5).Foreground(lipgloss.Color("63"))
	st.Levels[log.InfoLevel] = lipgloss.NewStyle().SetString("INFO ").Bold(true).MaxWidth(5).Foreground(lipgloss.Color("86"))
	st.Levels[log.WarnLevel] = lipgloss.NewStyle().SetString("WARN ").Bold(true).MaxWidth(5).Foreground(lipgloss.Color("192"))
	st.Levels[log.ErrorLevel] = lipgloss.NewStyle().SetString("ERROR").Bold(true).MaxWidth(5).Foreground(lipgloss.Color("204"))
	st.Levels[log.FatalLevel] = lipgloss.NewStyle().SetString("FATAL").Bold(true).MaxWidth(5).Foreground(lipgloss.Color("134"))
	return st
}

// L returns the lazy-initialised logger. Direct use is supported for
// callers that need the full log.Logger surface (With, SetLevel, etc.);
// most callers should prefer the package-level helpers below.
func L() *log.Logger {
	mu.Lock()
	defer mu.Unlock()
	if logger == nil {
		level := log.InfoLevel
		if Verbose {
			level = log.DebugLevel
		}
		logger = log.NewWithOptions(output, log.Options{
			ReportTimestamp: false,
			Level:           level,
		})
		logger.SetStyles(levelStyles())
	}
	return logger
}

// SetOutput redirects logger output. Used by tests; the default is stderr.
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	output = w
	logger = nil // re-init on next L() so the new writer takes effect
}

// Debug buffers an operational trace line (OTEL severity 5); shown only under Verbose, never under Quiet.
func Debug(msg string, kv ...any) {
	if Quiet {
		return
	}
	if Verbose {
		L().Debug(msg, kv...)
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if len(debugLines) >= maxBufferedDebug {
		copy(debugLines, debugLines[1:])
		debugLines = debugLines[:len(debugLines)-1]
	}
	debugLines = append(debugLines, debugLine(msg, kv...))
}

// FlushDebug dumps buffered Debug lines when Verbose is set and Quiet is not; it always clears the buffer.
func FlushDebug() {
	mu.Lock()
	lines := debugLines
	debugLines = nil
	verbose := Verbose
	quiet := Quiet
	mu.Unlock()
	if len(lines) == 0 || !verbose || quiet {
		return
	}
	out := currentOutput()
	for _, l := range lines {
		fmt.Fprint(out, l)
	}
}

// debugLine renders one Debug row in the logger's own format for byte-identical verbose/buffered output.
func debugLine(msg string, kv ...any) string {
	var sb strings.Builder
	l := log.NewWithOptions(&sb, log.Options{ReportTimestamp: false, Level: log.DebugLevel})
	l.SetStyles(levelStyles())
	l.Debug(msg, kv...)
	return sb.String()
}

// Info emits an operational log line, hidden unless Verbose is true and Quiet is false.
func Info(msg string, kv ...any) {
	if !Verbose || Quiet {
		return
	}
	L().Info(msg, kv...)
}

// Warn emits a warning; NEVER suppressed by Quiet and NEVER triggers a debug flush.
func Warn(msg string, kv ...any) {
	L().Warn(msg, kv...)
}

// Error emits the error line; buffered Debug context is NOT flushed (debug output is Verbose-only).
func Error(msg string, kv ...any) {
	L().Error(msg, kv...)
}

// Success prints a green checkmark line; shown ALWAYS, even under Quiet.
func Success(s string) {
	if !successStyled() {
		fmt.Fprintln(currentOutput(), "✓ "+s)
		return
	}
	fmt.Fprintln(currentOutput(), styleSuccess.Render("✓ "+s))
}

func successStyled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := currentOutput().(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// Section prints a digest header; verbose-only since the Decision rows it frames are debug by default.
func Section(title string) {
	if Quiet || !Verbose {
		return
	}
	fmt.Fprintf(currentOutput(), "\n%s\n", lipgloss.NewStyle().Bold(true).Render(title))
}

// Decision logs one column-aligned per-field assembly row; suppressed when Quiet is true.
func Decision(field, value, source, override string) {
	if Quiet {
		return
	}
	decisionRow(field, value, source, override)
}

// DebugRow renders one Decision-shaped row at debug severity: immediate under Verbose, buffered otherwise, dropped under Quiet.
func DebugRow(field, value, source, override string) {
	if Quiet {
		return
	}
	if Verbose {
		fmt.Fprintln(currentOutput(), decisionRowString(field, value, source, override))
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if len(debugLines) >= maxBufferedDebug {
		copy(debugLines, debugLines[1:])
		debugLines = debugLines[:len(debugLines)-1]
	}
	debugLines = append(debugLines, decisionRowString(field, value, source, override)+"\n")
}

func decisionRow(field, value, source, override string) {
	fmt.Fprintln(currentOutput(), decisionRowString(field, value, source, override))
}

func decisionRowString(field, value, source, override string) string {
	fieldCol := styleField.Render(padRight(field, colField))
	valueCol := padRight(value, colValue)
	sourceCol := styleSource.Render(source)
	row := fieldCol + colSeparator + valueCol + colSeparator + sourceCol
	if override != "" {
		row += "  " + styleOverride.Render("override: "+override)
	}
	return row
}

// Plain prints a single line to the logger destination without any
// styling. Used for status lines that should be visible alongside
// Decision rows but aren't themselves decisions ("LICENSE (created)").
func Plain(s string) {
	if Quiet {
		return
	}
	fmt.Fprintln(currentOutput(), s)
}

// currentOutput resolves the current writer; needed because Fprintln
// bypasses the *log.Logger writer wrapping. Kept consistent with L() so
// SetOutput takes effect for both paths.
func currentOutput() io.Writer {
	mu.Lock()
	defer mu.Unlock()
	if output == nil {
		return os.Stderr
	}
	return output
}

// padRight pads s with spaces on the right to width w. If s is already
// longer than w, it is returned unchanged — truncating a value would
// hide the very thing the decision trace exists to surface.
func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + spaces(w-len(s))
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}
