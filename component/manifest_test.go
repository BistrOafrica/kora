package component

import (
	"encoding/json"
	"errors"
	"testing"

	goyaml "github.com/goccy/go-yaml"
)

func eventsRegistry(t *testing.T) EventTypeRegistry {
	return func(eventType string) bool {
		return eventType == "sms.sent"
	}
}

func validManifest() Manifest {
	return Manifest{
		Name:            "sms.twilio",
		Version:         "1.2.3",
		Runtime:         RuntimeGoja,
		BindingStrategy: BindingStrategyPerOp,
		Lifecycle:       LifecycleActive,
		Provides:        []string{"sms.send"},
		Requires:        []string{"http.outbound"},
		Events: EventsDecl{
			Emits: []EventDecl{{Type: "sms.sent", Version: 1}},
		},
	}
}

func TestValidateHappyPath(t *testing.T) {
	if err := validManifest().Validate(eventsRegistry(t)); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
}

func TestSemverAndNameValidation(t *testing.T) {
	for name, m := range map[string]Manifest{
		"bad name":      {Name: "SMS.Twilio", Version: "1.0.0", Runtime: RuntimeGoja},
		"no name":       {Version: "1.0.0", Runtime: RuntimeGoja},
		"bad semver":    {Name: "sms.twilio", Version: "not-semver", Runtime: RuntimeGoja},
		"no version":    {Name: "sms.twilio", Runtime: RuntimeGoja},
		"bad runtime":   {Name: "sms.twilio", Version: "1.0.0", Runtime: "python"},
		"bad binding":   {Name: "sms.twilio", Version: "1.0.0", Runtime: RuntimeGoja, BindingStrategy: "sticky"},
		"bad lifecycle": {Name: "sms.twilio", Version: "1.0.0", Runtime: RuntimeGoja, Lifecycle: "paused"},
	} {
		if err := m.Validate(nil); err == nil {
			t.Fatalf("%s: expected validation error", name)
		} else {
			var inv *ErrManifestInvalid
			if !errors.As(err, &inv) {
				t.Fatalf("%s: want *ErrManifestInvalid, got %T", name, err)
			}
		}
	}
}

func TestDuplicateCapabilityRejected(t *testing.T) {
	// Duplicate within provides.
	m := validManifest()
	m.Provides = []string{"sms.send", "sms.send"}
	if err := m.Validate(nil); err == nil {
		t.Fatalf("expected duplicate-provides rejection")
	}

	// Same name across provides and requires.
	m = validManifest()
	m.Requires = []string{"sms.send"}
	if err := m.Validate(nil); err == nil {
		t.Fatalf("expected cross provides/requires rejection")
	}
}

func TestEventTypesMustBeRegistered(t *testing.T) {
	m := validManifest()
	m.Events.Emits = append(m.Events.Emits, EventDecl{Type: "sms.unknown", Version: 1})
	err := m.Validate(eventsRegistry(t))
	var inv *ErrManifestInvalid
	if !errors.As(err, &inv) {
		t.Fatalf("want *ErrManifestInvalid for unknown event, got %T: %v", err, err)
	}
	if inv.Path != "events" {
		t.Fatalf("path = %q, want events", inv.Path)
	}
}

func TestJSONRoundTrip(t *testing.T) {
	m := validManifest()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	back, err := LoadJSON(raw, eventsRegistry(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if back.Name != m.Name || back.Version != m.Version || back.Runtime != m.Runtime || len(back.Provides) != 1 {
		t.Fatalf("round-trip mismatch: %+v", back)
	}
	if back.BindingStrategy != m.BindingStrategy || back.Lifecycle != m.Lifecycle {
		t.Fatalf("round-trip policy mismatch: %+v", back)
	}
}

func TestYAMLRoundTrip(t *testing.T) {
	m := validManifest()
	raw, err := goyaml.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	back, err := LoadYAML(raw, eventsRegistry(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if back.Name != m.Name || back.Events.Emits[0].Type != "sms.sent" {
		t.Fatalf("yaml round-trip mismatch: %+v", back)
	}
	if back.BindingStrategy != m.BindingStrategy || back.Lifecycle != m.Lifecycle {
		t.Fatalf("yaml policy mismatch: %+v", back)
	}
}

func TestBindingAndLifecycleDefaultsAllowEmpty(t *testing.T) {
	m := Manifest{Name: "sms.twilio", Version: "1.0.0", Runtime: RuntimeGoja}
	if err := m.Validate(nil); err != nil {
		t.Fatalf("empty optional policy fields should validate: %v", err)
	}
}

func TestUnknownKeyRejectedWithSuggestion(t *testing.T) {
	for _, tc := range []struct {
		json string
		key  string
	}{
		{`{"name":"sms.twilio","version":"1.0.0","runtime":"goja","provdes":["sms.send"]}`, "provdes"},
		{`{"name":"sms.twilio","version":"1.0.0","runtime":"goja","versoin":"1.0.0"}`, "versoin"},
	} {
		_, err := LoadJSON([]byte(tc.json), nil)
		var inv *ErrManifestInvalid
		if !errors.As(err, &inv) {
			t.Fatalf("%s: want *ErrManifestInvalid, got %T: %v", tc.key, err, err)
		}
		if inv.Path != tc.key {
			t.Fatalf("path = %q, want %q", inv.Path, tc.key)
		}
		if inv.Suggestion == "" {
			t.Fatalf("%s: expected a suggestion", tc.key)
		}
	}
}

func TestUnknownKeyYAMLRejected(t *testing.T) {
	_, err := LoadYAML([]byte("name: sms.twilio\nversion: 1.0.0\nruntime: goja\nprovdes:\n  - sms.send\n"), nil)
	var inv *ErrManifestInvalid
	if !errors.As(err, &inv) {
		t.Fatalf("want *ErrManifestInvalid, got %T: %v", err, err)
	}
}
