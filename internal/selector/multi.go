// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package selector

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// MultiInput manages focus across a fixed set of textinput.Models with the
// Blur(old) + Focus(new) discipline that bubbles/textinput requires — its
// Update method short-circuits on KeyMsg when the input's `focus` flag is
// false, so moving a focus index without re-focusing drops keystrokes.
// Multi factors out that discipline so callers describe their layout (slice
// of inputs + which indices are focusable) and let the helper own
// navigation and message routing.
//
// Callers still own View rendering — Multi exposes the inputs and the
// currently-focused index but does not impose a layout.
type MultiInput struct {
	inputs    []textinput.Model
	focusable []int // indices into inputs that can receive focus
	cursor    int   // position inside focusable, NOT a direct input index
}

// NewMultiInput wraps inputs and initialises focus on the first focusable
// entry. Pass focusable=nil to allow every input to receive focus
// (equivalent to focusable=[0..len(inputs)-1]). An empty focusable slice is
// legal: the resulting Multi is inert (no input gets focused, Update is a
// no-op) — useful when every field has already been filled from another
// source and the prompt is only being shown for confirmation.
func NewMultiInput(inputs []textinput.Model, focusable []int) *MultiInput {
	if focusable == nil {
		focusable = make([]int, len(inputs))
		for i := range inputs {
			focusable[i] = i
		}
	}
	m := &MultiInput{inputs: inputs, focusable: focusable}
	if len(focusable) > 0 {
		m.focusInput(focusable[0])
	}
	return m
}

// Input returns a pointer to inputs[i] for direct access in View rendering.
// Callers must not Focus/Blur it directly — that would desync MultiInput's
// invariant. Use JumpTo / Next / Prev instead.
func (m *MultiInput) Input(i int) *textinput.Model { return &m.inputs[i] }

// Inputs exposes the backing slice for iteration in View rendering. Same
// caveat about direct Focus/Blur applies.
func (m *MultiInput) Inputs() []textinput.Model { return m.inputs }

// Focused returns the index in inputs of the currently-focused entry, or
// -1 when nothing is focusable.
func (m *MultiInput) Focused() int {
	if len(m.focusable) == 0 {
		return -1
	}
	return m.focusable[m.cursor]
}

// Next cycles focus forward through focusable, wrapping at the end.
func (m *MultiInput) Next() {
	if len(m.focusable) == 0 {
		return
	}
	m.cursor = (m.cursor + 1) % len(m.focusable)
	m.focusInput(m.focusable[m.cursor])
}

// Prev cycles focus backward through focusable, wrapping at the start.
func (m *MultiInput) Prev() {
	if len(m.focusable) == 0 {
		return
	}
	m.cursor = (m.cursor - 1 + len(m.focusable)) % len(m.focusable)
	m.focusInput(m.focusable[m.cursor])
}

// JumpTo focuses inputs[i] iff i is in focusable. No-op otherwise — silently
// refusing to focus a non-focusable index lets callers point at any field
// without first checking the focusable set.
func (m *MultiInput) JumpTo(i int) {
	for pos, fi := range m.focusable {
		if fi == i {
			m.cursor = pos
			m.focusInput(i)
			return
		}
	}
}

// Update routes msg to the currently-focused input and returns its tea.Cmd.
// No-op when nothing is focused (inert MultiInput).
func (m *MultiInput) Update(msg tea.Msg) tea.Cmd {
	idx := m.Focused()
	if idx < 0 {
		return nil
	}
	var cmd tea.Cmd
	m.inputs[idx], cmd = m.inputs[idx].Update(msg)
	return cmd
}

// focusInput is the private Blur-all-then-Focus-one primitive that every
// navigation method funnels through. Centralising it is the whole point of
// MultiInput — every prior bug in this codebase came from a navigation
// path that updated an index without performing this dance.
func (m *MultiInput) focusInput(idx int) {
	for i := range m.inputs {
		m.inputs[i].Blur()
	}
	m.inputs[idx].Focus()
}
