package cloud

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/asenawritescode/kora/contract"
)

func TestDelegatedIdentityJSONShape(t *testing.T) {
	actor := contract.ActorContext{
		PrincipalID:     "user-1",
		PrincipalType:   contract.PrincipalHuman,
		SubjectUserID:   "user-1",
		OrganizationID:  "org-1",
		Site:            "acme.example.com",
		Channel:         "whatsapp",
		DeviceID:        "device-1",
		AuthenticatedAt: time.Date(2026, 8, 13, 13, 30, 0, 0, time.UTC),
		AuthSessionID:   "sess-1",
	}
	identity := DelegatedIdentity{
		ID:             "di-1",
		Site:           actor.Site,
		Channel:        actor.Channel,
		ChannelAccount: "wa:+254700000001",
		Actor:          actor,
		State:          DelegatedIdentityRequested,
		IdentityRef:    "idref-1",
		DelegationRef:  "deleg-1",
		BoundAt:        time.Date(2026, 8, 13, 13, 31, 0, 0, time.UTC),
		ExpiresAt:      time.Date(2026, 8, 13, 14, 31, 0, 0, time.UTC),
	}
	rule := ChannelRoutingRule{
		ID:            "rule-1",
		Site:          actor.Site,
		Channel:       actor.Channel,
		Actor:         actor,
		DelegatedID:   identity.ID,
		EngineSubject: "kora.commands.org-1",
		CreatedAt:     identity.BoundAt,
	}

	for _, v := range []struct {
		val  any
		keys []string
	}{
		{identity, []string{"id", "site", "channel", "channel_account", "actor", "state", "identity_ref", "delegation_ref", "bound_at", "expires_at"}},
		{rule, []string{"id", "site", "channel", "actor", "delegated_id", "engine_subject", "created_at"}},
	} {
		b, err := json.Marshal(v.val)
		if err != nil {
			t.Fatalf("marshal %T: %v", v.val, err)
		}
		for _, key := range v.keys {
			if !strings.Contains(string(b), `"`+key+`"`) {
				t.Fatalf("%T missing key %q: %s", v.val, key, b)
			}
		}
	}
}

func TestDelegatedIdentityRoutingHelpers(t *testing.T) {
	actor := contract.ActorContext{
		PrincipalID:     "user-1",
		PrincipalType:   contract.PrincipalHuman,
		Site:            "acme.example.com",
		Channel:         "whatsapp",
		AuthenticatedAt: time.Date(2026, 8, 13, 13, 30, 0, 0, time.UTC),
		AuthSessionID:   "sess-1",
	}
	identity := DelegatedIdentity{
		ID:         "di-1",
		Site:       actor.Site,
		Channel:    actor.Channel,
		Actor:      actor,
		State:      DelegatedIdentityVerified,
		BoundAt:    time.Date(2026, 8, 13, 13, 31, 0, 0, time.UTC),
		VerifiedAt: time.Date(2026, 8, 13, 13, 32, 0, 0, time.UTC),
		ExpiresAt:  time.Date(2026, 8, 13, 14, 31, 0, 0, time.UTC),
	}
	rule := ChannelRoutingRule{
		ID:            "rule-1",
		Site:          actor.Site,
		Channel:       actor.Channel,
		Actor:         actor,
		DelegatedID:   identity.ID,
		EngineSubject: "kora.commands.org-1",
		CreatedAt:     identity.BoundAt,
	}

	if !DelegatedIdentityCanTransition(DelegatedIdentityRequested, DelegatedIdentityBound) {
		t.Fatal("requested identity should bind")
	}
	if DelegatedIdentityCanTransition(DelegatedIdentityVerified, DelegatedIdentityBound) {
		t.Fatal("verified identity should not move backwards")
	}
	if !DelegatedIdentityPreservesActor(rule, identity) {
		t.Fatal("routing rule should preserve the same actor")
	}
	if !DelegatedIdentityIsActive(identity, time.Date(2026, 8, 13, 13, 45, 0, 0, time.UTC)) {
		t.Fatal("verified unexpired identity should be active")
	}
	identity.State = DelegatedIdentityRevoked
	if DelegatedIdentityIsActive(identity, time.Date(2026, 8, 13, 13, 45, 0, 0, time.UTC)) {
		t.Fatal("revoked identity should not be active")
	}
}
