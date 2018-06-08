package tests

import (
	"time"

	"github.com/stretchr/testify/require"
)

func Eventually(t require.TestingT, f func() error, period, interval time.Duration) {
	var err error
	for start := time.Now(); time.Since(start) < period; {
		err = f()
		if err != nil {
			time.Sleep(interval)
			continue
		}
		return
	}
	t.Errorf(err.Error())
}

func Consistently(t require.TestingT, f func() error, period, interval time.Duration) {

}
