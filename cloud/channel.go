package cloud

import (
	"time"

	"github.com/asenawritescode/kora/contract"
)

// ChannelRouteState tracks the lifecycle of a routed channel conversation.
type ChannelRouteState string

const (
	ChannelRouteRequested ChannelRouteState = "requested"
	ChannelRouteBound     ChannelRouteState = "bound"
	ChannelRouteRunning   ChannelRouteState = "running"
	ChannelRoutePaused    ChannelRouteState = "paused"
	ChannelRouteCompleted ChannelRouteState = "completed"
	ChannelRouteFailed    ChannelRouteState = "failed"
)

// ChannelRoute binds an inbound channel conversation to an engine run.
type ChannelRoute struct {
	ID              string                `json:"id"`
	Site            string                `json:"site"`
	Channel         string                `json:"channel"`
	ConversationKey string                `json:"conversation_key"`
	SenderAddress   string                `json:"sender_address"`
	SessionID       string                `json:"session_id"`
	RunID           string                `json:"run_id"`
	Actor           contract.ActorContext `json:"actor"`
	State           ChannelRouteState     `json:"state"`
	EngineType      string                `json:"engine_type"`
	EngineVersion   int                   `json:"engine_version"`
	CreatedAt       time.Time             `json:"created_at"`
	UpdatedAt       time.Time             `json:"updated_at,omitempty"`
	LastError       string                `json:"last_error,omitempty"`
}

// ChannelRunSubmission is the engine-facing envelope Cloud emits rather than
// executing a separate business path.
type ChannelRunSubmission struct {
	RouteID         string                   `json:"route_id"`
	RunID           string                   `json:"run_id"`
	Site            string                   `json:"site"`
	Channel         string                   `json:"channel"`
	ConversationKey string                   `json:"conversation_key"`
	SenderAddress   string                   `json:"sender_address"`
	Actor           contract.ActorContext    `json:"actor"`
	Command         contract.CommandEnvelope `json:"command"`
	CreatedAt       time.Time                `json:"created_at"`
}

// ChannelRouteCanTransition validates lifecycle movement.
func ChannelRouteCanTransition(from, to ChannelRouteState) bool {
	switch from {
	case ChannelRouteRequested:
		return to == ChannelRouteBound || to == ChannelRouteFailed
	case ChannelRouteBound:
		return to == ChannelRouteRunning || to == ChannelRoutePaused || to == ChannelRouteFailed
	case ChannelRouteRunning:
		return to == ChannelRoutePaused || to == ChannelRouteCompleted || to == ChannelRouteFailed
	case ChannelRoutePaused:
		return to == ChannelRouteRunning || to == ChannelRouteCompleted || to == ChannelRouteFailed
	case ChannelRouteCompleted, ChannelRouteFailed:
		return false
	default:
		return false
	}
}

// ChannelRouteSubmissionIsEngineBound reports whether the submission carries a
// canonical engine command envelope and resolved actor identity.
func ChannelRouteSubmissionIsEngineBound(sub ChannelRunSubmission) bool {
	return sub.Site != "" &&
		sub.Channel != "" &&
		sub.ConversationKey != "" &&
		sub.SenderAddress != "" &&
		sub.Actor.Authenticated() &&
		sub.Command.Type == "agent.run" &&
		sub.Command.Site == sub.Site
}

// ChannelRoutePreservesIdentity reports whether a route still matches the
// delegated actor and channel session it was issued for.
func ChannelRoutePreservesIdentity(route ChannelRoute, actor contract.ActorContext, sessionID string) bool {
	return route.Site == actor.Site &&
		route.Channel == actor.Channel &&
		route.SessionID == sessionID &&
		route.Actor.PrincipalID == actor.PrincipalID &&
		route.Actor.PrincipalType == actor.PrincipalType &&
		route.Actor.SubjectUserID == actor.SubjectUserID &&
		route.Actor.AuthSessionID == actor.AuthSessionID
}
