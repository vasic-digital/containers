package boot_test

import (
	"testing"
	"time"

	"digital.vasic.containers/pkg/boot"

	"github.com/stretchr/testify/assert"
)

func TestBootResult_Fields(t *testing.T) {
	r := &boot.BootResult{
		Name:     "redis",
		Status:   "started",
		Duration: 500 * time.Millisecond,
		Error:    nil,
	}

	assert.Equal(t, "redis", r.Name)
	assert.Equal(t, "started", r.Status)
	assert.Equal(t, 500*time.Millisecond, r.Duration)
	assert.Nil(t, r.Error)
}

func TestBootSummary_String(t *testing.T) {
	s := &boot.BootSummary{
		Started:       3,
		Remote:        1,
		Discovered:    2,
		Failed:        0,
		Skipped:       1,
		TotalDuration: 1234 * time.Millisecond,
	}

	str := s.String()
	assert.Contains(t, str, "3 started")
	assert.Contains(t, str, "1 remote")
	assert.Contains(t, str, "2 discovered")
	assert.Contains(t, str, "0 failed")
	assert.Contains(t, str, "1 skipped")
	assert.Contains(t, str, "1.234s")
}

func TestBootSummary_HasFailures(t *testing.T) {
	tests := []struct {
		name   string
		failed int
		want   bool
	}{
		{"no failures", 0, false},
		{"one failure", 1, true},
		{"many failures", 5, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &boot.BootSummary{Failed: tc.failed}
			assert.Equal(t, tc.want, s.HasFailures())
		})
	}
}

func TestBootSummary_Empty(t *testing.T) {
	s := &boot.BootSummary{}
	assert.False(t, s.HasFailures())
	assert.Contains(t, s.String(), "0 started")
}
