package requests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/test-go/testify/require"
)

type testProfile struct {
	name string
	err  error
}

func (p testProfile) Name() string {
	return p.name
}

func (p testProfile) Options() []Option {
	return []Option{
		func(c *Client) error {
			if p.err != nil {
				return p.err
			}
			c.setDefaultHeader("X-Profile", p.name)
			return nil
		},
	}
}

func TestWithProfile(t *testing.T) {
	client := newTestClient(t, WithProfile(testProfile{name: "option"}))

	require.NotNil(t, client.headers)
	assert.Equal(t, "option", client.headers.Get("X-Profile"))
}

func TestWithProfileReturnsOptionError(t *testing.T) {
	client, err := New(WithProfile(testProfile{name: "broken", err: assert.AnError}))

	require.Error(t, err)
	assert.Nil(t, client)
	assert.ErrorIs(t, err, assert.AnError)
	assert.Contains(t, err.Error(), `apply profile "broken"`)
}

func TestWithProfileRejectsNilProfile(t *testing.T) {
	client, err := New(WithProfile(nil))

	require.Error(t, err)
	assert.Nil(t, client)
	assert.ErrorIs(t, err, ErrInvalidConfigValue)
}

func TestEnableHTTP2(t *testing.T) {
	client := newTestClient(t, WithHTTP2())

	transport, ok := client.httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assertHTTP2Configured(t, transport)
}
