// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package genlog is the structured-log surface for pf-cli: every status
// line, decision trace, and warning the CLI emits goes through here. It
// wraps charmbracelet/log with three small additions:
//
//   - Decision(field, value, source, override) — the per-field decision
//     trace each generator emits. Output is column-aligned so a SECURITY.md
//     run reads like a small table the user can scan.
//   - Trace — Decision's row format for a high-volume per-lookup trace
//     (interpolation resolution); gated by Verbose instead of Quiet.
//   - Quiet — package-level flag honoured by Decision and by the
//     plain-info helpers; the root command flips it from --quiet.
//   - Verbose — package-level flag that gates operational log lines
//     (file detection, include resolution, lock acquisition, etc.) and
//     Trace rows. Off by default; enabled by --verbose or PF_CLI_VERBOSE=1.
//
// The underlying logger writes to stderr by default and is created lazily
// so the package import order does not matter. Callers can override the
// destination with SetOutput; this is what cobra command tests hook in to
// capture log output alongside stdout.
package genlog

import (
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
)

var (
	mu     sync.Mutex
	logger *log.Logger
	output io.Writer = os.Stderr

	// Quiet, when true, suppresses Decision/Section/Plain output (the
	// per-bridge assembly trace). Warnings and errors are NEVER suppressed.
	Quiet bool

	// Verbose, when true, enables operational log lines (file detection,
	// lock acquisition, include resolution, SPDX lookups, etc.) and Trace
	// rows (per-lookup decision traces too high-volume for the Decision
	// digest). When false (default) these are hidden to keep output clean.
	// Toggled by the root --verbose flag or PF_CLI_VERBOSE=1 environment
	// variable.
	Verbose bool
)

// SetQuiet / SetVerbose let an out-of-package consumer (the pf-bridge root)
// drive the toggles across the module boundary — a value alias would copy the
// var, so the façade crosses via these setters.
func SetQuiet(b bool)   { Quiet = b }
func SetVerbose(b bool) { Verbose = b }

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
	styleField    = lipgloss.NewStyle().Foreground(lipgloss.Color("4")) // blue
	styleSource   = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // dim
	styleOverride = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
)

// L returns the lazy-initialised logger. Direct use is supported for
// callers that need the full log.Logger surface (With, SetLevel, etc.);
// most callers should prefer the package-level helpers below.
func L() *log.Logger {
	mu.Lock()
	defer mu.Unlock()
	if logger == nil {
		logger = log.NewWithOptions(output, log.Options{
			ReportTimestamp: false,
			Level:           log.InfoLevel,
		})
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

// Info emits an operational log line (file detection, include resolution,
// lock acquisition, etc.). Hidden unless Verbose is true. Use Decision /
// Section / Plain for user-visible output that is gated by --quiet instead.
func Info(msg string, kv ...any) {
	if !Verbose {
		return
	}
	L().Info(msg, kv...)
}

// Warn emits a warning. NEVER suppressed by Quiet — warnings represent
// state the user must see (person merge conflicts, fallback applied
// because configuration is missing, etc.).
func Warn(msg string, kv ...any) {
	L().Warn(msg, kv...)
}

// Error emits an error log line. Returning the error to the caller is
// still the right thing — this is for surfacing intermediate failures
// during a multi-step generator/sync.
func Error(msg string, kv ...any) {
	L().Error(msg, kv...)
}

// Section prints a one-line header that frames a sequence of Decision()
// rows. The intent is to make multi-generator runs (one section per
// generated file) visually navigable in the terminal.
func Section(title string) {
	if Quiet {
		return
	}
	fmt.Fprintf(currentOutput(), "\n%s\n", lipgloss.NewStyle().Bold(true).Render(title))
}

// Decision logs a single per-field assembly decision: which value was
// chosen, where it came from, and the override path (typically the
// extension namespace key the user can flip to change the default). One
// line per row, column-aligned. Suppressed when Quiet is true.
//
// field    — the conceptual slot (e.g. "contact", "disclosure-window")
// value    — the value chosen, or "(unset, omitted)" / "(default ...)"
// source   — where the value came from (e.g. "people[0].email",
//
//	"default", "[org.projectfile.security].contact")
//
// override — the user-facing knob the reader can flip (typically the
//
//	extension key); pass "" for unconditional rows.
func Decision(field, value, source, override string) {
	if Quiet {
		return
	}
	decisionRow(field, value, source, override)
}

// Trace is Decision's row format gated by Verbose instead of Quiet. Use it
// for a decision trace that fires once per lookup rather than once per
// generated field (interpolation resolution) — high-volume enough to drown
// the digest Decision exists to give, but exactly what --verbose is for.
func Trace(field, value, source, override string) {
	if !Verbose {
		return
	}
	decisionRow(field, value, source, override)
}

func decisionRow(field, value, source, override string) {
	fieldCol := styleField.Render(padRight(field, colField))
	valueCol := padRight(value, colValue)
	sourceCol := styleSource.Render(source)
	row := fieldCol + colSeparator + valueCol + colSeparator + sourceCol
	if override != "" {
		row += "  " + styleOverride.Render("override: "+override)
	}
	fmt.Fprintln(currentOutput(), row)
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
