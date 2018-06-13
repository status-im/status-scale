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

	tab := NewTab()
	tab.AddColumns(UIDColumn{"name"})
	tab.AddColumns(P2PColumns()...)
	require.NoError(t, tab.Append("test_1", data))
	require.NoError(t, tab.Append("test_2", data))
	ToASCII(tab, os.Stdout).Render()
}
