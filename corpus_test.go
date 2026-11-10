package interpolate_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"github.com/mfridman/interpolate"
)

// TestCorpus checks Expand against the cases in testdata/corpus.json. The expected results come
// from bash, for inputs that mean the same in this package and in bash. The test reads the file
// and does not need bash. To regenerate the file with bash 5 or later:
//
//	go test -run TestCorpus -update-corpus
//
// Set INTERPOLATE_BASH to choose the bash binary.
var updateCorpus = flag.Bool("update-corpus", false, "regenerate testdata/corpus.json with bash")

const corpusPath = "testdata/corpus.json"

// corpusEnv is the environment for every case, in bash and in Expand.
var corpusEnv = map[string]string{
	"SET":       "value",
	"EMPTY":     "",
	"SPACE":     "a b",
	"MULTIBYTE": "héllo wörld 🦀",
	"DOLLAR":    "$SET ${SET}",
	"BRACE":     "}{",
	"_U":        "underscore",
	"N1":        "one",
}

type corpusCase struct {
	Input   string `json:"input"`
	NoUnset bool   `json:"nounset,omitempty"`
	Want    string `json:"want"`
	// Error is the error from bash, without the "bash: line N: " prefix.
	Error string `json:"error,omitempty"`
}

type corpusFile struct {
	Bash  string            `json:"bash"`
	Env   map[string]string `json:"env"`
	Cases []corpusCase      `json:"cases"`
}

func TestCorpus(t *testing.T) {
	t.Parallel()

	if *updateCorpus {
		writeCorpus(t)
	}

	data, err := os.ReadFile(corpusPath)
	if err != nil {
		t.Fatalf("read %s: %v", corpusPath, err)
	}
	var corpus corpusFile
	if err := json.Unmarshal(data, &corpus); err != nil {
		t.Fatalf("parse %s: %v", corpusPath, err)
	}
	if len(corpus.Cases) < 1000 {
		t.Fatalf("%s has %d cases, want at least 1000", corpusPath, len(corpus.Cases))
	}

	env := interpolate.NewMapEnv(corpus.Env)
	for _, tc := range corpus.Cases {
		var opts []interpolate.Option
		if tc.NoUnset {
			opts = append(opts, interpolate.WithNoUnset())
		}
		got, err := interpolate.Expand(tc.Input, env, opts...)
		label := fmt.Sprintf("Expand(%q, nounset=%v)", tc.Input, tc.NoUnset)

		if tc.Error == "" {
			if err != nil || got != tc.Want {
				t.Errorf("%s = (%q, %v), want %q", label, got, err, tc.Want)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s = %q, want error %q", label, got, tc.Error)
			continue
		}
		if want, ok := bashUnsetError(tc.Error); ok {
			if err.Error() != want {
				t.Errorf("%s error = %q, want %q (bash: %q)", label, err, want, tc.Error)
			}
			continue
		}
		// bash says "bad substitution" for input that does not parse.
		var unsetErr *interpolate.UnsetVariableError
		if errors.As(err, &unsetErr) {
			t.Errorf("%s error = %q, want a parse error (bash: %q)", label, err, tc.Error)
		}
	}
}

// bashUnsetError converts an error from bash for an unset or empty variable to the error text of
// Expand. It reports false for other errors, such as "bad substitution".
func bashUnsetError(bashErr string) (string, bool) {
	name, message, ok := strings.Cut(bashErr, ": ")
	if !ok || !posixNameRegexp.MatchString(name) || strings.Contains(message, "arithmetic syntax error") {
		return "", false
	}
	switch message {
	case "parameter null or not set":
		message = "empty or not set"
	case "parameter not set", "unbound variable":
		message = "not set"
	}
	return "$" + name + ": " + message, true
}

// corpusInputs returns the inputs of the corpus. Every input must mean the same in this package
// and in bash inside double quotes. So it has no quotes, backslashes, $$, $(...), backticks, ~,
// special parameters such as $1 or $#, and no assignment or pattern operators.
func corpusInputs() []string {
	var inputs []string
	add := func(input ...string) { inputs = append(inputs, input...) }

	names := []string{"SET", "EMPTY", "UNSET", "SPACE", "MULTIBYTE", "DOLLAR", "BRACE", "_U", "N1"}
	words := []string{
		"", "word", "two words", "$SET", "${SET}", "$EMPTY", "$UNSET", "${EMPTY:-x}", "${UNSET-y}",
		"${SET:+alt}", "pre${SET}post", "${#MULTIBYTE}", "${SET:1:3}", "a}", "{", "${UNSET:-${SET}}",
		"$ x", "$", "${DOLLAR}",
	}
	operators := []string{":-", "-", ":+", "+", ":?", "?"}

	// Plain references and the length.
	for _, name := range names {
		add("$"+name, "${"+name+"}", "${#"+name+"}", "<$"+name+">", "$"+name+"_x", "${"+name+"}_x")
	}

	// Every operator with every word.
	for _, name := range names {
		for _, op := range operators {
			for _, word := range words {
				add("${" + name + op + word + "}")
			}
		}
	}

	// Substrings. A negative offset needs a space, so that it is not ${NAME:-word}.
	offsets := []string{"0", "1", "2", "5", "13", "14", "20", " -1", " -2", " -5", " -13", " -14", " -20", "", " "}
	lengths := []string{"", "0", "1", "3", "13", "20", "-1", "-3", "-13"}
	for _, name := range []string{"SET", "EMPTY", "UNSET", "MULTIBYTE", "SPACE"} {
		for _, offset := range offsets {
			if offset != "" {
				add("${" + name + ":" + offset + "}")
			}
			for _, length := range lengths {
				add("${" + name + ":" + offset + ":" + length + "}")
			}
		}
	}

	// Nesting, concatenation, and text around expansions.
	add(
		"${UNSET:-${EMPTY:-${SET}}}",
		"${UNSET:-${UNSET2:-${UNSET3:-deep}}}",
		"${SET:+${UNSET:-${SET:1}}}",
		"${EMPTY:-${SET:+${#SET}}}",
		"${UNSET-${EMPTY-not used}}",
		"$SET$SET${SET}x$SET",
		"${SET}${EMPTY}${UNSET}${SPACE}",
		"a${SET}b${UNSET:-c}d${EMPTY:+e}f",
		"${SET:-a}}",
		"${UNSET:-{a}b}",
		"{${SET}}",
		"plain text",
		"a}b{c",
		"price $ 5",
		"trailing $",
		"${UNSET?${UNSET2?inner}}",
		"${EMPTY:?${SET} is set}",
		"${UNSET:?message with ${SET:0:2}}",
	)

	// Input that bash does not accept, and this package does not either. Not ${ }, which is a
	// command in bash 5.3, and not ${SET:x}, which bash reads as arithmetic.
	add("${}", "${:-x}", "${SET!}", "${SET:}", "${1SET}", "${SET SET}", "${#SET:-x}", "${SET:1:2:3}")
	return inputs
}

func writeCorpus(t *testing.T) {
	t.Helper()

	bash := os.Getenv("INTERPOLATE_BASH")
	if bash == "" {
		bash = "bash"
	}
	out, err := exec.Command(bash, "-c", "echo $BASH_VERSION").Output()
	if err != nil {
		t.Fatalf("run %s: %v", bash, err)
	}
	version := strings.TrimSpace(string(out))
	if major, _, _ := strings.Cut(version, "."); major < "5" {
		t.Fatalf("%s is version %s, want 5 or later; set INTERPOLATE_BASH", bash, version)
	}

	envList := []string{"LC_ALL=en_US.UTF-8"}
	for name, value := range corpusEnv {
		envList = append(envList, name+"="+value)
	}
	prefix := regexp.MustCompile(`^.*?line \d+: `)

	corpus := corpusFile{Bash: version, Env: corpusEnv}
	for _, input := range corpusInputs() {
		for _, noUnset := range []bool{false, true} {
			script := `eval "out=\"$1\"" && printf %s "$out"`
			if noUnset {
				script = "set -u; " + script
			}
			cmd := exec.Command(bash, "-c", script, "bash", input)
			cmd.Env = envList
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			tc := corpusCase{Input: input, NoUnset: noUnset}
			if err := cmd.Run(); err != nil {
				tc.Error = prefix.ReplaceAllString(strings.TrimSpace(stderr.String()), "")
				if tc.Error == "" {
					tc.Error = err.Error()
				}
			} else {
				tc.Want = stdout.String()
			}
			if !keepCase(tc) {
				continue
			}
			corpus.Cases = append(corpus.Cases, tc)
		}
	}

	data, err := marshalCorpus(corpus)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("testdata", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpusPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d cases to %s with bash %s", len(corpus.Cases), corpusPath, version)
}

// marshalCorpus formats the corpus with one case on each line, so changes are easy to review.
func marshalCorpus(corpus corpusFile) ([]byte, error) {
	var buf bytes.Buffer
	bashVersion, err := json.Marshal(corpus.Bash)
	if err != nil {
		return nil, err
	}
	env, err := json.Marshal(corpus.Env)
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(&buf, "{\n\"bash\": %s,\n\"env\": %s,\n\"cases\": [\n", bashVersion, env)
	for i, tc := range corpus.Cases {
		line, err := json.Marshal(tc)
		if err != nil {
			return nil, err
		}
		buf.Write(line)
		if i < len(corpus.Cases)-1 {
			buf.WriteByte(',')
		}
		buf.WriteByte('\n')
	}
	buf.WriteString("]\n}\n")
	return buf.Bytes(), nil
}

// keepCase reports whether a case shows behavior that this package shares with bash. It drops
// the known differences, which the README documents.
func keepCase(tc corpusCase) bool {
	// bash fails when a negative substring length ends before the offset. Expand gives an empty
	// string.
	if strings.Contains(tc.Error, "substring expression < 0") {
		return false
	}
	// When the message of ${NAME?message} expands to an empty string, bash prints an empty
	// message. Expand uses the default message.
	if name, ok := strings.CutSuffix(tc.Error, ":"); ok && posixNameRegexp.MatchString(name) {
		return false
	}
	return true
}
