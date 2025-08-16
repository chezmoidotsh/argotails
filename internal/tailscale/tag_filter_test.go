package tsutils_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"tailscale.com/client/tailscale/v2"

	tsutils "github.com/chezmoidotsh/argotails/internal/tailscale"
)

func TestNewRegexpTagFilter_Error(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		err      string
	}{
		{
			name:     "InvalidPattern",
			patterns: []string{"^tag1$", "invalid[pattern"},
			err:      "invalid tag pattern: error parsing regexp: missing closing ]: `[pattern`",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := tsutils.NewRegexpTagFilter(tt.patterns...)
			assert.Nil(t, filter)
			assert.EqualError(t, err, tt.err)
		})
	}
}

func TestNewRegexpTagFilter_Match(t *testing.T) {
	devices := []tailscale.Device{
		{Tags: []string{"tag:tag1"}},
		{Tags: []string{"tag:tag1"}},
		{Tags: []string{"tag:tag2"}},
		{Tags: []string{"tag:tag3"}},
		{Tags: []string{"tag:tagA"}},
	}

	tests := []struct {
		name     string
		patterns []string
		expected []bool
	}{
		{
			name:     "MatchAll",
			patterns: []string{},
			expected: []bool{true, true, true, true, true},
		},
		{
			name:     "MatchTag1",
			patterns: []string{"^tag1$"},
			expected: []bool{true, true, false, false, false},
		},
		{
			name:     "MatchTagNumber",
			patterns: []string{"^tag[0-9]$"},
			expected: []bool{true, true, true, true, false},
		},
		{
			name:     "MatchTag1AndTag3",
			patterns: []string{"^tag1$", "^tag3$"},
			expected: []bool{true, true, false, true, false},
		},
		{
			name:     "NoMatch",
			patterns: []string{"^tagX$"},
			expected: []bool{false, false, false, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := tsutils.NewRegexpTagFilter(tt.patterns...)
			assert.NoError(t, err)
			assert.NotNil(t, filter)

			actual := make([]bool, len(devices))
			for i, device := range devices {
				actual[i] = filter.Match(device)
			}
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestFuncTagFilter_Match(t *testing.T) {
	devices := []tailscale.Device{
		{Tags: []string{"tag:tag1"}},
		{Tags: []string{"tag:tag1"}},
		{Tags: []string{"tag:tag2"}},
		{Tags: []string{"tag:tag3"}},
		{Tags: []string{"tag:tagA"}},
	}

	tests := []struct {
		name     string
		fn       tsutils.FuncTagFilter
		expected []bool
	}{
		{
			name: "MatchAll",
			fn: tsutils.FuncTagFilter(func(device tailscale.Device) bool {
				return true
			}),
			expected: []bool{true, true, true, true, true},
		},
		{
			name:     "MatchAll2",
			fn:       tsutils.FuncTagFilter(nil),
			expected: []bool{false, false, false, false, false},
		},
		{
			name: "MatchTag1",
			fn: tsutils.FuncTagFilter(func(device tailscale.Device) bool {
				return strings.Contains(strings.Join(device.Tags, ","), "tag1")
			}),
			expected: []bool{true, true, false, false, false},
		},
		{
			name: "NoMatch",
			fn: tsutils.FuncTagFilter(func(device tailscale.Device) bool {
				return false
			}),
			expected: []bool{false, false, false, false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := make([]bool, len(devices))
			for i, device := range devices {
				actual[i] = tt.fn.Match(device)
			}
			assert.Equal(t, tt.expected, actual)
		})
	}
}
