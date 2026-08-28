package effect

import (
	"errors"
	"testing"
	"time"

	"github.com/asenawritescode/kora/contract"
)

func ctx() contract.OperationContext {
	return contract.OperationContext{Site: "site-a", Actor: "u1", TraceID: "t1", Deadline: time.Now().Add(time.Minute)}
}

func TestScopeCloseDisposesAllEffectsLIFO(t *testing.T) {
	s := OpenScope(ctx(), "sms")
	var order []string
	_ = s.Track(Effect{Kind: EffectTimer, Disposer: func() error { order = append(order, "timer"); return nil }})
	_ = s.Track(Effect{Kind: EffectSubscription, Disposer: func() error { order = append(order, "subscription"); return nil }})

	ev := s.Close(CloseNormal)
	if ev.Effects != 2 || ev.Disposed != 2 || ev.Failed != 0 {
		t.Fatalf("evidence = %+v", ev)
	}
	if len(order) != 2 || order[0] != "subscription" || order[1] != "timer" {
		t.Fatalf("LIFO order violated: %v", order)
	}
}

func TestDisposerFailureContinues(t *testing.T) {
	s := OpenScope(ctx(), "sms")
	boom := errors.New("boom")
	var disposed int
	_ = s.Track(Effect{Kind: EffectTimer, Disposer: func() error { disposed++; return boom }})
	_ = s.Track(Effect{Kind: EffectSubscription, Disposer: func() error { disposed++; return nil }})

	ev := s.Close(CloseNormal)
	if ev.Failed != 1 || ev.Disposed != 1 || ev.Effects != 2 {
		t.Fatalf("evidence = %+v", ev)
	}
	if disposed != 2 {
		t.Fatalf("failing disposer skipped the rest: disposed=%d", disposed)
	}
	failures := s.Failures()
	if len(failures) != 1 || failures[0].Kind != EffectTimer || !errors.Is(failures[0].Cause, boom) {
		t.Fatalf("failure not recorded: %+v", failures)
	}
}

func TestTrackRejectsNilDisposer(t *testing.T) {
	s := OpenScope(ctx(), "sms")
	if err := s.Track(Effect{Kind: EffectTimer}); err == nil {
		t.Fatalf("expected nil-disposer rejection")
	}
	if err := s.Track(Effect{Kind: EffectTimer, Disposer: func() error { return nil }}); err != nil {
		t.Fatalf("valid effect rejected: %v", err)
	}
}

func TestCloseIdempotent(t *testing.T) {
	s := OpenScope(ctx(), "sms")
	_ = s.Track(Effect{Kind: EffectTimer, Disposer: func() error { return nil }})
	first := s.Close(CloseNormal)
	if first.Effects != 1 || first.Disposed != 1 {
		t.Fatalf("first close = %+v", first)
	}
	second := s.Close(CloseNormal)
	if second.Effects != 0 || second.Disposed != 0 {
		t.Fatalf("second close should be a no-op: %+v", second)
	}
}

func TestTrackAfterCloseRejected(t *testing.T) {
	s := OpenScope(ctx(), "sms")
	_ = s.Close(CloseNormal)
	if err := s.Track(Effect{Kind: EffectTimer, Disposer: func() error { return nil }}); err == nil {
		t.Fatalf("expected rejection tracking into a closed scope")
	}
}

func TestEvidenceCarriesOwnerAndReason(t *testing.T) {
	s := OpenScope(ctx(), "component-x")
	_ = s.Track(Effect{Kind: EffectTimer, Disposer: func() error { return nil }})
	ev := s.Close(CloseShutdown)
	if ev.Owner != "component-x" || ev.Reason != CloseShutdown {
		t.Fatalf("evidence missing owner/reason: %+v", ev)
	}
	if ev.Duration < 0 {
		t.Fatalf("negative duration: %+v", ev)
	}
}
