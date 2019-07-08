package utils

import "context"
import "time"

func PollImmediate(parent context.Context, f func(context.Context) error, period, timeout time.Duration) (err error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	err = f(ctx)
	if err != nil {
		return err
	}
	tick := time.NewTicker(period)
	after := time.NewTimer(timeout)
	defer tick.Stop()
	defer after.Stop()
	for {
		select {
		case <-parent.Done():
			cancel()
			return nil
		case <-tick.C:
			err = f(ctx)
			if err != nil {
				return err
			}
		case <-after.C:
			cancel()
			return nil
		}
	}
	return nil
}

func PollImmediateNoError(parent context.Context, f func(context.Context) error, period, timeout time.Duration) (err error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	err = f(ctx)
	if err == nil {
		return nil
	}
	tick := time.NewTicker(period)
	after := time.NewTimer(timeout)
	defer tick.Stop()
	defer after.Stop()
	for {
		select {
		case <-tick.C:
			err = f(ctx)
			if err == nil {
				return nil
			}
		case <-after.C:
			cancel()
			return err
		}
	}
	return nil
}
