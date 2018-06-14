package metrics

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetricsTableDebug(t *testing.T) {
	wd, err := os.Getwd()
	require.NoError(t, err)
	fp := filepath.Join(wd, "debug.json")
	f, err := os.Open(fp)
	require.NoError(t, err)
	defer f.Close()
	data, err := ioutil.ReadAll(f)
	require.NoError(t, err)

	tab := NewCompleteTab("name", P2PColumns(), DiscoveryColumns())
	require.NoError(t, tab.Append("test_1", data))
	require.NoError(t, tab.Append("test_2", data))
	ToASCII(tab, os.Stdout).Render()
}
