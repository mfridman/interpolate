package interpolate_test

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mfridman/interpolate"
)

var posixNameRegexp = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func FuzzInterpolate(f *testing.F) {
	for _, seed := range []string{
		"",
		"plain text",
		"$FOO ${BAR} $_ $1 $ $$ \\$ \\\\",
		"${FOO:-default} ${FOO-default} ${FOO:?message} ${FOO?message}",
		"${FOO:+alt} ${FOO+alt} ${#FOO}",
		"${FOO:1} ${FOO: -1} ${FOO:1:2} ${FOO:1:-2} ${FOO::2} ${FOO:2:}",
		"${FOO:-${BAR:-${BAZ:-$QUX}}}",
		"${FOO:1:9223372036854775807}",
		"$(echo hi) price: $5",
		"héllo ${FOO:2:3} \xff",
		"${FOO",
		"${",
	} {
		f.Add(seed)
	}

	env := interpolate.NewMapEnv(map[string]string{
		"FOO":   "héllo wörld 🦀",
		"BAR":   "bar",
		"EMPTY": "",
	})

	f.Fuzz(func(t *testing.T, input string) {
		got, err := interpolate.Interpolate(env, input)

		_, parseErr := interpolate.NewParser(input).Parse()
		identifiers, identifiersErr := interpolate.Identifiers(input)
		if (parseErr == nil) != (identifiersErr == nil) {
			t.Fatalf("Parse error = %v, but Identifiers error = %v", parseErr, identifiersErr)
		}
		if parseErr != nil && err == nil {
			t.Fatalf("Parse error = %v, but Interpolate succeeded", parseErr)
		}
		for _, identifier := range identifiers {
			if !posixNameRegexp.MatchString(identifier) {
				t.Errorf("Identifiers(%q) returned invalid name %q", input, identifier)
			}
		}

		// Without a dollar sign there is nothing to expand, so the input must not change.
		if !strings.Contains(input, "$") {
			if err != nil {
				t.Fatalf("Interpolate(%q) error = %v, want no error without a dollar sign", input, err)
			}
			if got != input {
				t.Fatalf("Interpolate(%q) = %q, want the input unchanged", input, got)
			}
		}

		// Valid UTF-8 input and env values must give valid UTF-8 output.
		if err == nil && utf8.ValidString(input) && !utf8.ValidString(got) {
			t.Fatalf("Interpolate(%q) = %q, which is not valid UTF-8", input, got)
		}
	})
}
