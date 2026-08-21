package auth

import "sort"

// AuthProvider describes a sign-in provider exposed by the runtime.
type AuthProvider struct {
	Name        string `json:"name"`
	Label       string `json:"label"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
}

// ProviderRegistry is the normalized source of auth provider discovery.
type ProviderRegistry struct {
	providers map[string]AuthProvider
}

func NewProviderRegistry() *ProviderRegistry {
	r := &ProviderRegistry{providers: make(map[string]AuthProvider)}
	r.Register(AuthProvider{
		Name:   "password",
		Label:  "Email & Password",
		Status: "supported",
	})
	r.Register(AuthProvider{
		Name:        "magic_link",
		Label:       "Magic Link",
		Status:      "supported",
		Description: "Passwordless email sign-in",
	})
	r.Register(AuthProvider{
		Name:        "oidc",
		Label:       "OpenID Connect",
		Status:      "planned",
		Description: "OIDC authorization-code + PKCE",
	})
	return r
}

func (r *ProviderRegistry) Register(provider AuthProvider) {
	if r.providers == nil {
		r.providers = make(map[string]AuthProvider)
	}
	r.providers[provider.Name] = provider
}

func (r *ProviderRegistry) List() []AuthProvider {
	out := make([]AuthProvider, 0, len(r.providers))
	for _, p := range r.providers {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
