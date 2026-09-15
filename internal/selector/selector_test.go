// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package selector

import (
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunRejectsEmptyItems(t *testing.T) {
	_, err := Run(Choices[string]{Title: "pick", Label: func(s string) string { return s }})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no items")
}

func TestRunRequiresLabel(t *testing.T) {
	_, err := Run(Choices[string]{Title: "pick", Items: []string{"a"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Label is required")
}

func TestFillRejectsEmptyFields(t *testing.T) {
	err := Fill("title", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no fields")
}

func TestMultiInputNavigation(t *testing.T) {
	inputs := []textinput.Model{textinput.New(), textinput.New(), textinput.New()}
	m := NewMultiInput(inputs, []int{0, 2})
	assert.Equal(t, 0, m.Focused())
	m.Next()
	assert.Equal(t, 2, m.Focused())
	m.Next()
	assert.Equal(t, 0, m.Focused(), "Next must wrap at the end")
	m.Prev()
	assert.Equal(t, 2, m.Focused(), "Prev must wrap at the start")
	m.JumpTo(1)
	assert.Equal(t, 2, m.Focused(), "JumpTo must ignore non-focusable indices")
	m.JumpTo(0)
	assert.Equal(t, 0, m.Focused())
}

func TestMultiInputInertWhenNothingFocusable(t *testing.T) {
	inputs := []textinput.Model{textinput.New()}
	m := NewMultiInput(inputs, []int{})
	assert.Equal(t, -1, m.Focused())
	assert.NotPanics(t, func() { m.Next(); m.Prev(); m.JumpTo(0); _ = m.Update(nil) })
}
