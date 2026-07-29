// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package fieldpath implements the dotted-path + selector + projection
// address grammar shared by the pf-cli get/set/add/del verbs. The grammar
// generalises the syntax originally prototyped in internal/derive/fields.go
// (links[type=X]) into a single addressing system every consumer can rely
// on. See the plan accompanying this package for the full grammar.
package fieldpath

import (
	"fmt"
	"strconv"
	"strings"
)

// SegKind names the kinds of step a Path can take. Key segments traverse
// map keys; Index/Selector/Project segments operate on the list value
// produced by the immediately-preceding Key segment.
type SegKind int

const (
	SegKey         SegKind = iota // map key
	SegIndex                      // list[N]
	SegSelector                   // list[k=v,...]
	SegProject                    // list[]
	SegMapProject                 // map{} — fan out KEY=VALUE pairs
	SegMapSelector                // map{k=v,...} — fan out MATCHING values
)

// Predicate is one k=v equality term inside a selector segment.
type Predicate struct {
	Key, Value string
}

// Segment is one step of an address.
type Segment struct {
	Kind  SegKind
	Key   string      // SegKey
	Index int         // SegIndex (negative = from end)
	Preds []Predicate // SegSelector, SegMapSelector
}

// Path is a parsed address. Raw is preserved for error messages so users
// see the same text they typed; segments are the canonical form.
type Path struct {
	Raw      string
	Segments []Segment
}

// String renders a Path back into its canonical text form. Used in error
// messages and the dry-run printer.
func (p Path) String() string {
	if p.Raw != "" {
		return p.Raw
	}
	var b strings.Builder
	for i, s := range p.Segments {
		switch s.Kind {
		case SegKey:
			if i > 0 {
				b.WriteByte('.')
			}
			b.WriteString(s.Key)
		case SegIndex:
			fmt.Fprintf(&b, "[%d]", s.Index)
		case SegSelector:
			fmt.Fprintf(&b, "[%s]", PredicateBody(s.Preds))
		case SegProject:
			b.WriteString("[]")
		case SegMapProject:
			b.WriteString("{}")
		case SegMapSelector:
			fmt.Fprintf(&b, "{%s}", PredicateBody(s.Preds))
		}
	}
	return b.String()
}

// PredicateBody renders predicates back into the `k=v,…` text the user typed,
// without brackets — the caller wraps them in the form the address arrived in
// (`[…]` for a list selector, `{…}` for a map one). Exists so Path.String and
// the resolver's error labels cannot drift apart on how a predicate reads.
func PredicateBody(preds []Predicate) string {
	var b strings.Builder
	for i, pr := range preds {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%s=%s", pr.Key, pr.Value)
	}
	return b.String()
}

// Parse turns an address string into a Path. The ext.<ns> shortcut is
// expanded to org.<ns> at parse time so downstream resolution always sees
// the canonical reverse-DNS form.
func Parse(s string) (Path, error) {
	raw := s
	if s == "" {
		return Path{}, fmt.Errorf("fieldpath: empty path")
	}
	// ext.<ns> — shorthand for accessing any extension namespace. Strip the
	// prefix so the remaining path (e.g. com.example.env) resolves through
	// the nested maps that ToMap()/nestExtension produce. No rewrite to org.:
	// the shorthand applies to all reverse-DNS namespaces, not just org.*.
	s = strings.TrimPrefix(s, "ext.")

	pieces, err := splitOnDots(s)
	if err != nil {
		return Path{}, err
	}

	segs := make([]Segment, 0, len(pieces))
	for _, piece := range pieces {
		key, brackets, err := splitKeyBrackets(piece)
		if err != nil {
			return Path{}, err
		}
		if key != "" {
			segs = append(segs, Segment{Kind: SegKey, Key: key})
		}
		for _, br := range brackets {
			bs, err := parseBracket(br)
			if err != nil {
				return Path{}, err
			}
			segs = append(segs, bs)
		}
	}
	if len(segs) == 0 {
		return Path{}, fmt.Errorf("fieldpath: no segments parsed from %q", raw)
	}
	return Path{Raw: raw, Segments: segs}, nil
}

// splitOnDots splits s on '.' that are NOT inside brackets or quoted
// strings. Bracket depth (both [] and {}) and double-quote state are
// tracked so the selector grammar's `links[type=foo,name="my.thing"]`
// survives the split. Curly braces participate in the depth counter
// even though the grammar admits only `{}` (no body) — defensive,
// keeps the rule "dots inside brackets stay put" uniform.
func splitOnDots(s string) ([]string, error) {
	out := []string{}
	depth := 0
	inQuote := false
	last := 0
	for i := range len(s) {
		c := s[i]
		switch {
		case inQuote:
			if c == '"' {
				inQuote = false
			}
		case c == '"':
			inQuote = true
		case c == '[' || c == '{':
			depth++
		case c == ']' || c == '}':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("fieldpath: unmatched %q at position %d in %q", c, i, s)
			}
		case c == '.' && depth == 0:
			out = append(out, s[last:i])
			last = i + 1
		}
	}
	if inQuote {
		return nil, fmt.Errorf("fieldpath: unterminated quoted string in %q", s)
	}
	if depth != 0 {
		return nil, fmt.Errorf("fieldpath: unmatched bracket in %q", s)
	}
	out = append(out, s[last:])
	return out, nil
}

// bracketPiece is the parsed form of one bracket-pair on a piece: the
// opener byte ('[' or '{') and the literal body between opener and the
// matching close. Carrying the opener lets parseBracket dispatch on
// grammar without re-scanning.
type bracketPiece struct {
	Open byte
	Body string
}

// splitKeyBrackets splits a single piece like `links[type=bugs][0]` or
// `env{}.keys` into its key (`links` / `env`) and the trailing bracket
// pairs. Both `[]` and `{}` are accepted; the opener is preserved so the
// caller can distinguish list addressing from map projection. Empty
// piece, or piece that begins with a bracket on a continuation segment
// (`.[0]`), is an error.
func splitKeyBrackets(piece string) (key string, brackets []bracketPiece, err error) {
	i := strings.IndexAny(piece, "[{")
	if i < 0 {
		if piece == "" {
			return "", nil, fmt.Errorf("fieldpath: empty segment")
		}
		return piece, nil, nil
	}
	key = piece[:i]
	rest := piece[i:]
	for len(rest) > 0 {
		open := rest[0]
		if open != '[' && open != '{' {
			return "", nil, fmt.Errorf("fieldpath: unexpected text after bracket in %q", piece)
		}
		end := findBracketEnd(rest)
		if end < 0 {
			return "", nil, fmt.Errorf("fieldpath: unmatched %q in %q", open, piece)
		}
		brackets = append(brackets, bracketPiece{Open: open, Body: rest[1:end]})
		rest = rest[end+1:]
	}
	if key == "" {
		// A segment that opens with a bracket has no host to address —
		// `repositories.[0]` is a typo for `repositories[0]`, and `{}` on
		// its own has no map to fan out. Either way, reject early so the
		// error points at the missing prefix instead of at some downstream
		// resolution miss.
		return "", nil, fmt.Errorf("fieldpath: bracket without preceding key in piece %q", piece)
	}
	return key, brackets, nil
}

// findBracketEnd locates the closing bracket that matches the opener at
// s[0] (`[` → `]`, `{` → `}`), skipping over quoted regions so a quoted
// value like `"x]y"` doesn't break the match. Returns -1 when no close
// is found.
func findBracketEnd(s string) int {
	if len(s) == 0 {
		return -1
	}
	var closer byte
	switch s[0] {
	case '[':
		closer = ']'
	case '{':
		closer = '}'
	default:
		return -1
	}
	inQuote := false
	for i := 1; i < len(s); i++ {
		c := s[i]
		switch {
		case inQuote:
			if c == '"' {
				inQuote = false
			}
		case c == '"':
			inQuote = true
		case c == closer:
			return i
		}
	}
	return -1
}

// parseBracket turns one bracket-pair into a Segment. `{...}` is the map form
// — an empty body (`{}`) fans every entry out as KEY=VALUE pairs, a predicate
// body (`{kind=image}`) selects the entries whose fields match. `[...]` keeps
// the original three arms — empty body → SegProject, integer → SegIndex,
// otherwise → SegSelector.
//
// A bare key list (`{key1,key2}` subselect) is still not grammar: every body
// with no `=` is refused by parsePredicates, so it fails loudly rather than
// silently matching nothing.
func parseBracket(bp bracketPiece) (Segment, error) {
	body := strings.TrimSpace(bp.Body)
	if bp.Open == '{' {
		if body == "" {
			return Segment{Kind: SegMapProject}, nil
		}
		preds, err := parsePredicates(body)
		if err != nil {
			return Segment{}, err
		}
		return Segment{Kind: SegMapSelector, Preds: preds}, nil
	}
	if body == "" {
		return Segment{Kind: SegProject}, nil
	}
	// An integer (possibly negative) is a positional index. We deliberately
	// accept leading '+' as well so '+0' and '0' behave the same; negative
	// values index from the end (Python-style) and are resolved in the
	// walker, not here.
	if n, err := strconv.Atoi(body); err == nil {
		return Segment{Kind: SegIndex, Index: n}, nil
	}
	preds, err := parsePredicates(body)
	if err != nil {
		return Segment{}, err
	}
	return Segment{Kind: SegSelector, Preds: preds}, nil
}

// parsePredicates splits a selector body into k=v Predicate entries. The
// split honours double-quoted values so commas/equals inside the quotes
// don't break the parse. Quotes are stripped from the value on the way
// out — they are an escape-mechanism, not part of the stored value.
func parsePredicates(body string) ([]Predicate, error) {
	parts, err := splitOnTopLevelCommas(body)
	if err != nil {
		return nil, err
	}
	preds := make([]Predicate, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		eq := strings.IndexByte(p, '=')
		if eq < 0 {
			return nil, fmt.Errorf("fieldpath: predicate %q missing '='", p)
		}
		k := strings.TrimSpace(p[:eq])
		v := strings.TrimSpace(p[eq+1:])
		if k == "" {
			return nil, fmt.Errorf("fieldpath: predicate %q has empty key", p)
		}
		v = unquote(v)
		preds = append(preds, Predicate{Key: k, Value: v})
	}
	if len(preds) == 0 {
		return nil, fmt.Errorf("fieldpath: selector body has no predicates")
	}
	return preds, nil
}

// splitOnTopLevelCommas mirrors splitOnDots for predicate bodies: commas
// inside double-quoted spans are not separators.
func splitOnTopLevelCommas(s string) ([]string, error) {
	out := []string{}
	inQuote := false
	last := 0
	for i := range len(s) {
		c := s[i]
		switch {
		case inQuote:
			if c == '"' {
				inQuote = false
			}
		case c == '"':
			inQuote = true
		case c == ',':
			out = append(out, s[last:i])
			last = i + 1
		}
	}
	if inQuote {
		return nil, fmt.Errorf("fieldpath: unterminated quoted string in predicate %q", s)
	}
	out = append(out, s[last:])
	return out, nil
}

// unquote strips a single pair of wrapping double quotes when present.
// Internal escape sequences are NOT honoured — the grammar quotes only to
// protect commas/equals/closing-brackets inside a value, not to support
// arbitrary string literals.
func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
