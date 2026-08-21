package cloud

import (
	"time"

	"github.com/asenawritescode/kora/contract"
)

// DelegatedIdentityState tracks Cloud-to-engine delegation lifecycle.
type DelegatedIdentityState string

const (
	DelegatedIdentityRequested DelegatedIdentityState = "requested"
	DelegatedIdentityBound     DelegatedIdentityState = "bound"
	DelegatedIdentityVerified  DelegatedIdentityState = "verified"
	DelegatedIdentityRevoked   DelegatedIdentityState = "revoked"
	DelegatedIdentityExpired   DelegatedIdentityState = "expired"
)

// DelegatedIdentity binds a verified channel identity to an engine actor.
type DelegatedIdentity struct {
	ID             string                 `json:"id"`
	Site           string                 `json:"site"`
	Channel        string                 `json:"channel"`
	ChannelAccount string                 `json:"channel_account"`
	Actor          contract.ActorContext  `json:"actor"`
	State          DelegatedIdentityState `json:"state"`
	IdentityRef    string                 `json:"identity_ref,omitempty"`
	DelegationRef  string                 `json:"delegation_ref,omitempty"`
	BoundAt        time.Time              `json:"bound_at"`
	VerifiedAt     time.Time              `json:"verified_at,omitempty"`
	RevokedAt      time.Time              `json:"revoked_at,omitempty"`
	ExpiresAt      time.Time              `json:"expires_at,omitempty"`
	LastError      string                 `json:"last_error,omitempty"`
}

// ChannelRoutingRule describes how Cloud forwards a verified identity to the
// engine without inventing a second authorization model.
type ChannelRoutingRule struct {
	ID            string                `json:"id"`
	Site          string                `json:"site"`
	Channel       string                `json:"channel"`
	Actor         contract.ActorContext `json:"actor"`
	DelegatedID   string                `json:"delegated_id"`
	EngineSubject string                `json:"engine_subject"`
	CreatedAt     time.Time             `json:"created_at"`
	UpdatedAt     time.Time             `json:"updated_at,omitempty"`
}

// DelegatedIdentityCanTransition validates identity lifecycle changes.
func DelegatedIdentityCanTransition(from, to DelegatedIdentityState) bool {
	switch from {
	case DelegatedIdentityRequested:
		return to == DelegatedIdentityBound || to == DelegatedIdentityRevoked || to == DelegatedIdentityExpired
	case DelegatedIdentityBound:
		return to == DelegatedIdentityVerified || to == DelegatedIdentityRevoked || to == DelegatedIdentityExpired
	case DelegatedIdentityVerified:
		return to == DelegatedIdentityRevoked || to == DelegatedIdentityExpired
	case DelegatedIdentityRevoked, DelegatedIdentityExpired:
		return false
	default:
		return false
	}
}

// DelegatedIdentityPreservesActor reports whether the forwarded routing rule
// still carries the same resolved identity.
func DelegatedIdentityPreservesActor(rule ChannelRoutingRule, identity DelegatedIdentity) bool {
	return rule.Site == identity.Site &&
		rule.Channel == identity.Channel &&
		rule.DelegatedID == identity.ID &&
		rule.Actor.PrincipalID == identity.Actor.PrincipalID &&
		rule.Actor.PrincipalType == identity.Actor.PrincipalType &&
		rule.Actor.SubjectUserID == identity.Actor.SubjectUserID &&
		rule.Actor.AuthSessionID == identity.Actor.AuthSessionID
}

// DelegatedIdentityIsActive reports whether Cloud may route the identity to
// engine execution.
func DelegatedIdentityIsActive(identity DelegatedIdentity, now time.Time) bool {
	if identity.State != DelegatedIdentityVerified {
		return false
	}
	if identity.ExpiresAt.IsZero() || !now.Before(identity.ExpiresAt) {
		return false
	}
	return identity.Actor.Authenticated() && identity.Channel != "" && identity.Site != ""
}
