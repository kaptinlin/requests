package requests

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestJitterBackoffStrategy(t *testing.T) {
	t.Run("OutputWithinExpectedBounds", func(t *testing.T) {
		base := DefaultBackoffStrategy(1 * time.Second)
		jittered := JitterBackoffStrategy(base, 0.25)

		for range 100 {
			delay := jittered(0)
			assert.GreaterOrEqual(t, delay, 750*time.Millisecond, "Delay should be >= 750ms with 25%% jitter on 1s base")
			assert.LessOrEqual(t, delay, 1250*time.Millisecond, "Delay should be <= 1250ms with 25%% jitter on 1s base")
		}
	})

	t.Run("ZeroFractionReturnsBaseDelay", func(t *testing.T) {
		base := DefaultBackoffStrategy(500 * time.Millisecond)
		jittered := JitterBackoffStrategy(base, 0)

		for range 10 {
			delay := jittered(0)
			assert.Equal(t, 500*time.Millisecond, delay, "Zero fraction should return exact base delay")
		}
	})

	t.Run("NegativeFractionReturnsBaseDelay", func(t *testing.T) {
		base := DefaultBackoffStrategy(500 * time.Millisecond)
		jittered := JitterBackoffStrategy(base, -0.5)

		for range 10 {
			delay := jittered(0)
			assert.Equal(t, 500*time.Millisecond, delay, "Negative fraction should return exact base delay")
		}
	})

	t.Run("ResultNeverNegative", func(t *testing.T) {
		// Use a very high fraction to try to produce negative results
		base := DefaultBackoffStrategy(10 * time.Millisecond)
		jittered := JitterBackoffStrategy(base, 2.0)

		for range 1000 {
			delay := jittered(0)
			assert.GreaterOrEqual(t, delay, time.Duration(0), "Jittered delay should never be negative")
		}
	})

	t.Run("WorksWithExponentialBackoff", func(t *testing.T) {
		base := ExponentialBackoffStrategy(100*time.Millisecond, 2.0, 10*time.Second)
		jittered := JitterBackoffStrategy(base, 0.1)

		// Attempt 0: base = 100ms, jitter ±10%
		delay := jittered(0)
		assert.GreaterOrEqual(t, delay, 90*time.Millisecond)
		assert.LessOrEqual(t, delay, 110*time.Millisecond)

		// Attempt 2: base = 400ms, jitter ±10%
		delay = jittered(2)
		assert.GreaterOrEqual(t, delay, 360*time.Millisecond)
		assert.LessOrEqual(t, delay, 440*time.Millisecond)
	})
}

func TestDefaultBackoffStrategy(t *testing.T) {
	strategy := DefaultBackoffStrategy(2 * time.Second)
	assert.Equal(t, 2*time.Second, strategy(0))
	assert.Equal(t, 2*time.Second, strategy(5))
}

func TestLinearBackoffStrategy(t *testing.T) {
	strategy := LinearBackoffStrategy(1 * time.Second)
	assert.Equal(t, 1*time.Second, strategy(0))
	assert.Equal(t, 2*time.Second, strategy(1))
	assert.Equal(t, 3*time.Second, strategy(2))
}

func TestExponentialBackoffStrategy(t *testing.T) {
	strategy := ExponentialBackoffStrategy(100*time.Millisecond, 2.0, 5*time.Second)
	assert.Equal(t, 100*time.Millisecond, strategy(0))
	assert.Equal(t, 200*time.Millisecond, strategy(1))
	assert.Equal(t, 400*time.Millisecond, strategy(2))

	// Should cap at maxBackoffTime
	assert.Equal(t, 5*time.Second, strategy(10))
}
