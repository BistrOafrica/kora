package auth

import "testing"

func TestProviderRegistryDefaults(t *testing.T) {
	r := NewProviderRegistry()
	providers := r.List()
	if len(providers) < 3 {
		t.Fatalf("provider registry list too short: %+v", providers)
	}
	if providers[0].Name == "" {
		t.Fatal("expected named providers")
	}
	foundOIDC := false
	for _, p := range providers {
		if p.Name == "oidc" {
			foundOIDC = true
			if p.Status != "planned" {
				t.Fatalf("oidc status = %q, want planned", p.Status)
			}
		}
	}
	if !foundOIDC {
		t.Fatal("expected oidc provider to be registered")
	}
}
