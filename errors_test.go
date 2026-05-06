package requests

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

type fakeTimeoutErr struct{}

func (fakeTimeoutErr) Error() string   { return "fake timeout" }
func (fakeTimeoutErr) Timeout() bool   { return true }
func (fakeTimeoutErr) Temporary() bool { return false }

var (
	errPlain  = errors.New("plain error")
	errDialed = errors.New("refused")
)

func TestIsTimeout(t *testing.T) {
	t.Parallel()

	assert.False(t, IsTimeout(nil))
	assert.True(t, IsTimeout(context.DeadlineExceeded))
	assert.True(t, IsTimeout(fmt.Errorf("wrapped: %w", context.DeadlineExceeded)))
	assert.True(t, IsTimeout(fakeTimeoutErr{}))
	assert.True(t, IsTimeout(fmt.Errorf("wrapped: %w", fakeTimeoutErr{})))

	assert.False(t, IsTimeout(context.Canceled))
	assert.False(t, IsTimeout(errPlain))
}

func TestIsCanceled(t *testing.T) {
	t.Parallel()

	assert.False(t, IsCanceled(nil))
	assert.True(t, IsCanceled(context.Canceled))
	assert.True(t, IsCanceled(fmt.Errorf("wrapped: %w", context.Canceled)))

	// Deadline-driven errors are timeouts, not cancellations.
	assert.False(t, IsCanceled(context.DeadlineExceeded))
	assert.False(t, IsCanceled(fakeTimeoutErr{}))
	assert.False(t, IsCanceled(errPlain))
}

func TestIsConnectionError(t *testing.T) {
	t.Parallel()

	assert.False(t, IsConnectionError(nil))

	opErr := &net.OpError{Op: "dial", Net: "tcp", Err: errDialed}
	assert.True(t, IsConnectionError(opErr))
	assert.True(t, IsConnectionError(fmt.Errorf("wrapped: %w", opErr)))

	assert.False(t, IsConnectionError(context.Canceled))
	assert.False(t, IsConnectionError(context.DeadlineExceeded))
	assert.False(t, IsConnectionError(errPlain))
}
