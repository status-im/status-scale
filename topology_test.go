package scale

import (
	"flag"
)

var (
	central = flag.Int("central", 3, "central peers number.")
	leaf    = flag.Int("leaf", 5, "leaf peers number")
)
