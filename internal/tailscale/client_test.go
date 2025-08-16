package tsutils_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tailscale.com/client/tailscale/v2"

	tsutils "github.com/chezmoidotsh/argotails/internal/tailscale"
)

func TestNewTailscaleClient_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := json.Marshal(map[string]interface{}{
			"devices": []tailscale.Device{
				{Name: "device1"},
			},
		})
		_, _ = w.Write(raw)
	}))
	defer srv.Close()

	serverURL, err := url.Parse(srv.URL)
	require.NoError(t, err)

	_, err = tsutils.NewTailscaleClient(serverURL, "example-tailnet", "tskey-client-1234CNTRL-5678")
	assert.NoError(t, err)
}

func TestNewTailscaleClient_Error(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		tailnet string
		authkey string
		err     string
	}{
		{
			name:    "InvalidAuthKeyFormat",
			baseURL: "https://api.tailscale.com",
			tailnet: "example-tailnet",
			authkey: "invalid-key",
			err:     "invalid auth key format",
		},
		{
			name:    "UnsupportedAPIKey",
			baseURL: "https://api.tailscale.com",
			tailnet: "example-tailnet",
			authkey: "tskey-auth-1234CNTRL-5678",
			err:     "API keys are not supported in this controller, use OAuth keys instead",
		},
		{
			name:    "EmptyTailnet",
			baseURL: "https://api.tailscale.com",
			tailnet: "",
			authkey: "tskey-client-1234CNTRL-5678",
			err:     "tailnet is required",
		},
		{
			name:    "EmptyAuthKey",
			baseURL: "https://api.tailscale.com",
			tailnet: "example-tailnet",
			authkey: "",
			err:     "invalid auth key format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseURL, err := url.Parse(tt.baseURL)
			require.NoError(t, err)

			_, err = tsutils.NewTailscaleClient(baseURL, tt.tailnet, tt.authkey)
			assert.EqualError(t, err, tt.err)
		})
	}
}
