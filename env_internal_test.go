package interpolate

import "testing"

// TestFoldCase tests the Windows behavior, where names are case insensitive, on every platform.
func TestFoldCase(t *testing.T) {
	t.Parallel()

	t.Run("map", func(t *testing.T) {
		t.Parallel()

		// "PATH" sorts before "Path", so "Path" is last and wins. Run it several times, because
		// map iteration order is random.
		for range 20 {
			env := newMapEnv(map[string]string{"PATH": "upper", "Path": "mixed", "home": "lower"}, true)
			if got, ok := env.Lookup("path"); got != "mixed" || !ok {
				t.Fatalf(`Lookup("path") = (%q, %v), want ("mixed", true)`, got, ok)
			}
			if got, ok := env.Lookup("HOME"); got != "lower" || !ok {
				t.Fatalf(`Lookup("HOME") = (%q, %v), want ("lower", true)`, got, ok)
			}
		}
	})

	t.Run("list", func(t *testing.T) {
		t.Parallel()

		env := newListEnv([]string{"Path=first", "PATH=second"}, true)
		if got, ok := env.Lookup("path"); got != "second" || !ok {
			t.Errorf(`Lookup("path") = (%q, %v), want ("second", true)`, got, ok)
		}
	})

	t.Run("case sensitive", func(t *testing.T) {
		t.Parallel()

		env := newListEnv([]string{"Path=mixed"}, false)
		if got, ok := env.Lookup("PATH"); got != "" || ok {
			t.Errorf(`Lookup("PATH") = (%q, %v), want ("", false)`, got, ok)
		}
	})
}
