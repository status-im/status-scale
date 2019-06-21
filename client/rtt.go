package client

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/montanaflynn/stats"
	"github.com/status-im/status-console-client/protocol/gethservice"
	"github.com/status-im/status-scale/cluster"
)

func NewRTTMeter(chat gethservice.Contact, sender, receiver *cluster.Client) *RTTMeter {
	return &RTTMeter{
		chat:     chat,
		sender:   sender,
		receiver: receiver,
	}
}

type RTTMeter struct {
	chat gethservice.Contact

	sender, receiver *cluster.Client
	samples          []float64
}

func (m *RTTMeter) MeterSequantially(count int) error {
	for i := 0; i < count; i++ {
		err := m.meter(i)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m RTTMeter) Percentile(percent float64) float64 {
	rst, err := stats.Percentile(m.samples, percent)
	if err != nil {
		return 0
	}
	return rst
}

func (m *RTTMeter) send(i int) (time.Time, error) {
	tick := time.Tick(20 * time.Millisecond)
	after := time.After(10 * time.Minute)
	payload := fmt.Sprintf("hello receiver: %d", i)
	// TODO(dshulyak) add util PollImmediatly(func(context.Context) error, period, timeout time.Duration)
	for {
		select {
		case <-tick:
			sent := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := ChatClient(m.sender.Rpc()).Send(ctx, m.chat, payload)
			cancel()
			if err != nil {
				log.Debug("can't send msg", "payload", payload, "error", err)
				continue
			}
			return sent, nil
		case <-after:
			return time.Time{}, fmt.Errorf("failed to send a message %s", payload)
		}
	}
}

func (m *RTTMeter) receive(i int) error {
	tick := time.Tick(20 * time.Millisecond)
	after := time.After(10 * time.Minute)
	payload := fmt.Sprintf("hello receiver: %d", i)
	for {
		select {
		case <-tick:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			msgs, err := ChatClient(m.sender.Rpc()).Messages(ctx, m.chat, int64(i))
			cancel()
			if err != nil {
				return err
			}
			for _, msg := range msgs {
				if msg.Text == payload {
					return nil
				}
			}
		case <-after:
			return fmt.Errorf("failed waiting for a message with payload %s", payload)
		}
	}
}

func (m *RTTMeter) meter(i int) error {
	sent, err := m.send(i)
	if err != nil {
		return err
	}
	err = m.receive(i)
	if err != nil {
		return err
	}
	log.Debug("latency for msg", "i", i, "duration", time.Since(sent))
	m.samples = append(m.samples, time.Since(sent).Seconds())
	return nil
}
