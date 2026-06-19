package requests

import (
	"testing"

	"github.com/test-go/testify/require"
)

func newTestClient(t testing.TB, opts ...Option) *Client {
	t.Helper()

	client, err := New(opts...)
	require.NoError(t, err)
	return client
}
