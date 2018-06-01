package network

import (
	"context"
	"errors"
	"strconv"
)

var (
	ErrorNothingToRun = errors.New("nothing to run")
	ErrorNoConditions = errors.New("no condtitions")
)

type Executor interface {
	Execute(context.Context, []string) error
}

type ComcastExecutor struct {
	Shell Executor
}

func (e ComcastExecutor) Start(ctx context.Context, options ...Options) error {
	if len(options) == 0 {
		return ErrorNothingToRun
	}
	for _, opt := range options {
		if err := e.StartSingle(ctx, opt); err != nil {
			return err
		}
	}
	return nil
}

func (e ComcastExecutor) StartSingle(ctx context.Context, opt Options) error {
	cmd := []string{"comcast"}
	if opt.Latency != 0 {
		cmd = append(cmd, "-latency", strconv.Itoa(opt.Latency))
	}
	if opt.PacketLoss != 0 {
		cmd = append(cmd, "-packet-loss", strconv.Itoa(opt.PacketLoss))
	}
	if len(opt.TargetAddr) != 0 {
		cmd = append(cmd, "-target-addr", opt.TargetAddr)
	}
	if len(cmd) == 1 {
		return ErrorNoConditions
	}
	return e.Shell.Execute(ctx, cmd)
}

func (e ComcastExecutor) Stop(ctx context.Context, options ...Options) error {
	return e.Shell.Execute(ctx, []string{"comcast", "-stop"})
}

type Options struct {
	TargetInterface string // comcast selects eth0 by default
	TargetAddr      string // all addresses will be blocked by default
	Latency         int    // milliseconds
	PacketLoss      int    // percents
}
