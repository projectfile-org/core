// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package selector

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// FillField is one row in the fill-mode prompt: a labelled text input with
// an optional hint shown below. OnSubmit is called once with the trimmed
// value when the user presses enter through every input in sequence.
type FillField struct {
	Label    string
	Hint     string
	OnSubmit func(value string) error
}

// Fill shows a sequence of textinput rows and invokes each field's OnSubmit
// with the captured value after the user accepts the form (enter on the
// last field). ESC / ctrl+c returns ErrCancelled. An OnSubmit error halts
// the loop and is propagated — the rest of the form remains entered so the
// caller can re-show the same fields with partial values intact via state
// preserved outside Fill.
func Fill(title string, fields []FillField) error {
	if len(fields) == 0 {
		return errors.New("selector: no fields to fill")
	}
	m := newFillModel(title, fields)
	prog := tea.NewProgram(m)
	res, err := prog.Run()
	if err != nil {
		return fmt.Errorf("selector fill: %w", err)
	}
	final := res.(fillModel)
	if final.cancelled {
		return ErrCancelled
	}
	for i, fld := range fields {
		if fld.OnSubmit == nil {
			continue
		}
		if err := fld.OnSubmit(strings.TrimSpace(final.inputs[i].Value())); err != nil {
			return fmt.Errorf("fill %s: %w", fld.Label, err)
		}
	}
	return nil
}

type fillModel struct {
	title      string
	fields     []FillField
	inputs     []textinput.Model
	focusIndex int
	cancelled  bool
}

func newFillModel(title string, fields []FillField) fillModel {
	inputs := make([]textinput.Model, len(fields))
	for i, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.Hint
		ti.PromptStyle = styleHighlight
		ti.Width = 50
		inputs[i] = ti
	}
	inputs[0].Focus()
	return fillModel{title: title, fields: fields, inputs: inputs}
}

func (m fillModel) Init() tea.Cmd { return textinput.Blink }

func (m fillModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyEnter:
			// Move to next field; if we're on the last one, accept the form.
			if m.focusIndex == len(m.inputs)-1 {
				return m, tea.Quit
			}
			m.inputs[m.focusIndex].Blur()
			m.focusIndex++
			m.inputs[m.focusIndex].Focus()
			return m, nil
		case tea.KeyTab, tea.KeyDown:
			m.inputs[m.focusIndex].Blur()
			m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
			m.inputs[m.focusIndex].Focus()
			return m, nil
		case tea.KeyShiftTab, tea.KeyUp:
			m.inputs[m.focusIndex].Blur()
			if m.focusIndex == 0 {
				m.focusIndex = len(m.inputs) - 1
			} else {
				m.focusIndex--
			}
			m.inputs[m.focusIndex].Focus()
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
	return m, cmd
}

func (m fillModel) View() string {
	var b strings.Builder
	if m.title != "" {
		fmt.Fprintf(&b, "\n  %s\n\n", styleHighlight.Render(m.title))
	}
	for i, f := range m.fields {
		focus := "  "
		if m.focusIndex == i {
			focus = styleHighlight.Render("> ")
		}
		fmt.Fprintf(&b, "  %s %s\n", focus, styleHighlight.Render(f.Label))
		fmt.Fprintf(&b, "    %s\n", m.inputs[i].View())
		if f.Hint != "" {
			fmt.Fprintf(&b, "    %s\n", styleDim.Render(f.Hint))
		}
		b.WriteString("\n")
	}
	b.WriteString("  tab/↓ next, enter accept (last field submits), esc cancel\n")
	return b.String()
}
