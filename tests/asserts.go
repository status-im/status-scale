package tests

import (
	"time"

	"github.com/stretchr/testify/require"
)

func Eventually(t require.TestingT, f func() error, period, interval time.Duration) {
	var err error
	for start := time.Now(); time.Since(start) < period; {
		err = f()
		if err == nil {
			return
		}
		time.Sleep(interval)
	}
	t.Errorf(err.Error())
	t.FailNow()
}

func Consistently(t require.TestingT, f func() error, period, interval time.Duration) {
	for start := time.Now(); time.Since(start) < period; {
		if err := f(); err != nil {
			t.Errorf(err.Error())
			t.FailNow()
			return
		}
		time.Sleep(interval)
	}
}
