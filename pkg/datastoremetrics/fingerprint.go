// Copyright (C) 2025-2026, Hanzo AI Inc. All rights reserved.
// SPDX-License-Identifier: BSD-3-Clause

package datastoremetrics

import "sort"

// Metric-series fingerprinting — the native port of the labels-hash the
// datastore metrics schema uses to join a sample (samples_v4) to its series
// metadata (time_series_v4). It is a plain FNV-1a walk over the sorted
// (key, value) attribute pairs with a 0xFF separator, seeded by a parent
// offset so the hierarchy resource → scope → point composes: the scope hash
// seeds the point hash, which is finally salted with the metric __name__.
//
// This is the minimal, dependency-free equivalent of the histogram-fork's
// fingerprint package (which lives behind an internal/ import wall and pulls
// OTLP pdata). Reproducing the exact constants here keeps the native writer on
// upstream ch-go with NO fork dependency while still producing byte-identical
// join keys, so the existing query plane reads what this writer writes.
const (
	// initialOffset is the FNV-1a 64-bit offset basis — the seed for the
	// top-level (resource) fingerprint.
	initialOffset uint64 = 14695981039346656037
	prime64       uint64 = 1099511628211
	// separatorByte delimits key from value and pair from pair, so
	// {a=bc} and {ab=c} hash differently.
	separatorByte byte = 255
)

func hashAdd(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}

func hashAddByte(h uint64, b byte) uint64 {
	h ^= uint64(b)
	h *= prime64
	return h
}

// fingerprint folds attrs into a hash seeded by offset. attrs is walked in
// sorted key order so the result is independent of map iteration order.
func fingerprint(offset uint64, attrs map[string]string) uint64 {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	h := offset
	for _, k := range keys {
		h = hashAdd(h, k)
		h = hashAddByte(h, separatorByte)
		h = hashAdd(h, attrs[k])
		h = hashAddByte(h, separatorByte)
	}
	return h
}

// hashWithName salts a series fingerprint with the metric name, matching the
// schema's __name__ dimension. This is the value stored in the `fingerprint`
// column of both samples_v4 and time_series_v4.
func hashWithName(h uint64, name string) uint64 {
	sum := hashAdd(h, "__name__")
	sum = hashAddByte(sum, separatorByte)
	sum = hashAdd(sum, name)
	return sum
}

// mergeAttrs overlays maps left-to-right (later wins) into a fresh map. Used
// to compose point + scope + resource attributes for the `labels` JSON with
// the same precedence the schema expects (resource overrides point on clash).
func mergeAttrs(maps ...map[string]string) map[string]string {
	out := make(map[string]string)
	for _, m := range maps {
		for k, v := range m {
			out[k] = v
		}
	}
	return out
}

// marshalLabels renders attrs as the compact, key-sorted JSON object the
// datastore's JSON functions consume for the `labels` column. It is the native
// port of the schema's label marshaller: sorted keys, minimal escaping, no
// reflection. __name__ is expected to already be present in attrs.
func marshalLabels(attrs map[string]string) string {
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	if len(keys) == 0 {
		return "{}"
	}

	b := make([]byte, 0, 128)
	b = append(b, '{')
	for _, name := range keys {
		b = append(b, '"')
		b = append(b, name...)
		b = append(b, '"', ':', '"')
		for _, c := range []byte(attrs[name]) {
			switch c {
			case '\\', '"':
				b = append(b, '\\', c)
			case '\n':
				b = append(b, '\\', 'n')
			case '\r':
				b = append(b, '\\', 'r')
			case '\t':
				b = append(b, '\\', 't')
			default:
				b = append(b, c)
			}
		}
		b = append(b, '"', ',')
	}
	b[len(b)-1] = '}' // replace trailing comma
	return string(b)
}

// labelsJSON builds the `labels` column value for a series: the merged
// point/scope/resource attributes plus __name__, key-sorted and JSON-encoded.
func labelsJSON(name string, point, scope, resource map[string]string) string {
	merged := mergeAttrs(point, scope, resource)
	merged["__name__"] = name
	return marshalLabels(merged)
}
