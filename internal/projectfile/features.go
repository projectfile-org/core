// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package projectfile

import (
	"fmt"
	"math"
	"reflect"
)

// FeaturesLevelKey is the key inside a tool's extension namespace naming the
// newest document feature level the projectfile relies on (Android-style API
// level). A tool refuses a document declaring a higher level than it supports
// instead of silently misreading a language it cannot see.
const FeaturesLevelKey = "features-level"

// CheckFeaturesLevel refuses a document whose declared features level under ns
// exceeds maxSupported. An absent namespace, non-map namespace, or absent key
// means the oldest level and is compatible. A present-but-malformed value is a
// hard error, like an exceeding one.
func CheckFeaturesLevel(doc *Document, ns string, maxSupported int) error {
	v, ok := LookupExtension(doc, ns)
	if !ok {
		return nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	lv, ok := m[FeaturesLevelKey]
	if !ok {
		return nil
	}
	n, ok := asNonNegativeInt(lv)
	if !ok {
		return fmt.Errorf("invalid %s %s %v: must be a non-negative integer", ns, FeaturesLevelKey, lv)
	}
	if n > maxSupported {
		return fmt.Errorf("projectfile requires %s %s %d, but this tool supports at most %d", ns, FeaturesLevelKey, n, maxSupported)
	}
	return nil
}

// asNonNegativeInt coerces YAML (int), TOML (int64), and JSON (float64)
// decodings of one integer; anything else (strings, fractions, negatives,
// overflow) reports false so the caller fails closed on garbage.
func asNonNegativeInt(v any) (int, bool) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n := rv.Int(); n >= 0 {
			return int(n), true
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if u := rv.Uint(); u <= uint64(math.MaxInt) {
			return int(u), true
		}
	case reflect.Float32, reflect.Float64:
		if f := rv.Float(); f >= 0 && f == math.Trunc(f) && f <= math.MaxInt {
			return int(f), true
		}
	}
	return 0, false
}
