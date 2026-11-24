//go:build linux

package hostbudget

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The RLIMIT_NPROC constant is easy to get wrong (6 on linux; 8 is
// RLIMIT_MEMLOCK). Cross-check the live value against /proc/self/limits.
func TestRlimitNproc_MatchesProcSelfLimits(t *testing.T) {
	b, err := os.ReadFile("/proc/self/limits")
	require.NoError(t, err)
	var want string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "Max processes") {
			want = strings.Fields(strings.TrimPrefix(l, "Max processes"))[0]
		}
	}
	require.NotEmpty(t, want)
	lim, limited, err := rlimitNproc()
	require.NoError(t, err)
	if want == "unlimited" {
		require.False(t, limited)
		return
	}
	require.True(t, limited)
	require.Equal(t, want, strconv.FormatUint(lim, 10))
}
