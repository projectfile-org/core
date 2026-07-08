// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package selector is a generic bubbletea picker over a slice of choices.
// Lifted out of internal/scaffold/prompt.go so it can be reused by the
// cmd/bridge no-args picker path without dragging scaffold's source.Partial
// into every caller.
//
// Usage:
//
//	chosen, err := selector.Run(selector.Choices[core.Bridge]{
//	    Title:  "Pick a file to bridge",
//	    Items:  bridges,
//	    Label:  func(b core.Bridge) string { return b.Filename() },
//	    Detail: func(b core.Bridge) string { return string(b.Policy().Marker) },
//	})
//
// Cancellation (q, esc, ctrl+c) returns (zero, ErrCancelled) so callers can
// distinguish "user backed out" from a real I/O error.
package selector

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrCancelled is returned by Run when the user backs out (esc/ctrl+c/q).
// Callers should treat this as "user said no" rather than a failure to wrap.
var ErrCancelled = errors.New("selector: cancelled by user")

// Choices configures the picker. Label is required; Detail is optional and
// shown in a dim style next to each item. Title appears as a one-line header.
type Choices[T any] struct {
	Title  string
	Items  []T
	Label  func(T) string
	Detail func(T) string
}

var (
	styleHighlight = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))
	styleDim       = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

// Run shows the picker and returns the chosen item. The current focus starts
// at index 0; arrows / j / k move, enter / space selects.
func Run[T any](c Choices[T]) (T, error) {
	var zero T
	if len(c.Items) == 0 {
		return zero, errors.New("selector: no items to choose from")
	}
	if c.Label == nil {
		return zero, errors.New("selector: Label is required")
	}
	m := model[T]{choices: c}
	prog := tea.NewProgram(m)
	res, err := prog.Run()
	if err != nil {
		return zero, fmt.Errorf("selector: %w", err)
	}
	final := res.(model[T])
	if final.cancelled {
		return zero, ErrCancelled
	}
	return final.choices.Items[final.cursor], nil
}

type model[T any] struct {
	choices   Choices[T]
	cursor    int
	cancelled bool
	done      bool
}

func (m model[T]) Init() tea.Cmd { return nil }

func (m model[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.choices.Items)-1 {
				m.cursor++
			}
		case "enter", " ":
			m.done = true
			return m, tea.Quit
		case "q", "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model[T]) View() string {
	var b strings.Builder
	if m.choices.Title != "" {
		fmt.Fprintf(&b, "\n  %s\n\n", styleHighlight.Render(m.choices.Title))
	}
	for i, item := range m.choices.Items {
		cursor := "  "
		if m.cursor == i {
			cursor = styleHighlight.Render("> ")
		}
		label := m.choices.Label(item)
		if m.cursor == i {
			label = styleHighlight.Render(label)
		}
		row := fmt.Sprintf("  %s %s", cursor, label)
		if m.choices.Detail != nil {
			if d := m.choices.Detail(item); d != "" {
				row += "  " + styleDim.Render(d)
			}
		}
		b.WriteString(row)
		b.WriteString("\n")
	}
	b.WriteString("\n  ↑/↓ move, enter select, esc cancel\n")
	return b.String()
}
