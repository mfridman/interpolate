package interpolate_test

import (
	"fmt"
	"log"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mfridman/interpolate"
)

func ExampleExpand() {
	env := interpolate.NewSliceEnv([]string{"NAME=James"})

	output, err := interpolate.Expand("Hello ${NAME}, welcome to the ${PLACE:-🏖}", env)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(output)

	// Output: Hello James, welcome to the 🏖
}

func ExampleVariables() {
	names, err := interpolate.Variables("${HOST:-localhost}:${PORT:?port is required} $1 $HOST")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(names)

	// Output: [HOST PORT]
}

func ExampleNewMapEnv() {
	env := interpolate.NewMapEnv(map[string]string{
		"USER":  "llama",
		"EMPTY": "",
	})

	output, err := interpolate.Expand("${USER:+--user=$USER} ${EMPTY:-default} ${#USER}", env)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(output)

	// Output: --user=llama default 5
}

// expandTest is a test case for Expand. If wantErr is set, the test expects an error with
// that exact message.
type expandTest struct {
	input   string
	want    string
	wantErr string
}

func runExpandTests(t *testing.T, env interpolate.Env, tests []expandTest) {
	t.Helper()

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			got, err := interpolate.Expand(tc.input, env)
			if tc.wantErr != "" {
				if err == nil || err.Error() != tc.wantErr {
					t.Errorf("Expand(%q) error = %v, want %q", tc.input, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Expand(%q) error = %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("Expand(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestBasicInterpolation(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{
		"TEST1": "A test",
		"TEST2": "Another",
		"TEST3": "Llamas",
		"TEST4": "Only one level of $TEST3 interpolation",
	})

	runExpandTests(t, env, []expandTest{
		{input: "", want: ""},
		{input: "foo", want: "foo"},
		{input: "test1", want: "test1"},
		{input: "TEST1", want: "TEST1"},
		{input: "$TEST1", want: "A test"},
		{input: "${TEST1}", want: "A test"},
		{input: "$TEST1, $TEST2, $TEST3", want: "A test, Another, Llamas"},
		{input: "$Test1, $Test2, $TeST3", want: ", , "},
		{input: "${TEST1}, ${Test2}, ${tEST3}", want: "A test, , "},
		{input: "my$TEST1", want: "myA test"},
		{input: "$TEST4", want: "Only one level of $TEST3 interpolation"},
		{input: "${TEST4}", want: "Only one level of $TEST3 interpolation"},
		{input: "$TEST1$TEST2", want: "A testAnother"},
		{input: "${TEST1}${TEST2}", want: "A testAnother"},
		{input: "unmatched } brace", want: "unmatched } brace"},
		{input: `back\slash \n`, want: `back\slash \n`},
		{input: `\\`, want: `\\`},
		{input: `\`, want: `\`},
	})
}

func TestNilEnv(t *testing.T) {
	t.Parallel()

	runExpandTests(t, nil, []expandTest{
		{input: "$UNSET", want: ""},
		{input: "${UNSET:-default}", want: "default"},
		{input: "${UNSET?}", wantErr: "$UNSET: not set"},
	})
}

func TestNestedInterpolation(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{
		"TEST1": "A test",
		"TEST2": "Another",
		"TEST3": "Llamas",
	})

	runExpandTests(t, env, []expandTest{
		{input: "${TEST5:-${TEST6:-$TEST1}}", want: "A test"},
		{input: "${TEST5:-${TEST2:-$TEST1}}", want: "Another"},
		{input: "${TEST5:-Some text ${TEST2:-$TEST1} with $TEST3}", want: "Some text Another with Llamas"},
		{input: "${TEST5:-${TEST6:-${TEST7:-${TEST8:-deep}}}}", want: "deep"},
	})
}

func TestLiteralDollarSigns(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{
		"TEST1": "A test",
	})

	runExpandTests(t, env, []expandTest{
		{input: "$", want: "$"},
		{input: "trailing $", want: "trailing $"},
		{input: "This $ is not a variable", want: "This $ is not a variable"},
		{input: "$1 burgers", want: "$1 burgers"},
		{input: "$99bottles", want: "$99bottles"},
		{input: "price: $5, $TEST1", want: "price: $5, A test"},
		{input: "SELECT * FROM t WHERE id = $1 AND name = $2", want: "SELECT * FROM t WHERE id = $1 AND name = $2"},
		{input: "a$\tb", want: "a$\tb"},
		{input: "$-$@$*$#$?$!", want: "$-$@$*$#$?$!"},
		{input: "grep 'xi$'", want: "grep 'xi$'"},
		{input: "${UNSET:-$}", want: "$"},
		{input: "${UNSET:-$5}", want: "$5"},
		{input: "$é", want: "$é"},
		{input: "$(echo hello world)", want: "$(echo hello world)"},
		{input: "testing $(echo $TEST1)", want: "testing $(echo A test)"},
		{input: "$(", want: "$("},
	})
}

func TestUnderscoreNames(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{
		"_":       "underscore",
		"_TEST":   "leading",
		"TEST_1_": "trailing",
	})

	runExpandTests(t, env, []expandTest{
		{input: "$_", want: "underscore"},
		{input: "${_}", want: "underscore"},
		{input: "$_TEST", want: "leading"},
		{input: "${_TEST}", want: "leading"},
		{input: "$TEST_1_", want: "trailing"},
		{input: "${_UNSET:-default}", want: "default"},
	})
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	runExpandTests(t, nil, []expandTest{
		{input: "${", wantErr: "expected identifier to start with a letter or underscore, got end of input"},
		{input: "${}", wantErr: "empty expansion ${}, expected a variable name"},
		{input: "text ${} text", wantErr: "empty expansion ${}, expected a variable name"},
		{input: "${UNSET:-${}}", wantErr: "empty expansion ${}, expected a variable name"},
		{input: "$${}", want: "${}"},
		{input: "${1}", wantErr: "expected identifier to start with a letter or underscore, got '1'"},
		{input: "${é}", wantErr: "expected identifier to start with a letter or underscore, got 'é'"},
		{input: "${FOO", wantErr: "expected an operator, got end of input"},
		{input: "${FOO!}", wantErr: "expected an operator, got '!'"},
		{input: "${FOO=bar}", wantErr: "expected an operator, got '='"},
		{input: "${FOO:=bar}", wantErr: `parse substring offset: strconv.Atoi: parsing "=bar": invalid syntax`},
		{input: "${FOO:-bar", wantErr: "expected brace expansion to end with }, got end of input"},
		{input: "${FOO:-${BAR}", wantErr: "expected brace expansion to end with }, got end of input"},
		{input: "${#}", wantErr: "expected identifier to start with a letter or underscore, got '}'"},
		{input: "${#FOO", wantErr: "expected length expansion to end with }, got end of input"},
		{input: "${#FOO:-x}", wantErr: "expected length expansion to end with }, got ':'"},
		{input: "${FOO:}", wantErr: "expected substring offset, got '}'"},
		{input: "${FOO:1", wantErr: "expected brace expansion to end with }, got end of input"},
		{input: "${FOO:1:2", wantErr: "expected brace expansion to end with }, got end of input"},
		{input: "${FOO:x}", wantErr: `parse substring offset: strconv.Atoi: parsing "x": invalid syntax`},
		{input: "${FOO:1:x}", wantErr: `parse substring length: strconv.Atoi: parsing "x": invalid syntax`},
		{input: "${FOO:1:2:3}", wantErr: `parse substring length: strconv.Atoi: parsing "2:3": invalid syntax`},
		{input: "${FOO:99999999999999999999}", wantErr: `parse substring offset: strconv.Atoi: parsing "99999999999999999999": value out of range`},
	})
}

func TestNestingDepthLimit(t *testing.T) {
	t.Parallel()

	// maxNestingDepth is the same as the unexported limit in parser.go.
	const maxNestingDepth = 1000

	nested := func(depth int) string {
		return strings.Repeat("${A:-", depth) + "deep" + strings.Repeat("}", depth)
	}
	wantErr := "brace expansions nested deeper than " + strconv.Itoa(maxNestingDepth) + " levels"

	runExpandTests(t, nil, []expandTest{
		{input: nested(maxNestingDepth), want: "deep"},
		{input: nested(maxNestingDepth + 1), wantErr: wantErr},
		// Sibling expansions do not add to the depth.
		{input: strings.Repeat("${A:-x}", maxNestingDepth+1), want: strings.Repeat("x", maxNestingDepth+1)},
		// Escaped expansions are text, so they do not add to the depth.
		{input: strings.Repeat("$${A:-", maxNestingDepth+1), want: strings.Repeat("${A:-", maxNestingDepth+1)},
	})

	// A very deep input must fail fast with an error, not exhaust the stack.
	if _, err := interpolate.Expand(nested(10_000_000), nil); err == nil || err.Error() != wantErr {
		t.Errorf("Expand(nested(10_000_000)) error = %v, want %q", err, wantErr)
	}
}

func TestMissingParameterValuesReturnEmptyStrings(t *testing.T) {
	t.Parallel()

	runExpandTests(t, nil, []expandTest{
		{input: "$BUILDKITE_COMMIT", want: ""},
		{input: "${BUILDKITE_COMMIT}", want: ""},
		{input: "${BUILDKITE_COMMIT:0:7}", want: ""},
		{input: "${BUILDKITE_COMMIT:7}", want: ""},
		{input: "${BUILDKITE_COMMIT:7:14}", want: ""},
		{input: "${BUILDKITE_COMMIT: -7:-1}", want: ""},
	})
}

func TestSubstringsWithOffsets(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{
		"BUILDKITE_COMMIT": "1adf998e39f647b4b25842f107c6ed9d30a3a7c7",
	})

	runExpandTests(t, env, []expandTest{
		// in range offsets, no lengths
		{input: "${BUILDKITE_COMMIT:0}", want: "1adf998e39f647b4b25842f107c6ed9d30a3a7c7"},
		{input: "${BUILDKITE_COMMIT:7}", want: "e39f647b4b25842f107c6ed9d30a3a7c7"},
		{input: "${BUILDKITE_COMMIT: -7}", want: "0a3a7c7"},
		{input: "${BUILDKITE_COMMIT:+7}", want: "7"}, // ":+" is an alternate value, not an offset
		{input: "${BUILDKITE_COMMIT: +7}", want: "e39f647b4b25842f107c6ed9d30a3a7c7"},

		// out of range offsets, no lengths
		{input: "${BUILDKITE_COMMIT: -128}", want: ""}, // before the start, the same as bash
		{input: "${BUILDKITE_COMMIT: -128:7}", want: ""},
		{input: "${BUILDKITE_COMMIT: -40}", want: "1adf998e39f647b4b25842f107c6ed9d30a3a7c7"},
		{input: "${BUILDKITE_COMMIT: -41}", want: ""},
		{input: "${BUILDKITE_COMMIT:128}", want: ""},

		// in range offsets and lengths
		{input: "${BUILDKITE_COMMIT:0:7}", want: "1adf998"},
		{input: "${BUILDKITE_COMMIT:7:7}", want: "e39f647"},
		{input: "${BUILDKITE_COMMIT:7:-7}", want: "e39f647b4b25842f107c6ed9d3"},
		{input: "${BUILDKITE_COMMIT: -7:3}", want: "0a3"},
		{input: "${BUILDKITE_COMMIT: -7:-3}", want: "0a3a"},

		// zero lengths
		{input: "${BUILDKITE_COMMIT:0:0}", want: ""},
		{input: "${BUILDKITE_COMMIT:7:0}", want: ""},

		// in range offsets and out of range lengths
		{input: "${BUILDKITE_COMMIT:0:128}", want: "1adf998e39f647b4b25842f107c6ed9d30a3a7c7"},
		{input: "${BUILDKITE_COMMIT:7:128}", want: "e39f647b4b25842f107c6ed9d30a3a7c7"},
		{input: "${BUILDKITE_COMMIT:0:-128}", want: ""},
		{input: "${BUILDKITE_COMMIT:7:-128}", want: ""},

		// empty or blank offsets and lengths count as 0, the same as bash
		{input: "${BUILDKITE_COMMIT: }", want: "1adf998e39f647b4b25842f107c6ed9d30a3a7c7"},
		{input: "${BUILDKITE_COMMIT::7}", want: "1adf998"},
		{input: "${BUILDKITE_COMMIT:7:}", want: ""},
		{input: "${BUILDKITE_COMMIT:7: }", want: ""},
		{input: "${BUILDKITE_COMMIT: 7 : 3 }", want: "e39"},

		// extreme values must not overflow
		{input: "${BUILDKITE_COMMIT:1:9223372036854775807}", want: "adf998e39f647b4b25842f107c6ed9d30a3a7c7"},
		{input: "${BUILDKITE_COMMIT:9223372036854775807:9223372036854775807}", want: ""},
		{input: "${BUILDKITE_COMMIT: -9223372036854775808:1}", want: ""},
		{input: "${BUILDKITE_COMMIT:1:-9223372036854775808}", want: ""},
	})
}

func TestSubstringsWithMultibyteCharacters(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{"GREETING": "héllo wörld 🦀"})

	runExpandTests(t, env, []expandTest{
		{input: "${GREETING:1}", want: "éllo wörld 🦀"},
		{input: "${GREETING:2}", want: "llo wörld 🦀"},
		{input: "${GREETING:1:3}", want: "éll"},
		{input: "${GREETING: -1}", want: "🦀"},
		{input: "${GREETING: -7:5}", want: "wörld"},
		{input: "${GREETING:6:-2}", want: "wörld"},
	})
}

func TestInvalidUTF8IsPreserved(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{"TEST1": "A test"})

	runExpandTests(t, env, []expandTest{
		{input: "bad\xffbyte", want: "bad\xffbyte"},
		{input: "\xfe\xff", want: "\xfe\xff"},
		{input: "$TEST1\xff", want: "A test\xff"},
		{input: "${UNSET:-\xff}", want: "\xff"},
		{input: "$\xff", want: "$\xff"},
		{input: "valid � stays", want: "valid � stays"},
	})
}

func TestInvalidUTF8InValuesIsPreserved(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{"BAD": "a\xffb\xfe"})

	runExpandTests(t, env, []expandTest{
		{input: "${BAD}", want: "a\xffb\xfe"},
		{input: "${BAD:0}", want: "a\xffb\xfe"},
		{input: "${BAD:1}", want: "\xffb\xfe"},
		{input: "${BAD:1:2}", want: "\xffb"},
		{input: "${BAD: -1}", want: "\xfe"},
		{input: "${#BAD}", want: "4"},
	})
}

func TestInterpolateIsntGreedy(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{
		"BUILDKITE_COMMIT":       "cfeeee3fa7fa1a6311723f5cbff95b738ec6e683",
		"BUILDKITE_PARALLEL_JOB": "456",
	})

	runExpandTests(t, env, []expandTest{
		{input: `echo "ENV_1=test_$BUILDKITE_COMMIT_$BUILDKITE_PARALLEL_JOB"`, want: `echo "ENV_1=test_456"`},
		{input: `echo "ENV_1=test-$BUILDKITE_COMMIT-$BUILDKITE_PARALLEL_JOB"`, want: `echo "ENV_1=test-cfeeee3fa7fa1a6311723f5cbff95b738ec6e683-456"`},
		{input: `echo "ENV_1=test_${BUILDKITE_COMMIT}_${BUILDKITE_PARALLEL_JOB}"`, want: `echo "ENV_1=test_cfeeee3fa7fa1a6311723f5cbff95b738ec6e683_456"`},
	})
}

func TestDefaultValues(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{
		"DAY":       "Blarghday",
		"EMPTY_DAY": "",
	})

	runExpandTests(t, env, []expandTest{
		{input: "Today is ${TODAY-Tuesday}", want: "Today is Tuesday"},
		{input: "Tomorrow is ${TOMORROW-Wednesday}", want: "Tomorrow is Wednesday"},
		{input: "Today is ${DAY-Wednesday}", want: "Today is Blarghday"},
		{input: "Today is ${DAY:-Wednesday}", want: "Today is Blarghday"},
		{input: "Today is ${EMPTY_DAY-Wednesday}", want: "Today is "},
		{input: "Today is ${EMPTY_DAY:-Wednesday}", want: "Today is Wednesday"},
		{input: "${EMPTY_DAY:--:{}}", want: "-:{}"},
		{input: "${EMPTY:-${LLAMAS-test}}", want: "test"},
		{input: "${EMPTY:-127}", want: "127"},
		{input: "${EMPTY:-}", want: ""},
		{input: "${EMPTY-}", want: ""},
		{input: "${DAY:-${UNSET?not evaluated}}", want: "Blarghday"},
	})
}

func TestRequiredVariables(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{
		"SET":   "value",
		"EMPTY": "",
		"NAME":  "llamas",
	})

	runExpandTests(t, env, []expandTest{
		{input: "Hello ${REQUIRED_VAR?}", wantErr: "$REQUIRED_VAR: not set"},
		{input: "Hello ${REQUIRED_VAR?y u no set me? :-{}", wantErr: "$REQUIRED_VAR: y u no set me? :-{"},
		{input: "Hello ${REQUIRED_VAR?{}}", wantErr: "$REQUIRED_VAR: {"},
		{input: "${REQUIRED_VAR?missing $NAME}", wantErr: "$REQUIRED_VAR: missing llamas"},
		{input: "${SET?}", want: "value"},
		{input: "${EMPTY?}", want: ""},
		{input: "${SET:?}", want: "value"},
		{input: "${EMPTY:?}", wantErr: "$EMPTY: empty or not set"},
		{input: "${REQUIRED_VAR:?}", wantErr: "$REQUIRED_VAR: empty or not set"},
		{input: "${EMPTY:?must be set}", wantErr: "$EMPTY: must be set"},
		{input: "${EMPTY:?${UNSET:-fallback} message}", wantErr: "$EMPTY: fallback message"},
		{input: "${UNSET:-${EMPTY:?inner}}", wantErr: "$EMPTY: inner"},
		{input: "${UNSET?${OTHER?message error wins}}", wantErr: "$OTHER: message error wins"},
		{input: "$SET ${UNSET?first} ${UNSET2?second}", wantErr: "$UNSET: first"},
	})
}

func TestAlternateValues(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{
		"SET":   "value",
		"EMPTY": "",
		"NAME":  "llamas",
	})

	runExpandTests(t, env, []expandTest{
		{input: "${SET+alt}", want: "alt"},
		{input: "${EMPTY+alt}", want: "alt"},
		{input: "${UNSET+alt}", want: ""},
		{input: "${SET:+alt}", want: "alt"},
		{input: "${EMPTY:+alt}", want: ""},
		{input: "${UNSET:+alt}", want: ""},
		{input: "${SET+}", want: ""},
		{input: "${SET:+--name=$NAME}", want: "--name=llamas"},
		{input: "${SET:+${UNSET:-nested}}", want: "nested"},
		{input: "${UNSET:+${UNSET2:?not evaluated}}", want: ""},
	})
}

func TestLength(t *testing.T) {
	t.Parallel()

	env := interpolate.NewMapEnv(map[string]string{
		"SET":       "value",
		"EMPTY":     "",
		"MULTIBYTE": "héllo 🦀",
	})

	runExpandTests(t, env, []expandTest{
		{input: "${#SET}", want: "5"},
		{input: "${#EMPTY}", want: "0"},
		{input: "${#UNSET}", want: "0"},
		{input: "${#MULTIBYTE}", want: "7"},
		{input: "${UNSET:-${#SET}}", want: "5"},
		{input: "$#SET", want: "$#SET"},
	})
}

func TestEscapingVariables(t *testing.T) {
	t.Parallel()

	runExpandTests(t, nil, []expandTest{
		{input: "Do this $$ESCAPE_PARTY", want: "Do this $ESCAPE_PARTY"},
		{input: `Do this \$ESCAPE_PARTY`, want: "Do this $ESCAPE_PARTY"},
		{input: "Do this $${SUCH_ESCAPE}", want: "Do this ${SUCH_ESCAPE}"},
		{input: `Do this \${SUCH_ESCAPE}`, want: "Do this ${SUCH_ESCAPE}"},
		{input: "Do this $${SUCH_ESCAPE:-$OTHERWISE}", want: "Do this ${SUCH_ESCAPE:-}"},
		{input: `Do this \${SUCH_ESCAPE:-$OTHERWISE}`, want: "Do this ${SUCH_ESCAPE:-}"},
		{input: "Do this $${SUCH_ESCAPE:-$$OTHERWISE}", want: "Do this ${SUCH_ESCAPE:-$OTHERWISE}"},
		{input: `Do this \${SUCH_ESCAPE:-\$OTHERWISE}`, want: "Do this ${SUCH_ESCAPE:-$OTHERWISE}"},
		{input: `echo "my favourite mountain is cotopaxi" | grep 'xi$$'`, want: `echo "my favourite mountain is cotopaxi" | grep 'xi$'`},
		{input: "$${not actually a brace expression", want: "${not actually a brace expression"},
		{input: "$$$$", want: "$$"},
		{input: "$$$", want: "$$"},
		{input: `\\$UNSET`, want: `\\`},
		{input: `\\\$UNSET`, want: `\\$UNSET`},
		{input: "${UNSET:-$$}", want: "$"},
		// SQL examples, as documented in the README.
		{input: "format('%1$$s', name)", want: "format('%1$s', name)"},
		{input: `E'\\n'`, want: `E'\\n'`},
		{input: `name ~ '^abc[$]'`, want: `name ~ '^abc[$]'`},
		{input: `name ~ '^abc\$'`, want: `name ~ '^abc$'`},
		{input: "AS $$ SELECT 1 $$", want: "AS $ SELECT 1 $"},
		{input: "AS $body$ SELECT 1 $body$", want: "AS $ SELECT 1 $"},
		{input: "AS $$$$ SELECT 1 $$$$", want: "AS $$ SELECT 1 $$"},
		{input: "AS $$body$$ SELECT 1 $$body$$", want: "AS $body$ SELECT 1 $body$"},
		{input: `${UNSET:-\}}`, want: `}`},
		{input: `${UNSET:-a\}b}`, want: `a}b`},
		{input: `${UNSET:-{"a":1\}}::jsonb`, want: `{"a":1}::jsonb`},
		{input: `${UNSET:-\\}}`, want: `\\}`},
		{input: `outside \} braces`, want: `outside \} braces`},
	})
}

func TestVariables(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		want  []string
	}{
		{input: "", want: []string{}},
		{input: "Hello ${REQUIRED_VAR?}", want: []string{"REQUIRED_VAR"}},
		{input: "${LLAMAS:-${ROCK:-true}}", want: []string{"LLAMAS", "ROCK"}},
		{input: "${BUILDKITE_COMMIT:0}", want: []string{"BUILDKITE_COMMIT"}},
		{input: "${UNSET-$DEFAULT}", want: []string{"UNSET", "DEFAULT"}},
		{input: "${REQUIRED?missing $NAME}", want: []string{"REQUIRED", "NAME"}},
		{input: "${A:?$B} ${C+$D} ${E:+$F} ${#G}", want: []string{"A", "B", "C", "D", "E", "F", "G"}},
		{input: "This $ is not a variable", want: []string{}},
		{input: "$1 and $_UNDERSCORE", want: []string{"_UNDERSCORE"}},
		{input: "$$ESCAPED and $NOT_ESCAPED", want: []string{"NOT_ESCAPED"}},
		{input: "$A $A ${A}", want: []string{"A"}},
		{input: "$B ${A:-$B} $A ${#B}", want: []string{"B", "A"}},
		{input: "${A:?$A}", want: []string{"A"}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			got, err := interpolate.Variables(tc.input)
			if err != nil {
				t.Fatalf("Variables(%q) error = %v", tc.input, err)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("Variables(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestVariablesParseError(t *testing.T) {
	t.Parallel()

	if _, err := interpolate.Variables("${FOO"); err == nil {
		t.Error(`Variables("${FOO") error = nil, want error`)
	}
}

func BenchmarkExpand(b *testing.B) {
	env := interpolate.NewSliceEnv([]string{
		"HELLO_WORLD=🦀",
	})

	b.ReportAllocs()
	for b.Loop() {
		_, _ = interpolate.Expand("Buildkite... ${HELLO_WORLD} ${ANOTHER_VAR:-🏖}", env)
	}
}

func BenchmarkSubstring(b *testing.B) {
	env := interpolate.NewMapEnv(map[string]string{
		"COMMIT": "1adf998e39f647b4b25842f107c6ed9d30a3a7c7",
	})

	b.ReportAllocs()
	for b.Loop() {
		_, _ = interpolate.Expand("${COMMIT:0:7}", env)
	}
}
