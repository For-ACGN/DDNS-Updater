package ddns

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	l, err := newLogger("testdata/testdata.log")
	require.NoError(t, err)

	l.Info("info")
	l.Infof("infof: %s", "info")

	l.Warning("warning")
	l.Warningf("warningf: %s", "warning")

	l.Error("error")
	l.Errorf("errorf: %s", "error")

	l.Fatal("func", "fatal")
	l.Fatalf("func", "fatalf: %s", "fatal")

	err = l.Close()
	require.NoError(t, err)

	err = os.Remove("testdata/testdata.log")
	require.NoError(t, err)
}
