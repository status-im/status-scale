package network

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

var (
	ErrNothingToRun = errors.New("nothing to run")
)

type executor func(context.Context, []string) error

func ComcastStart(shell executor, ctx context.Context, opt Options) error {
	return ComcastStartSingle(shell, ctx, opt)
}

func ComcastStartSingle(shell executor, ctx context.Context, opt Options) error {
	cmd := []string{"comcast"}
	if opt.Latency != 0 {
		cmd = append(cmd, "-latency", strconv.Itoa(opt.Latency))
	}
	if opt.PacketLoss != 0 {
		cmd = append(cmd, "-packet-loss", strconv.Itoa(opt.PacketLoss))
	}
	if len(opt.TargetAddrs) != 0 {
		cmd = append(cmd, "-target-addr", strings.Join(opt.TargetAddrs, ","))
	}
	if len(cmd) == 1 {
		return ErrNothingToRun
	}
	return shell(ctx, cmd)
}

func ComcastStop(shell executor, ctx context.Context, options ...Options) error {
	return shell(ctx, []string{"comcast", "-stop"})
}

type Options struct {
	TargetInterface string   // comcast selects eth0 by default
	TargetAddrs     []string // all addresses will be blocked by default
	Latency         int      // milliseconds
	PacketLoss      int      // percents
}
