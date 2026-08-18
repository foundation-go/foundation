package foundation

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Shutdown used to read a cancelFunc field that Start assigned — a data race
// with any goroutine that called it, and a silent no-op for anything that
// called it before Start got that far.
func TestShutdownBeforeStart(t *testing.T) {
	svc := &Service{}

	require.NotPanics(t, func() { svc.Shutdown() })

	select {
	case <-svc.stopSignal():
	default:
		t.Fatal("Shutdown before Start must still be observed")
	}
}

func TestShutdownIsIdempotentAndConcurrencySafe(t *testing.T) {
	svc := &Service{}

	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()
			svc.Shutdown()
		}()
	}

	// Reading the signal concurrently with the closes is the race the old
	// implementation had.
	for i := 0; i < 32; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			select {
			case <-svc.stopSignal():
			default:
			}
		}()
	}

	wg.Wait()

	select {
	case <-svc.stopSignal():
	default:
		t.Fatal("the stop signal should be closed")
	}
}

func TestVersionHasNoLeadingV(t *testing.T) {
	// The startup banner and the info metric add their own "v"; a version that
	// already carries one printed "vv0.3.0".
	assert.NotEmpty(t, Version)
	assert.NotEqual(t, byte('v'), Version[0])

	assert.Equal(t, "0.3.0", normalizeVersion("v0.3.0"))
	assert.Equal(t, "0.3.0", normalizeVersion("0.3.0"))
}

func TestVersionFromBuildInfoFallsBack(t *testing.T) {
	assert.Equal(t, "fallback", versionFromBuildInfo("fallback"))
}
