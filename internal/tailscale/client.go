package tsutils

import (
	"fmt"
	"net/url"
	"regexp"

	"tailscale.com/client/tailscale/v2"
)

var (
	rxAPIKey   = regexp.MustCompile(`^tskey-auth-(?P<client_id>[a-zA-Z0-9]+CNTRL)-[a-zA-Z0-9]+$`)
	rxOAuthKey = regexp.MustCompile(`^tskey-client-(?P<client_id>[a-zA-Z0-9]+CNTRL)-[a-zA-Z0-9]+$`)
)

// NewTailscaleClient creates a new Tailscale client based on the provided configuration.
func NewTailscaleClient(baseURL *url.URL, tailnet, authkey string) (*tailscale.Client, error) {
	if tailnet == "" {
		return nil, fmt.Errorf("tailnet is required")
	}

	ts := &tailscale.Client{Tailnet: tailnet, BaseURL: baseURL}

	switch {
	case rxAPIKey.MatchString(authkey):
		return nil, fmt.Errorf("API keys are not supported in this controller, use OAuth keys instead")
	case rxOAuthKey.MatchString(authkey):
		ts.HTTP = tailscale.OAuthConfig{
			ClientID:     rxOAuthKey.FindStringSubmatch(authkey)[1],
			ClientSecret: authkey,
			Scopes:       []string{"devices:core:read"},
		}.HTTPClient()
	default:
		return nil, fmt.Errorf("invalid auth key format")
	}

	return ts, nil
}
