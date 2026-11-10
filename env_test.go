package interpolate_test

import (
	"runtime"
	"testing"

	"github.com/mfridman/interpolate"
)

func TestNewSliceEnv(t *testing.T) {
	t.Parallel()

	env := interpolate.NewSliceEnv([]string{
		"SIMPLE=value",
		"EQUALS=a=b=c",
		"EMPTY=",
		"NO_EQUALS",
		"DUPLICATE=first",
		"DUPLICATE=second",
		"=C:=C:\\",
	})

	for _, tc := range []struct {
		key    string
		want   string
		wantOK bool
	}{
		{key: "SIMPLE", want: "value", wantOK: true},
		{key: "EQUALS", want: "a=b=c", wantOK: true},
		{key: "EMPTY", want: "", wantOK: true},
		{key: "NO_EQUALS", want: "", wantOK: false},
		{key: "DUPLICATE", want: "second", wantOK: true},
		{key: "UNSET", want: "", wantOK: false},
	} {
		t.Run(tc.key, func(t *testing.T) {
			t.Parallel()

			got, ok := env.Get(tc.key)
			if got != tc.want || ok != tc.wantOK {
				t.Errorf("Get(%q) = (%q, %v), want (%q, %v)", tc.key, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestNewMapEnv(t *testing.T) {
	t.Parallel()

	values := map[string]string{"KEY": "value", "EMPTY": ""}
	env := interpolate.NewMapEnv(values)

	// The env must copy the map, so later changes to the map have no effect.
	values["KEY"] = "changed"
	values["ADDED"] = "added"

	if got, ok := env.Get("KEY"); got != "value" || !ok {
		t.Errorf(`Get("KEY") = (%q, %v), want ("value", true)`, got, ok)
	}
	if got, ok := env.Get("EMPTY"); got != "" || !ok {
		t.Errorf(`Get("EMPTY") = (%q, %v), want ("", true)`, got, ok)
	}
	if got, ok := env.Get("ADDED"); got != "" || ok {
		t.Errorf(`Get("ADDED") = (%q, %v), want ("", false)`, got, ok)
	}
}

func TestEnvKeyCase(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{"Path": "value"})
	got, ok := env.Get("PATH")

	// Windows env names are case insensitive. Other platforms are case sensitive.
	if runtime.GOOS == "windows" {
		if got != "value" || !ok {
			t.Errorf(`Get("PATH") = (%q, %v), want ("value", true)`, got, ok)
		}
		return
	}
	if got != "" || ok {
		t.Errorf(`Get("PATH") = (%q, %v), want ("", false)`, got, ok)
	}
}

func TestNilEnvValues(t *testing.T) {
	t.Parallel()

	for name, env := range map[string]interpolate.Env{
		"NewSliceEnv(nil)": interpolate.NewSliceEnv(nil),
		"NewMapEnv(nil)":   interpolate.NewMapEnv(nil),
	} {
		if got, ok := env.Get("ANY"); got != "" || ok {
			t.Errorf(`%s.Get("ANY") = (%q, %v), want ("", false)`, name, got, ok)
		}
	}
}
