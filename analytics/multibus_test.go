package analytics

import (
	"testing"
	"time"
)

type multiBusStub struct {
	ch chan ChangeEvent
}

func (s *multiBusStub) Publish(event ChangeEvent) error { return nil }
func (s *multiBusStub) Subscribe() (<-chan ChangeEvent, error) {
	s.ch = make(chan ChangeEvent, 1)
	return s.ch, nil
}
func (s *multiBusStub) DrainWAL(handler func(ChangeEvent)) (int, error) { return 0, nil }
func (s *multiBusStub) RotateWAL() (string, error)                        { return "", nil }
func (s *multiBusStub) CommitWALRotation(string) error                    { return nil }
func (s *multiBusStub) Dropped() int64                                   { return 0 }
func (s *multiBusStub) Close() error                                     { return nil }

func TestMultiBusDroppedCountsSlowListeners(t *testing.T) {
	inner := &multiBusStub{}
	mb, err := NewMultiBus(inner)
	if err != nil {
		t.Fatalf("NewMultiBus: %v", err)
	}
	defer mb.Close()

	listener := make(chan ChangeEvent)
	mb.AddListener(listener)

	inner.ch <- ChangeEvent{Site: "test", Doctype: "Customer", Timestamp: time.Now().UTC()}

	time.Sleep(10 * time.Millisecond)

	if got := mb.Dropped(); got == 0 {
		t.Fatalf("Dropped() = %d, want > 0 when listener is slow", got)
	}
}
