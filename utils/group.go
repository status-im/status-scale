package utils

import (
	"bytes"
	"context"
	"errors"
	"sync"
)

func NewGroup(ctx context.Context, n int) *Group {
	r := &Group{}
	r.wg.Add(n)
	r.errors = make(chan error, n)
	r.ctx, r.cancel = context.WithCancel(ctx)
	return r
}

type Group struct {
	ctx    context.Context
	cancel func()
	wg     sync.WaitGroup
	errors chan error
}

func (r *Group) Run(f func(ctx context.Context) error) {
	go func() {
		r.errors <- f(r.ctx)
		r.wg.Done()
	}()
}

func (r *Group) Error() error {
	r.wg.Wait()
	close(r.errors)
	var b bytes.Buffer
	for err := range r.errors {
		if err != nil {
			b.WriteString(err.Error())
			b.WriteString("\n")
		}
	}
	if len(b.String()) != 0 {
		return errors.New(b.String())
	}
	return nil
}

func (r *Group) Stop() {
	r.cancel()
}
