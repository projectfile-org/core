// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package rawdoc_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/internal/rawdoc"
)

// ── OrderedJSON ──────────────────────────────────────────────────────────────

func TestOrderedJSONKeyOrder(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, json.Unmarshal([]byte(`{"z":1,"a":2,"m":3}`), om))
	assert.Equal(t, []string{"z", "a", "m"}, om.Keys())
}

func TestOrderedJSONRoundTripContent(t *testing.T) {
	input := `{"name":"foo","version":"1.0.0","scripts":{"build":"tsc"}}`
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, json.Unmarshal([]byte(input), om))
	out, err := json.Marshal(om)
	require.NoError(t, err)
	assert.JSONEq(t, input, string(out))
}

func TestOrderedJSONSetPreservesOrder(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, json.Unmarshal([]byte(`{"a":"1","b":"2","c":"3"}`), om))
	om.Set("b", json.RawMessage(`"updated"`))
	assert.Equal(t, []string{"a", "b", "c"}, om.Keys(), "set must not reorder keys")
	v, ok := om.Get("b")
	require.True(t, ok)
	assert.Equal(t, json.RawMessage(`"updated"`), v)
}

func TestOrderedJSONSetNewKeyAppends(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, json.Unmarshal([]byte(`{"a":1}`), om))
	om.Set("z", json.RawMessage(`99`))
	assert.Equal(t, []string{"a", "z"}, om.Keys())
}

func TestOrderedJSONDelete(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, json.Unmarshal([]byte(`{"a":1,"b":2,"c":3}`), om))
	om.Delete("b")
	assert.Equal(t, []string{"a", "c"}, om.Keys())
	assert.False(t, om.Has("b"))
}

func TestOrderedJSONDeleteNoop(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, json.Unmarshal([]byte(`{"a":1}`), om))
	om.Delete("nonexistent") // must not panic or change keys
	assert.Equal(t, []string{"a"}, om.Keys())
}

func TestOrderedJSONCloneIndependence(t *testing.T) {
	om := rawdoc.NewOrderedJSON()
	require.NoError(t, json.Unmarshal([]byte(`{"x":1,"y":2}`), om))
	cp := om.Clone()
	cp.Set("x", json.RawMessage(`99`))
	original, _ := om.Get("x")
	assert.Equal(t, json.RawMessage(`1`), original, "clone mutation must not affect original")
}

func TestOrderedJSONCloneNil(t *testing.T) {
	var om *rawdoc.OrderedJSON
	assert.Nil(t, om.Clone())
}

// ── YAMLNode ─────────────────────────────────────────────────────────────────

func TestYAMLNodePreservesHeadComment(t *testing.T) {
	input := "# leading comment\nfoo: bar\nbaz: qux\n"
	y, err := rawdoc.FromBytes([]byte(input))
	require.NoError(t, err)
	out, err := y.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(out), "leading comment")
}

func TestYAMLNodeKeyOrder(t *testing.T) {
	y, err := rawdoc.FromBytes([]byte("z: 1\na: 2\nm: 3\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"z", "a", "m"}, y.Keys())
}

func TestYAMLNodeSetValueUpdatesInPlace(t *testing.T) {
	y, err := rawdoc.FromBytes([]byte("foo: old\nbar: unchanged\n"))
	require.NoError(t, err)
	require.NoError(t, y.SetValue("foo", "new"))
	n, ok := y.Get("foo")
	require.True(t, ok)
	assert.Equal(t, "new", n.Value)
	assert.Equal(t, []string{"foo", "bar"}, y.Keys(), "order must be preserved after update")
}

func TestYAMLNodeSetValueNewKeyAppends(t *testing.T) {
	y, err := rawdoc.FromBytes([]byte("a: 1\n"))
	require.NoError(t, err)
	require.NoError(t, y.SetValue("z", "added"))
	assert.Equal(t, []string{"a", "z"}, y.Keys())
}

func TestYAMLNodeDelete(t *testing.T) {
	y, err := rawdoc.FromBytes([]byte("a: 1\nb: 2\nc: 3\n"))
	require.NoError(t, err)
	y.Delete("b")
	assert.False(t, y.Has("b"))
	assert.Equal(t, []string{"a", "c"}, y.Keys())
}

func TestYAMLNodeCloneIndependence(t *testing.T) {
	y, err := rawdoc.FromBytes([]byte("x: original\n"))
	require.NoError(t, err)
	cp := y.Clone()
	require.NoError(t, cp.SetValue("x", "modified"))
	n, _ := y.Get("x")
	assert.Equal(t, "original", n.Value, "clone mutation must not affect original")
}

func TestYAMLNodeEmptyInput(t *testing.T) {
	y, err := rawdoc.FromBytes([]byte(""))
	require.NoError(t, err)
	assert.Empty(t, y.Keys())
}

// ── OrderedTOML ──────────────────────────────────────────────────────────────

func TestOrderedTOMLKeyOrder(t *testing.T) {
	ot, err := rawdoc.TOMLFromBytes([]byte("z = 1\na = 2\nm = 3\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"z", "a", "m"}, ot.Keys())
}

func TestOrderedTOMLSetPreservesOrder(t *testing.T) {
	ot := rawdoc.NewOrderedTOML()
	ot.Set("first", "A")
	ot.Set("second", "B")
	ot.Set("first", "updated")
	assert.Equal(t, []string{"first", "second"}, ot.Keys())
	v, _ := ot.Get("first")
	assert.Equal(t, "updated", v)
}

func TestOrderedTOMLDelete(t *testing.T) {
	ot, err := rawdoc.TOMLFromBytes([]byte("a = 1\nb = 2\nc = 3\n"))
	require.NoError(t, err)
	ot.Delete("b")
	assert.Equal(t, []string{"a", "c"}, ot.Keys())
	assert.False(t, ot.Has("b"))
}

func TestOrderedTOMLCloneIndependence(t *testing.T) {
	ot := rawdoc.NewOrderedTOML()
	ot.Set("x", "original")
	cp := ot.Clone()
	cp.Set("x", "modified")
	v, _ := ot.Get("x")
	assert.Equal(t, "original", v, "clone mutation must not affect original")
}

func TestOrderedTOMLCloneNil(t *testing.T) {
	var ot *rawdoc.OrderedTOML
	assert.Nil(t, ot.Clone())
}

func TestOrderedTOMLMarshalRoundTrip(t *testing.T) {
	input := []byte("name = \"my-proj\"\nversion = \"1.0.0\"\n")
	ot, err := rawdoc.TOMLFromBytes(input)
	require.NoError(t, err)
	out, err := ot.Marshal()
	require.NoError(t, err)
	// Re-parse and compare values (byte order may differ after go-toml re-serialises)
	ot2, err := rawdoc.TOMLFromBytes(out)
	require.NoError(t, err)
	v, ok := ot2.Get("name")
	require.True(t, ok)
	assert.Equal(t, "my-proj", v)
}
