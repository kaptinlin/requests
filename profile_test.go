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

func (p testProfile) Apply(c *Client) error {
	if p.err != nil {
		return p.err
	}
	c.SetDefaultHeader("X-Profile", p.name)
	return nil
}

func TestApplyProfile(t *testing.T) {
	t.Run("applies profile", func(t *testing.T) {
		client := Create(nil)

		err := client.ApplyProfile(testProfile{name: "test"})

		require.NoError(t, err)
		require.NotNil(t, client.Headers)
		assert.Equal(t, "test", client.Headers.Get("X-Profile"))
	})

	t.Run("rejects nil profile", func(t *testing.T) {
		client := Create(nil)

		err := client.ApplyProfile(nil)

		require.Error(t, err)
		assert.ErrorIs(t, err, ErrInvalidConfigValue)
	})

	t.Run("wraps profile errors", func(t *testing.T) {
		client := Create(nil)

		err := client.ApplyProfile(testProfile{name: "broken", err: assert.AnError})

		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
		assert.Contains(t, err.Error(), `apply profile "broken"`)
	})
}

func TestWithProfile(t *testing.T) {
	client := New(WithProfile(testProfile{name: "option"}))

	require.NotNil(t, client.Headers)
	assert.Equal(t, "option", client.Headers.Get("X-Profile"))
}

func TestWithProfileIgnoresOptionError(t *testing.T) {
	client := New(WithProfile(testProfile{name: "broken", err: assert.AnError}))

	assert.NotNil(t, client)
	assert.Nil(t, client.Headers)
}

func TestEnableHTTP2(t *testing.T) {
	client := Create(nil)

	client.EnableHTTP2()

	transport, ok := client.HTTPClient.Transport.(*http.Transport)
	require.True(t, ok)
	assertHTTP2Configured(t, transport)
}
