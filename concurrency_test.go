package interpolate_test

import (
	"sync"
	"testing"

	"github.com/mfridman/interpolate"
)

// TestConcurrentUse checks the package documentation: Expand and Variables are safe for
// concurrent use with a shared Env. Run it with -race.
func TestConcurrentUse(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{"NAME": "llama", "EMPTY": ""})
	const input = "${NAME:0:3} ${EMPTY:-default} ${#NAME} ${UNSET+alt} $1"
	const want = "lla default 5  $1"

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 100 {
				got, err := interpolate.Expand(input, env)
				if err != nil || got != want {
					t.Errorf("Expand(%q) = (%q, %v), want (%q, nil)", input, got, err, want)
					return
				}
				if _, err := interpolate.Variables(input); err != nil {
					t.Errorf("Variables(%q) error = %v", input, err)
					return
				}
			}
		})
	}
	wg.Wait()
}
