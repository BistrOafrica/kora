package contract

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestPageManifestRoundTrip(t *testing.T) {
	m := PageManifest{
		SchemaVersion: "1",
		Page:          "sales-dashboard",
		Title:         "Sales Dashboard",
		Regions: []Region{{
			Name:   "main",
			Layout: Layout{Desktop: 12, Tablet: 12, Mobile: 4},
			Blocks: []Block{{
				Kind:       "collection",
				Collection: "sales.summary",
				Actions:    []ActionRef{{Label: "Create", Command: "record.create", IdempotencyKey: true}},
			}},
		}},
		States: ManifestStates{Loading: "loading", Empty: "empty", Error: "error", Offline: "offline"},
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	back, err := LoadPageManifestJSON(raw)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if back.Page != m.Page || back.Title != m.Title || len(back.Regions) != 1 || back.Regions[0].Blocks[0].Actions[0].Command != "record.create" {
		t.Fatalf("round-trip mismatch: %+v", back)
	}
}

func TestManifestRejectsUnknownKeys(t *testing.T) {
	for _, tc := range []struct{ json, key string }{
		{`{"schema_version":"1","page":"p","title":"t","unknown":1}`, "unknown"},
		{`{"schema_version":"1","page":"p","title":"t","regions":[{"name":"r","bocks":[]}]}`, "bocks"},
	} {
		_, err := LoadPageManifestJSON([]byte(tc.json))
		var inv *ErrUIManifestInvalid
		if !errors.As(err, &inv) {
			t.Fatalf("%s: want *ErrUIManifestInvalid, got %T: %v", tc.key, err, err)
		}
		if inv.Path != tc.key {
			t.Fatalf("path = %q, want %q", inv.Path, tc.key)
		}
	}
}

func TestManifestValidationRules(t *testing.T) {
	base := PageManifest{SchemaVersion: "1", Page: "p", Title: "t"}
	for name, m := range map[string]PageManifest{
		"no page":           {SchemaVersion: "1", Title: "t"},
		"no title":          {SchemaVersion: "1", Page: "p"},
		"bad kind":          {SchemaVersion: "1", Page: "p", Title: "t", Regions: []Region{{Name: "r", Blocks: []Block{{Kind: "react"}}}}},
		"dup region":        {SchemaVersion: "1", Page: "p", Title: "t", Regions: []Region{{Name: "r"}, {Name: "r"}}},
		"action no command": {SchemaVersion: "1", Page: "p", Title: "t", Regions: []Region{{Name: "r", Blocks: []Block{{Kind: "form", Actions: []ActionRef{{Label: "x"}}}}}}},
	} {
		_ = base
		if err := m.Validate(); err == nil {
			t.Fatalf("%s: expected validation error", name)
		} else {
			var inv *ErrUIManifestInvalid
			if !errors.As(err, &inv) {
				t.Fatalf("%s: want *ErrUIManifestInvalid, got %T", name, err)
			}
		}
	}
}

func TestManifestHasNoFrameworkIdentifiers(t *testing.T) {
	// The wire schema must never carry framework/DOM identifiers. Serialize a
	// full manifest and assert the well-known framework terms are absent.
	m := PageManifest{SchemaVersion: "1", Page: "p", Title: "t", Regions: []Region{{Name: "r", Blocks: []Block{{Kind: "record"}}}}}
	raw, _ := json.Marshal(m)
	lower := strings.ToLower(string(raw))
	for _, term := range []string{"react", "vue", "svelte", "dom", "jsx", "html"} {
		if strings.Contains(lower, term) {
			t.Fatalf("framework identifier %q leaked into manifest schema: %s", term, raw)
		}
	}
}
