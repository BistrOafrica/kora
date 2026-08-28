package natsprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/asenawritescode/kora/contract"
	"github.com/nats-io/nats.go"
)

// Consumer provides durable pull-based delivery with dead-letter handling.
type Consumer struct {
	p    *Provider
	cfg  Config
	sub  *nats.Subscription
	seen chan struct{}
}

// NewConsumer creates a durable JetStream consumer against the provider stream.
func NewConsumer(p *Provider, cfg Config) (*Consumer, error) {
	if p == nil {
		return nil, fmt.Errorf("natsprovider: provider is nil")
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.MaxDeliver <= 0 {
		cfg.MaxDeliver = 5
	}
	return &Consumer{p: p, cfg: cfg, seen: make(chan struct{}, 1)}, nil
}

// Run consumes messages until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context, handler contract.Handler) error {
	consumerCfg := &nats.ConsumerConfig{
		Durable:       c.cfg.ConsumerName,
		AckPolicy:     nats.AckExplicitPolicy,
		DeliverPolicy: nats.DeliverAllPolicy,
		FilterSubject: c.cfg.SubjectPrefix + ".>",
		MaxDeliver:    c.cfg.MaxDeliver,
		AckWait:       500 * time.Millisecond,
	}
	_, err := c.p.js.AddConsumer(c.cfg.StreamName, consumerCfg, nats.Context(ctx))
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("natsprovider: add consumer: %w", err)
	}

	sub, err := c.p.js.PullSubscribe(c.cfg.SubjectPrefix+".>", c.cfg.ConsumerName, nats.BindStream(c.cfg.StreamName))
	if err != nil {
		return fmt.Errorf("natsprovider: pull subscribe: %w", err)
	}
	c.sub = sub

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		msgs, err := sub.Fetch(1, nats.MaxWait(250*time.Millisecond))
		if err != nil {
			if err == nats.ErrTimeout {
				continue
			}
			return err
		}
		for _, msg := range msgs {
			attempt := 1
			if md, metaErr := msg.Metadata(); metaErr == nil && md != nil {
				attempt = int(md.NumDelivered)
			}
			delivery := contract.Delivery{
				ID:      msg.Header.Get("Nats-Msg-Id"),
				Type:    msg.Subject,
				Site:    "",
				Data:    append([]byte(nil), msg.Data...),
				Attempt: attempt,
			}
			if err := handler(ctx, delivery); err != nil {
				if attempt >= c.cfg.MaxDeliver {
					_ = c.publishDeadLetter(ctx, delivery, err)
					_ = msg.Term()
					continue
				}
				_ = msg.Nak()
				continue
			}
			_ = msg.Ack()
		}
	}
}

// Drain stops delivery and drains the subscription if it exists.
func (c *Consumer) Drain(ctx context.Context) error {
	if c.sub == nil {
		return nil
	}
	done := make(chan error, 1)
	go func() { done <- c.sub.Drain() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Consumer) publishDeadLetter(ctx context.Context, delivery contract.Delivery, cause error) error {
	payload, _ := json.Marshal(map[string]any{
		"delivery": delivery,
		"error":    cause.Error(),
	})
	_, err := c.p.js.Publish(c.cfg.DeadLetterSubject, payload, nats.Context(ctx))
	return err
}
