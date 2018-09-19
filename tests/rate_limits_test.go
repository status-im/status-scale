package tests

import (
	"context"
	"testing"
	"time"

	"github.com/status-im/status-go/params"
	"github.com/status-im/status-scale/cluster"
	"github.com/stretchr/testify/require"
)

// TestGeneralWhisperRateLimiting tests two things:
// 1st round users exchange messages without any spam
// 2nd round spammer generates messages and users do receive them
// how fast we disconnect, what would be % of usefull lost messages
func TestGeneralWhisperRateLimiting(t *testing.T) {
	userIngress := params.RateLimitConfig{
		Interval: uint64(time.Second),
		Capacity: 1 << 20,  // 1mb - surge of 35k envelopes per single connection
		Quantum:  29 << 10, // 29kb - 98 envelopes per second
	}
	userEgress := userIngress
	userEgress.Quantum *= 5
	relayIngress := params.RateLimitConfig{
		Interval: uint64(time.Second),
		Capacity: 50 << 20,  // 50mb, surge of ~170l envelopes per single connection
		Quantum:  292 << 10, // 1000 envelopes per second. 292kb
	}
	relayEgress := relayIngress
	relayEgress.Quantum *= 5
	c := ClusterFromConfig()
	require.NoError(t, c.Create(context.TODO(), cluster.ScaleOpts{
		Boot: 1, Relay: 4, Users: 10,
		RelayIngress: relayIngress,
		RelayEgress:  relayEgress,
		UserIngress:  userIngress,
		UserEgress:   userEgress,
		Deploy:       true}))
	defer c.Clean(context.TODO())
}
