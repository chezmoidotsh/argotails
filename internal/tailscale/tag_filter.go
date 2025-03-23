package tsutils

import (
	"fmt"
	"regexp"
	"strings"

	"tailscale.com/client/tailscale/v2"
)

type (
	TagFilter interface {
		Match(device tailscale.Device) bool
	}

	rxTagFilter regexp.Regexp

	FuncTagFilter func(device tailscale.Device) bool
)

// NewRegexpTagFilter creates a new tag filter based on the provided regular expressions.
func NewRegexpTagFilter(patterns ...string) (TagFilter, error) {
	if len(patterns) == 0 {
		// No patterns provided, match all devices.
		return FuncTagFilter(func(tailscale.Device) bool { return true }), nil
	}

	filter := ""
	for _, pattern := range patterns {
		_, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid tag pattern: %w", err)
		}
		filter += fmt.Sprintf("(%s)|", strings.Trim(pattern, "^$"))
	}
	filter = fmt.Sprintf("tag:(%s)(,|$)", strings.TrimSuffix(filter, "|"))
	rx, _ := regexp.Compile(filter)

	return (*rxTagFilter)(rx), nil
}

// Match returns true if the device matches the tag filter.
func (rx *rxTagFilter) Match(device tailscale.Device) bool {
	tags := strings.Join(device.Tags, ",")
	return (*regexp.Regexp)(rx).MatchString(tags)
}

// Match returns true if the device matches the tag filter.
func (f FuncTagFilter) Match(device tailscale.Device) bool {
	if f == nil {
		return false
	}
	return f(device)
}
