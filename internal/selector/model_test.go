// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package selector

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Named keys the tests press, so the cases and the tables can never drift apart.
const (
	keyEnter    = "enter"
	keyEsc      = "esc"
	keyTab      = "tab"
	keyShiftTab = "shift+tab"
	keyUp       = "up"
	keyDown     = "down"
	keyCtrlC    = "ctrl+c"
)

// key builds the KeyPressMsg the program would deliver for a named key.
func key(name string) tea.KeyPressMsg {
	switch name {
	case keyEnter:
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case keyEsc:
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case keyTab:
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case keyShiftTab:
		return tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	case keyUp:
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case keyDown:
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case keyCtrlC:
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	default:
		r := []rune(name)[0]
		return tea.KeyPressMsg{Code: r, Text: name}
	}
}

// press sends one key to m and returns the advanced model.
func press(t *testing.T, m tea.Model, name string) tea.Model {
	t.Helper()
	next, _ := m.Update(key(name))
	require.NotNil(t, next)
	return next
}

// msgOf runs a command to the message it yields, so two commands compare by output.
func msgOf(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	require.NotNil(t, cmd)
	return cmd()
}

// plain strips the ANSI styling so a View assertion reads what the user sees.
func plain(v tea.View) string {
	var b strings.Builder
	s := v.Content
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

func sampleModel() model[string] {
	return model[string]{choices: Choices[string]{
		Title:  "Pick a bridge",
		Items:  []string{"package.json", "pyproject.toml", "composer.json"},
		Label:  func(s string) string { return s },
		Detail: func(s string) string { return "policy:" + s[:1] },
	}}
}

// ── picker model ─────────────────────────────────────────────────────────────

func TestModelInit(t *testing.T) {
	assert.Nil(t, sampleModel().Init(), "the picker has no startup command")
}

func TestModelUpAndDownClampAtEdges(t *testing.T) {
	m := sampleModel()
	for range 3 {
		m = press(t, m, keyUp).(model[string])
	}
	assert.Equal(t, 0, m.cursor, "up at the top must stay at the top")

	for range 5 {
		m = press(t, m, keyDown).(model[string])
	}
	assert.Equal(t, 2, m.cursor, "down past the last item must clamp, not wrap")
}

func TestModelVimKeysMirrorArrows(t *testing.T) {
	m := sampleModel()
	m = press(t, m, "j").(model[string])
	assert.Equal(t, 1, m.cursor)
	m = press(t, m, "k").(model[string])
	assert.Equal(t, 0, m.cursor)
}

// TestModelSelectQuits is the load-bearing path.
func TestModelSelectQuits(t *testing.T) {
	for _, name := range []string{keyEnter, "space"} {
		t.Run(name, func(t *testing.T) {
			m := press(t, sampleModel(), keyDown).(model[string])
			next, cmd := m.Update(key(name))
			got := next.(model[string])
			assert.True(t, got.done, "%s must accept the form", name)
			assert.False(t, got.cancelled)
			require.NotNil(t, cmd, "%s must ask the program to quit", name)
			assert.Equal(t, msgOf(t, cmd), msgOf(t, tea.Quit))
		})
	}
}

// TestModelCancelQuits pins every back-out key.
func TestModelCancelQuits(t *testing.T) {
	for _, name := range []string{"q", keyEsc, keyCtrlC} {
		t.Run(name, func(t *testing.T) {
			next, cmd := sampleModel().Update(key(name))
			got := next.(model[string])
			assert.True(t, got.cancelled, "%s must cancel", name)
			assert.False(t, got.done, "%s must not select", name)
			require.NotNil(t, cmd)
			assert.Equal(t, msgOf(t, cmd), msgOf(t, tea.Quit))
		})
	}
}

func TestModelIgnoresUnrelatedMessages(t *testing.T) {
	m := sampleModel()
	for _, msg := range []tea.Msg{tea.MouseClickMsg{}, key("x"), key("left")} {
		next, cmd := m.Update(msg)
		got := next.(model[string])
		assert.Equal(t, 0, got.cursor, "%T must not move the cursor", msg)
		assert.False(t, got.done)
		assert.False(t, got.cancelled)
		assert.Nil(t, cmd)
	}
}

func TestModelViewRendersEveryItemAndTheCursor(t *testing.T) {
	m := sampleModel()
	view := plain(m.View())
	assert.Contains(t, view, "Pick a bridge")
	for _, item := range m.choices.Items {
		assert.Contains(t, view, item)
		assert.Contains(t, view, "policy:"+item[:1], "Detail must be rendered next to its item")
	}
	assert.Contains(t, view, "↑/↓ move, enter select, esc cancel")
	assert.Contains(t, view, ">  package.json", "the cursor marks the focused item")
	assert.NotContains(t, view, ">  pyproject.toml")
}

func TestModelViewFollowsTheCursor(t *testing.T) {
	m := press(t, sampleModel(), keyDown).(model[string])
	view := plain(m.View())
	assert.Contains(t, view, ">  pyproject.toml")
	assert.NotContains(t, view, ">  package.json")
}

func TestModelViewWithoutTitle(t *testing.T) {
	m := model[string]{choices: Choices[string]{Items: []string{"one"}, Label: func(s string) string { return s }}}
	view := plain(m.View())
	assert.Contains(t, view, "one")
	assert.NotContains(t, view, "Pick")
}

// TestModelViewEmptyDetailIsOmitted pins that an empty Detail adds no padding.
func TestModelViewEmptyDetailIsOmitted(t *testing.T) {
	m := model[string]{choices: Choices[string]{
		Items:  []string{"one"},
		Label:  func(s string) string { return s },
		Detail: func(string) string { return "" },
	}}
	assert.NotContains(t, plain(m.View()), "  \n")
}

// ── fill model ───────────────────────────────────────────────────────────────

// fillFields builds n labelled fields with distinct hints.
func fillFields(n int) []FillField {
	fields := make([]FillField, n)
	for i := range fields {
		fields[i] = FillField{
			Label: "field-" + string(rune('a'+i)),
			Hint:  "hint-" + string(rune('a'+i)),
		}
	}
	return fields
}

// TestNewFillModelFocusesFirst is the invariant every later keystroke depends on.
func TestNewFillModelFocusesFirst(t *testing.T) {
	m := newFillModel("Fill", fillFields(3))
	require.Len(t, m.inputs, 3)
	assert.Equal(t, 0, m.focusIndex)
	assert.True(t, m.inputs[0].Focused(), "the first input starts focused")
	assert.False(t, m.inputs[1].Focused())
	assert.Equal(t, "hint-a", m.inputs[0].Placeholder, "Hint is the placeholder")
}

func TestFillModelInit(t *testing.T) {
	assert.NotNil(t, newFillModel("Fill", fillFields(1)).Init(), "a text input needs its blink")
}

func TestFillModelCancelQuits(t *testing.T) {
	for _, name := range []string{keyEsc, keyCtrlC} {
		t.Run(name, func(t *testing.T) {
			next, cmd := newFillModel("Fill", fillFields(2)).Update(key(name))
			got := next.(fillModel)
			assert.True(t, got.cancelled, "%s must cancel", name)
			require.NotNil(t, cmd)
			assert.Equal(t, msgOf(t, cmd), msgOf(t, tea.Quit))
		})
	}
}

// TestFillModelEnterWalksThenAccepts is the load-bearing fill contract.
func TestFillModelEnterWalksThenAccepts(t *testing.T) {
	m := newFillModel("Fill", fillFields(3))

	m = press(t, m, keyEnter).(fillModel)
	assert.Equal(t, 1, m.focusIndex)
	assert.False(t, m.inputs[0].Focused(), "the abandoned input is blurred")
	assert.True(t, m.inputs[1].Focused())

	m = press(t, m, keyEnter).(fillModel)
	assert.Equal(t, 2, m.focusIndex)

	next, cmd := m.Update(key(keyEnter))
	assert.Equal(t, 2, next.(fillModel).focusIndex, "enter on the last field stays put")
	require.NotNil(t, cmd, "enter on the last field accepts the form")
	assert.Equal(t, msgOf(t, cmd), msgOf(t, tea.Quit))
}

// TestFillModelEnterOnSingleFieldAccepts covers the one-field form.
func TestFillModelEnterOnSingleFieldAccepts(t *testing.T) {
	_, cmd := newFillModel("Fill", fillFields(1)).Update(key(keyEnter))
	require.NotNil(t, cmd)
	assert.Equal(t, msgOf(t, cmd), msgOf(t, tea.Quit))
}

func TestFillModelTabWrapsForward(t *testing.T) {
	m := newFillModel("Fill", fillFields(2))
	m = press(t, m, keyTab).(fillModel)
	assert.Equal(t, 1, m.focusIndex)
	m = press(t, m, keyDown).(fillModel)
	assert.Equal(t, 0, m.focusIndex, "tab cycles back to the first field")
	assert.True(t, m.inputs[0].Focused())
	assert.False(t, m.inputs[1].Focused())
}

// TestFillModelShiftTabWrapsBackward pins the wrap at index 0.
func TestFillModelShiftTabWrapsBackward(t *testing.T) {
	m := newFillModel("Fill", fillFields(3))
	m = press(t, m, keyShiftTab).(fillModel)
	assert.Equal(t, 2, m.focusIndex, "shift+tab from the first field wraps to the last")
	m = press(t, m, keyUp).(fillModel)
	assert.Equal(t, 1, m.focusIndex)
	m = press(t, m, keyUp).(fillModel)
	assert.Equal(t, 0, m.focusIndex)
}

// TestFillModelRoutesTypingToFocus proves an unhandled key still reaches the focused input.
func TestFillModelRoutesTypingToFocus(t *testing.T) {
	m := newFillModel("Fill", fillFields(2))
	m = press(t, m, keyTab).(fillModel)
	for _, r := range "abc" {
		m = press(t, m, string(r)).(fillModel)
	}
	assert.Equal(t, "abc", m.inputs[1].Value(), "typing lands in the focused field")
	assert.Empty(t, m.inputs[0].Value(), "a blurred field must drop keystrokes")
}

func TestFillModelView(t *testing.T) {
	view := plain(newFillModel("Fill me", fillFields(2)).View())
	assert.Contains(t, view, "Fill me")
	assert.Contains(t, view, "field-a")
	assert.Contains(t, view, "field-b")
	assert.Contains(t, view, "hint-a")
	assert.Contains(t, view, ">  field-a", "the cursor marks the focused row")
	assert.NotContains(t, view, ">  field-b")
	assert.Contains(t, view, "tab/↓ next, enter accept (last field submits), esc cancel")
}

// TestFillModelViewWithoutHintOrTitle pins that both optional blocks drop out.
func TestFillModelViewWithoutHintOrTitle(t *testing.T) {
	view := plain(newFillModel("", []FillField{{Label: "only"}}).View())
	assert.Contains(t, view, "only")
	assert.NotContains(t, view, "hint")
	assert.NotContains(t, strings.ReplaceAll(view, "\n", ""), "    \n", "no blank hint line")
}

func TestFillModelViewFollowsFocus(t *testing.T) {
	m := press(t, newFillModel("Fill", fillFields(2)), keyTab).(fillModel)
	view := plain(m.View())
	assert.Contains(t, view, ">  field-b")
	assert.NotContains(t, view, ">  field-a")
}

// ── MultiInput ───────────────────────────────────────────────────────────────

// threeInputs builds n blank text inputs.
func threeInputs() []textinput.Model {
	return []textinput.Model{textinput.New(), textinput.New(), textinput.New()}
}

func TestMultiInputNilFocusableFocusesAll(t *testing.T) {
	m := NewMultiInput(threeInputs(), nil)
	assert.Equal(t, 0, m.Focused())
	assert.Equal(t, []int{0, 1, 2}, m.focusable, "nil focusable means every input")

	m.Next()
	assert.Equal(t, 1, m.Focused())
}

func TestMultiInputAccessors(t *testing.T) {
	m := NewMultiInput(threeInputs(), []int{1})
	assert.Equal(t, 1, m.Focused(), "focus starts on the first focusable entry")
	assert.Len(t, m.Inputs(), 3)
	assert.True(t, m.Input(1).Focused())
	assert.False(t, m.Input(0).Focused())
}

func TestMultiInputNavigationWraps(t *testing.T) {
	m := NewMultiInput(threeInputs(), nil)
	for i, want := range []int{1, 2, 0} {
		m.Next()
		assert.Equal(t, want, m.Focused(), "Next step %d", i)
	}
	m.Prev()
	assert.Equal(t, 2, m.Focused(), "Prev wraps at the start")
	m.Prev()
	assert.Equal(t, 1, m.Focused())
}

func TestMultiInputJumpToFocusable(t *testing.T) {
	m := NewMultiInput(threeInputs(), []int{0, 2})
	m.JumpTo(2)
	assert.Equal(t, 2, m.Focused())
	m.JumpTo(0)
	assert.Equal(t, 0, m.Focused())
}

func TestMultiInputUpdateRoutesToFocus(t *testing.T) {
	m := NewMultiInput(threeInputs(), []int{0, 2})
	m.JumpTo(2)
	for _, r := range "hi" {
		m.Update(key(string(r)))
	}
	assert.Equal(t, "hi", m.Input(2).Value())
	assert.Empty(t, m.Input(0).Value())
}

// TestMultiInputFocusBlursEveryInput is the invariant the type exists to hold.
func TestMultiInputFocusBlursEveryInput(t *testing.T) {
	m := NewMultiInput(threeInputs(), nil)
	m.Next()
	m.Next()

	focused := 0
	for i := range m.Inputs() {
		if m.Input(i).Focused() {
			focused++
		}
	}
	assert.Equal(t, 1, focused)
	assert.Equal(t, 2, m.Focused())
}

// TestMultiInputUpdatesWhileFocusedAtIndexZero pins the Update guard's lower bound.
func TestMultiInputUpdatesWhileFocusedAtIndexZero(t *testing.T) {
	m := NewMultiInput([]textinput.Model{textinput.New(), textinput.New()}, nil)
	require.Equal(t, 0, m.Focused(), "focus starts on the first input")

	m.Update(key("a"))
	m.Update(key("b"))

	assert.Equal(t, "ab", m.Input(0).Value(), "index 0 is a real focus, not the inert sentinel")
	assert.Empty(t, m.Input(1).Value())
}

// TestMultiInputInertUpdateReturnsNothing pins the other end.
func TestMultiInputInertUpdateReturnsNothing(t *testing.T) {
	m := NewMultiInput([]textinput.Model{textinput.New()}, []int{})
	require.Equal(t, -1, m.Focused())
	assert.Nil(t, m.Update(key("a")))
	assert.Empty(t, m.Input(0).Value())
}

// TestMultiInputUpdateAfterWrapReturnsToZero is the same guard reached the long way.
func TestMultiInputUpdateAfterWrapReturnsToZero(t *testing.T) {
	m := NewMultiInput([]textinput.Model{textinput.New(), textinput.New()}, nil)
	m.Next()
	require.Equal(t, 1, m.Focused())
	m.Next()
	require.Equal(t, 0, m.Focused(), "Next wrapped back to the first input")

	m.Update(key("z"))
	assert.Equal(t, "z", m.Input(0).Value(), "a wrapped-to-zero focus still receives its keystrokes")
}
