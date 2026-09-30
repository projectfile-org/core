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

	"charm.land/lipgloss/v2"
	"charm.land/log/v2"
	"github.com/charmbracelet/colorprofile"
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
	// results receives Success and Plain, the lines a user pipes or a script reads.
	results io.Writer = os.Stdout
	// colorMode is the --colors choice: ColorAuto defers to the environment and the TTY.
	colorMode = ColorAuto
	// profiles caches each file's resolved profile, since detection can exec `tmux info`.
	profiles = map[*os.File]colorprofile.Profile{}
	// Quiet suppresses Decision/Section/Plain/Info/Debug output; warnings, errors and Success are NEVER suppressed.
	Quiet bool
	// Verbose writes Info and Debug straight through. Off by default; enabled by --verbose or PF_CLI_VERBOSE=1.
	Verbose bool
	// debugLines buffers Debug output while neither Verbose nor Quiet applies; FlushDebug dumps it only under Verbose.
	debugLines []string
)

// Color modes SetColor accepts.
const (
	ColorAuto   = "auto"
	ColorAlways = "always"
	ColorNever  = "never"
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
		logger.SetColorProfile(profileLocked(output))
		logger.SetStyles(levelStyles())
	}
	return logger
}

// SetColor applies a --colors value (auto, always, never); an unknown value is refused and changes nothing.
func SetColor(mode string) error {
	switch mode {
	case ColorAuto, ColorAlways, ColorNever:
	default:
		return fmt.Errorf("unknown colour mode %q, expected %s, %s or %s", mode, ColorAuto, ColorAlways, ColorNever)
	}
	mu.Lock()
	defer mu.Unlock()
	colorMode = mode
	profiles = map[*os.File]colorprofile.Profile{}
	logger = nil
	return nil
}

// Profile is the one colour decision for w: --colors, then NO_COLOR, FORCE_COLOR, TERM=dumb and the TTY check.
func Profile(w io.Writer) colorprofile.Profile {
	mu.Lock()
	defer mu.Unlock()
	return profileLocked(w)
}

// profileLocked resolves w's profile from the environment and file mode, never by querying the terminal.
func profileLocked(w io.Writer) colorprofile.Profile {
	f, isFile := w.(*os.File)
	if p, ok := profiles[f]; isFile && ok {
		return p
	}
	env := os.Environ()
	switch {
	case colorMode == ColorNever:
		env = append(env, "NO_COLOR=1")
	case colorMode == ColorAlways:
		env = append(env, "NO_COLOR=", "CLICOLOR_FORCE=1")
	case os.Getenv("NO_COLOR") != "":
		env = append(env, "NO_COLOR=1")
	case os.Getenv("FORCE_COLOR") != "" && os.Getenv("FORCE_COLOR") != "0":
		env = append(env, "CLICOLOR_FORCE=1")
	}
	p := colorprofile.Detect(w, env)
	if isFile {
		profiles[f] = p
	}
	return p
}

// Styled wraps w so every lipgloss string written to it is downsampled to Profile(w).
func Styled(w io.Writer) io.Writer {
	return &colorprofile.Writer{Forward: w, Profile: Profile(w)}
}

// SetOutput redirects the trace (stderr by default).
func SetOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	output = w
	logger = nil // re-init on next L() so the new writer takes effect
}

// SetResultOutput redirects Success and Plain (stdout by default).
func SetResultOutput(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	results = w
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
	writeLines(Styled(currentOutput()), lines)
}

// DumpDebug writes and clears the buffered Debug lines regardless of Verbose, for the unexpected-error path.
func DumpDebug(w io.Writer) int {
	mu.Lock()
	lines := debugLines
	debugLines = nil
	mu.Unlock()
	writeLines(Styled(w), lines)
	return len(lines)
}

// writeLines prints each buffered line to w.
func writeLines(w io.Writer, lines []string) {
	for _, l := range lines {
		fmt.Fprint(w, l)
	}
}

// debugLine renders one Debug row in the logger's own format for byte-identical verbose/buffered output.
func debugLine(msg string, kv ...any) string {
	var sb strings.Builder
	l := log.NewWithOptions(&sb, log.Options{ReportTimestamp: false, Level: log.DebugLevel})
	l.SetColorProfile(colorprofile.TrueColor)
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

// Success prints a green checkmark result line to the result writer; shown ALWAYS, even under Quiet.
func Success(s string) {
	fmt.Fprintln(Styled(currentResults()), styleSuccess.Render("✓ "+s))
}

// Section prints a digest header; verbose-only since the Decision rows it frames are debug by default.
func Section(title string) {
	if Quiet || !Verbose {
		return
	}
	fmt.Fprintf(Styled(currentOutput()), "\n%s\n", lipgloss.NewStyle().Bold(true).Render(title))
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
		fmt.Fprintln(Styled(currentOutput()), decisionRowString(field, value, source, override))
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
	fmt.Fprintln(Styled(currentOutput()), decisionRowString(field, value, source, override))
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

// Plain prints an unstyled result line to the result writer ("LICENSE (created)"); suppressed under Quiet.
func Plain(s string) {
	if Quiet {
		return
	}
	fmt.Fprintln(currentResults(), s)
}

// currentResults resolves the result writer under the lock.
func currentResults() io.Writer {
	mu.Lock()
	defer mu.Unlock()
	return results
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
