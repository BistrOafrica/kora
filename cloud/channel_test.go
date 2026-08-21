package cloud

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/asenawritescode/kora/contract"
)

func TestChannelRouteJSONShape(t *testing.T) {
	actor := contract.ActorContext{
		PrincipalID:     "user-1",
		PrincipalType:   contract.PrincipalHuman,
		SubjectUserID:   "user-1",
		OrganizationID:  "org-1",
		Site:            "acme.example.com",
		Channel:         "whatsapp",
		AuthenticatedAt: time.Date(2026, 8, 13, 14, 0, 0, 0, time.UTC),
		AuthSessionID:   "sess-1",
	}
	route := ChannelRoute{
		ID:              "route-1",
		Site:            actor.Site,
		Channel:         actor.Channel,
		ConversationKey: "conv-1",
		SenderAddress:   "wa:+254700000001",
		SessionID:       "chs-1",
		RunID:           "run-1",
		Actor:           actor,
		State:           ChannelRouteRequested,
		EngineType:      "agent.run",
		EngineVersion:   1,
		CreatedAt:       time.Date(2026, 8, 13, 14, 1, 0, 0, time.UTC),
	}
	submission := ChannelRunSubmission{
		RouteID:         route.ID,
		RunID:           route.RunID,
		Site:            route.Site,
		Channel:         route.Channel,
		ConversationKey: route.ConversationKey,
		SenderAddress:   route.SenderAddress,
		Actor:           actor,
		Command: contract.CommandEnvelope{
			Type:     "agent.run",
			Site:     route.Site,
			Actor:    actor,
			Deadline: time.Date(2026, 8, 13, 14, 5, 0, 0, time.UTC),
		},
		CreatedAt: route.CreatedAt,
	}

	for _, v := range []struct {
		val  any
		keys []string
	}{
		{route, []string{"id", "site", "channel", "conversation_key", "sender_address", "session_id", "run_id", "actor", "state", "engine_type", "engine_version"}},
		{submission, []string{"route_id", "run_id", "site", "channel", "conversation_key", "sender_address", "actor", "command", "created_at"}},
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

func TestChannelRouteTransitionAndSubmissionHelpers(t *testing.T) {
	if !ChannelRouteCanTransition(ChannelRouteRequested, ChannelRouteBound) {
		t.Fatal("requested route should bind")
	}
	if ChannelRouteCanTransition(ChannelRouteCompleted, ChannelRouteRunning) {
		t.Fatal("completed route should not restart")
	}
	actor := contract.ActorContext{
		PrincipalID:     "user-1",
		PrincipalType:   contract.PrincipalHuman,
		Site:            "acme.example.com",
		Channel:         "whatsapp",
		AuthenticatedAt: time.Date(2026, 8, 13, 14, 0, 0, 0, time.UTC),
		AuthSessionID:   "sess-1",
	}
	route := ChannelRoute{
		ID:            "route-1",
		Site:          actor.Site,
		Channel:       actor.Channel,
		SessionID:     "chs-1",
		Actor:         actor,
		State:         ChannelRouteBound,
		EngineType:    "agent.run",
		EngineVersion: 1,
	}
	if !ChannelRoutePreservesIdentity(route, actor, "chs-1") {
		t.Fatal("route should preserve actor identity")
	}
	submission := ChannelRunSubmission{
		RouteID:         route.ID,
		RunID:           "run-1",
		Site:            route.Site,
		Channel:         route.Channel,
		ConversationKey: "conv-1",
		SenderAddress:   "wa:+254700000001",
		Actor:           actor,
		Command: contract.CommandEnvelope{
			Type:  "agent.run",
			Site:  route.Site,
			Actor: actor,
		},
	}
	if !ChannelRouteSubmissionIsEngineBound(submission) {
		t.Fatal("submission should be engine bound")
	}
	submission.Command.Type = "document.create"
	if ChannelRouteSubmissionIsEngineBound(submission) {
		t.Fatal("non-agent run command should not qualify")
	}
}
