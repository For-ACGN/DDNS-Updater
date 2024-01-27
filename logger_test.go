package ddns

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogger(t *testing.T) {
	logger, err := newLogger("testdata/testdata.log")
	require.NoError(t, err)
	defer func() {
		err = os.Remove("testdata/testdata.log")
		require.NoError(t, err)
	}()

	logger.Info("info")
	logger.Infof("infof: %s", "info")

	logger.Warning("warning")
	logger.Warningf("warningf: %s", "warning")

	logger.Error("error")
	logger.Errorf("errorf: %s", "error")

	logger.Fatal("func", "fatal")
	logger.Fatalf("func", "fatalf: %s", "fatal")

	err = logger.Close()
	require.NoError(t, err)
}
