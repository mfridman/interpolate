# interpolate

[![Build Status](https://github.com/mfridman/interpolate/actions/workflows/ci.yaml/badge.svg)](https://github.com/mfridman/interpolate/actions/workflows/ci.yaml)
[![Go Reference](https://pkg.go.dev/badge/github.com/mfridman/interpolate.svg)](https://pkg.go.dev/github.com/mfridman/interpolate)

A Go library that expands shell-style parameters, such as `$NAME` and `${NAME:-default}`, in strings. It implements [POSIX parameter expansion](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_06_02) and some [bash extensions](https://www.gnu.org/software/bash/manual/html_node/Shell-Parameter-Expansion.html).

## Installation

```sh
go get github.com/mfridman/interpolate@latest
```

Requires Go 1.25 or later.

## Usage

```go
package main

import (
	"fmt"
	"log"

	"github.com/mfridman/interpolate"
)

func main() {
	env := interpolate.NewListEnv("NAME=James")

	output, err := interpolate.Expand("Hello ${NAME}, welcome to the ${PLACE:-🏖}", env)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(output) // Hello James, welcome to the 🏖
}
```

Values come from an `Env`:

- `interpolate.LookupFunc(os.LookupEnv)` reads the process environment.
- `interpolate.NewMapEnv(m)` reads a `map[string]string`.
- `interpolate.NewListEnv("A=1", "B=2")` reads `NAME=value` pairs, such as `os.Environ()...`.
- Any type with a `Lookup(name string) (string, bool)` method is an `Env`.

`interpolate.Variables` returns the names of the variables in a string, without repeats, and does not expand it.

### Errors and strict mode

`${NAME?message}` and `${NAME:?message}` return an `*interpolate.UnsetVariableError` with the variable `Name` and the `Message`. Its text is `$NAME: message`.

```go
var unsetErr *interpolate.UnsetVariableError
if errors.As(err, &unsetErr) {
	fmt.Println(unsetErr.Name) // PORT
}
```

By default, an unset variable expands to an empty string, the same as a shell. Use `interpolate.WithNoUnset()` to make a reference to an unset variable an error, the same as `set -u` in bash. Expansions that handle an unset variable, such as `${NAME:-word}`, do not change.

```go
output, err := interpolate.Expand(input, env, interpolate.WithNoUnset())
// $TYPO -> error: $TYPO: not set
```

## Supported expansions

| Syntax | Result |
| --- | --- |
| `$NAME` or `${NAME}` | The value of `NAME`, or an empty string if it is unset |
| `${NAME:-word}` | `word` if `NAME` is unset or empty, otherwise the value |
| `${NAME-word}` | `word` if `NAME` is unset, otherwise the value |
| `${NAME:?message}` | An error if `NAME` is unset or empty, otherwise the value |
| `${NAME?message}` | An error if `NAME` is unset, otherwise the value |
| `${NAME:+word}` | `word` if `NAME` is set and not empty, otherwise an empty string |
| `${NAME+word}` | `word` if `NAME` is set, otherwise an empty string |
| `${#NAME}` | The length of the value in characters |
| `${NAME:offset}` | The value from `offset` to the end |
| `${NAME:offset:length}` | At most `length` characters of the value from `offset` |

- A name starts with a letter or an underscore, followed by letters, digits, or underscores (`[A-Za-z_][A-Za-z0-9_]*`).
- A `word` or `message` can contain other expansions, such as `${NAME:-${FALLBACK:-default}}`. It is expanded only when it is used. Expansions can be nested up to 1000 levels deep.
- Without a message, the error from `?` is `$NAME: not set`, and the error from `:?` is `$NAME: empty or not set`.

### Substrings

- Offsets and lengths count characters, not bytes. They are decimal integers; arithmetic is not supported.
- A negative offset counts from the end. Put a space before it, as in `${NAME: -2}`, because `${NAME:-2}` is a default value.
- An offset past the end, or before the start, gives an empty string.
- A length past the end gives the rest of the value. A negative length ends that many characters before the end, so `${NAME:1:-1}` removes the first and last characters.
- A blank offset or length counts as 0, so `${NAME::2}` gives the first two characters. `${NAME:}` is an error.

## Escaping

| Input | Output |
| --- | --- |
| `$$NAME` or `\$NAME` | `$NAME` |
| `$${NAME}` or `\${NAME}` | `${NAME}` |
| `\}` in a `word` or `message` | `}` |
| `$` not followed by a letter, `_`, or `{` | Kept as-is, so `$1`, `$(command)`, `price: $5`, and a trailing `$` do not change |
| `\\` | Kept as-is |

A word ends at the first `}`, so write `${CONFIG:-{"a":1\}}` to get `{"a":1}`. Outside of `${...}`, `\}` is kept as-is.

A double backslash is kept as two backslashes, and it does not escape the `$` after it: `\\$NAME` gives `\\` followed by the value of `NAME`. You cannot put one backslash directly before a literal `$`. In a regular expression, write `[$]` instead of `\$`.

### SQL

These rules matter when the input is SQL, for example in [goose](https://github.com/pressly/goose) migrations with `ENVSUB ON`:

- Write `$$$$` for a `$$` dollar-quoted string, and `$$tag$$` for `$tag$`. Without escaping, `$$` becomes `$`, and `$tag` expands as a variable.
- Write `%1$$s` in `format('%1$s', name)`, because `$s` is a variable.
- Positional parameters such as `$1`, and a `$` at the end of a regular expression such as `'^abc$'`, are kept as-is.

## Not supported

- Pattern removal: `${NAME%word}`, `${NAME%%word}`, `${NAME#word}`, and `${NAME##word}`.
- Assignment: `${NAME:=word}` and `${NAME=word}`.
- Special and positional parameters, such as `$1`, `$@`, `$*`, `$#`, and `$?`. They are kept as literal text.

## Acknowledgements

This project started as a fork of [buildkite/interpolate](https://github.com/buildkite/interpolate). Thanks to its authors for their work.

## License

[MIT](LICENSE.txt)
