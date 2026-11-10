package interpolate_test

import (
	"runtime"
	"slices"
	"testing"

	"github.com/mfridman/interpolate"
)

func TestLookupFunc(t *testing.T) {
	t.Parallel()

	var looked []string
	env := interpolate.LookupFunc(func(name string) (string, bool) {
		looked = append(looked, name)
		if name == "EMPTY" {
			return "", true
		}
		return "", false
	})

	got, err := interpolate.Expand("${EMPTY-unset}${OTHER-unset}", env)
	if err != nil || got != "unset" {
		t.Errorf("Expand() = (%q, %v), want (\"unset\", nil)", got, err)
	}
	if want := []string{"EMPTY", "OTHER"}; !slices.Equal(looked, want) {
		t.Errorf("looked up %q, want %q", looked, want)
	}
}

func TestNewListEnv(t *testing.T) {
	t.Parallel()

	env := interpolate.NewListEnv(
		"SIMPLE=value",
		"EQUALS=a=b=c",
		"EMPTY=",
		"NO_EQUALS",
		"DUPLICATE=first",
		"DUPLICATE=second",
		"=C:=C:\\",
	)

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

			got, ok := env.Lookup(tc.key)
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

	if got, ok := env.Lookup("KEY"); got != "value" || !ok {
		t.Errorf(`Lookup("KEY") = (%q, %v), want ("value", true)`, got, ok)
	}
	if got, ok := env.Lookup("EMPTY"); got != "" || !ok {
		t.Errorf(`Lookup("EMPTY") = (%q, %v), want ("", true)`, got, ok)
	}
	if got, ok := env.Lookup("ADDED"); got != "" || ok {
		t.Errorf(`Lookup("ADDED") = (%q, %v), want ("", false)`, got, ok)
	}
}

func TestEnvKeyCase(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{"Path": "value"})
	got, ok := env.Lookup("PATH")

	// Windows env names are case insensitive. Other platforms are case sensitive.
	if runtime.GOOS == "windows" {
		if got != "value" || !ok {
			t.Errorf(`Lookup("PATH") = (%q, %v), want ("value", true)`, got, ok)
		}
		return
	}
	if got != "" || ok {
		t.Errorf(`Lookup("PATH") = (%q, %v), want ("", false)`, got, ok)
	}
}

func TestNilEnvValues(t *testing.T) {
	t.Parallel()

	for name, env := range map[string]interpolate.Env{
		"NewListEnv()":   interpolate.NewListEnv(),
		"NewMapEnv(nil)": interpolate.NewMapEnv(nil),
	} {
		if got, ok := env.Lookup("ANY"); got != "" || ok {
			t.Errorf(`%s.Lookup("ANY") = (%q, %v), want ("", false)`, name, got, ok)
		}
	}
}
