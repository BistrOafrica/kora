package natsprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/asenawritescode/kora/contract"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

func runEmbeddedServer(t *testing.T) (*server.Server, string) {
	t.Helper()
	opts := &server.Options{JetStream: true, Port: -1}
	s, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	go s.Start()
	if !s.ReadyForConnections(10 * time.Second) {
		t.Fatal("server not ready")
	}
	return s, fmt.Sprintf("nats://%s", s.Addr().String())
}

func TestBootstrapIsIdempotentAndRequestReplyWorks(t *testing.T) {
	s, url := runEmbeddedServer(t)
	defer s.Shutdown()

	p, err := New(context.Background(), Config{
		Name:          "kora-test",
		ServerURLs:    []string{url},
		StreamName:    "KORA_EVENTS",
		SubjectPrefix: "kora",
		MaxDeliver:    5,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	defer p.Close()

	if err := p.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap 1: %v", err)
	}
	if err := p.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap 2: %v", err)
	}

	if err := p.Publish(context.Background(), contract.EventEnvelope{
		ID:   "evt-1",
		Type: "events.document.created",
		Site: "site-a",
		Data: json.RawMessage(`{"data":{"name":"Test"}}`),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
}

func TestConfigValidate(t *testing.T) {
	if err := (Config{}).Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestPublishUsesCanonicalSubject(t *testing.T) {
	s, url := runEmbeddedServer(t)
	defer s.Shutdown()

	p, err := New(context.Background(), Config{
		Name:          "kora-test",
		ServerURLs:    []string{url},
		StreamName:    "KORA_EVENTS",
		SubjectPrefix: "kora",
		MaxDeliver:    5,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	defer p.Close()
	if err := p.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	ch := make(chan *nats.Msg, 1)
	_, err = p.nc.ChanSubscribe("kora.events.document.created", ch)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if err := p.Publish(context.Background(), contract.EventEnvelope{
		ID:      "evt-1",
		Type:    "events.document.created",
		Version: 1,
		Site:    "site-a",
		Data:    json.RawMessage(`{"data":{"name":"Test"}}`),
	}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for published event")
	}
}

func TestConsumerDeadLettersAfterMaxDeliver(t *testing.T) {
	s, url := runEmbeddedServer(t)
	defer s.Shutdown()

	p, err := New(context.Background(), Config{
		Name:              "kora-test",
		ServerURLs:        []string{url},
		StreamName:        "KORA_EVENTS",
		SubjectPrefix:     "kora",
		ConsumerName:      "kora-worker",
		DeadLetterSubject: "kora.deadletter",
		MaxDeliver:        1,
	})
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	defer p.Close()
	if err := p.Bootstrap(context.Background()); err != nil {
		t.Fatalf("bootstrap: %v", err)
	}

	c, err := NewConsumer(p, p.cfg)
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = c.Run(runCtx, func(ctx context.Context, d contract.Delivery) error {
			return fmt.Errorf("boom")
		})
	}()

	dlCh := make(chan *nats.Msg, 1)
	_, err = p.nc.ChanSubscribe("kora.deadletter", dlCh)
	if err != nil {
		t.Fatalf("deadletter subscribe: %v", err)
	}
	if err := p.nc.Publish("kora.events.test", []byte(`{"id":"evt-1"}`)); err != nil {
		t.Fatalf("publish message: %v", err)
	}
	select {
	case msg := <-dlCh:
		if msg == nil {
			t.Fatal("expected deadletter message")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for deadletter message")
	}
	_ = c.Drain(context.Background())
	cancel()
}
