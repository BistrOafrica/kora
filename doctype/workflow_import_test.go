package doctype

import "testing"

func TestNormalizeAllowEditForDatabase(t *testing.T) {
	tests := map[string]string{
		"":                  "0",
		"0":                 "0",
		"false":             "0",
		"no":                "0",
		"Administrator":     "1",
		"Operator, Manager": "1",
		"1":                 "1",
	}
	for input, want := range tests {
		if got := normalizeAllowEdit(input); got != want {
			t.Errorf("normalizeAllowEdit(%q) = %q, want %q", input, got, want)
		}
	}
}
