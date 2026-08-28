package component

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"

	goyaml "github.com/goccy/go-yaml"
)

var unknownFieldRe = regexp.MustCompile(`unknown field "([^"]+)"`)

// knownFields is the flat set of manifest field names used for "did you mean"
// suggestions. It covers both top-level and nested keys.
var knownFields = []string{
	"name", "version", "runtime", "provides", "requires", "events", "emits", "handles", "type",
}

// LoadJSON decodes a JSON manifest strictly (unknown fields rejected) and
// validates it. It returns a typed *ErrManifestInvalid for unknown keys or
// semantic validation failures.
func LoadJSON(data []byte, events EventTypeRegistry) (*Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		if field := unknownField(err.Error()); field != "" {
			return nil, &ErrManifestInvalid{Path: field, Detail: "unknown field", Suggestion: suggestField(field)}
		}
		return nil, fmt.Errorf("component: parse json: %w", err)
	}
	if err := m.Validate(events); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadYAML decodes a YAML manifest strictly and validates it. YAML is
// interchange only; the typed Manifest is authoritative.
func LoadYAML(data []byte, events EventTypeRegistry) (*Manifest, error) {
	var m Manifest
	if err := goyaml.UnmarshalWithOptions(data, &m, goyaml.DisallowUnknownField()); err != nil {
		if field := unknownField(err.Error()); field != "" {
			return nil, &ErrManifestInvalid{Path: field, Detail: "unknown field", Suggestion: suggestField(field)}
		}
		return nil, fmt.Errorf("component: parse yaml: %w", err)
	}
	if err := m.Validate(events); err != nil {
		return nil, err
	}
	return &m, nil
}

func unknownField(errMsg string) string {
	m := unknownFieldRe.FindStringSubmatch(errMsg)
	if len(m) == 2 {
		return m[1]
	}
	return ""
}

// suggestField returns the closest known field name within Levenshtein distance
// ≤ 3, or an empty string when nothing is close enough.
func suggestField(unknown string) string {
	best := ""
	bestDist := 1 << 30
	for _, k := range knownFields {
		if d := levenshtein(unknown, k); d < bestDist && d <= 3 {
			bestDist = d
			best = k
		}
	}
	return best
}

func levenshtein(a, b string) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}
