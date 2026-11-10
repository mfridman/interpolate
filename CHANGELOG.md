# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `LookupFunc`, an adapter that makes a function such as `os.LookupEnv` an `Env`.

### Changed

- `Interpolate(env, s)` is now `Expand(s, env)`.
- `Identifiers` is now `Variables`, and returns each name only once, in the order of first use.
- The `Get` method of `Env` is now `Lookup`.
- `NewSliceEnv(slice)` is now `NewListEnv(pairs...)`. Use `NewListEnv(os.Environ()...)` for a slice.

### Removed

- The exported parser and expression types, such as `Parser`, `NewParser`, `Expression`, `ExpressionItem`, `Expansion`, and the `*Expansion` types. Use `Expand` and `Variables`.

### Fixed

- On Windows, `NewMapEnv` with keys that differ only in case, such as `Path` and `PATH`, gave a random result. The last key in sorted order now wins.

## [v0.1.0] - 2026-11-10

### Added

- `${VAR:?message}`: error if `VAR` is unset or empty ([#2](https://github.com/mfridman/interpolate/issues/2)).
- `${VAR+word}` and `${VAR:+word}`: use `word` if `VAR` is set, or set and not empty.
- `${#VAR}`: the length of the value in characters.
- `\}` in the word of a brace expansion is a literal `}`, for example `${CONFIG:-{"a":1\}}`.
- New expansion types: `RequiredNonEmptyExpansion`, `AlternateExpansion`, `AlternateNonEmptyExpansion`, and `LengthExpansion`.
- Brace expansions can be nested at most 1000 levels deep. Deeper input returns an error instead of exhausting the stack.

### Changed

- Go 1.25 or later is required.
- A `$` that is not followed by a letter, `_`, or `{` is literal text. Before, inputs such as `$1`, `$ `, `$5`, `$@`, `'^abc$'`, and `${VAR:-$}` returned an error.
- Names follow POSIX: `[A-Za-z_][A-Za-z0-9_]*`. `$_VAR` and `${_VAR}` now expand. Non-ASCII letters are no longer part of a name, so `$é` is literal text, `$FOOé` expands `FOO`, and `${é}` is an error.
- `${VAR:+word}` is the alternate value. Before, it was parsed as a substring with offset `+word`, so `${VAR:+1}` was the value after the first character.
- Substring offsets and lengths count characters, not bytes.
- A negative substring offset before the start of the value gives an empty string, the same as bash. Before, it gave the whole value.
- A blank substring offset or length counts as 0, so `${VAR::2}`, `${VAR:2:}`, and `${VAR: }` work. Before, they returned an error.
- `Identifiers` includes the names in the word of `${VAR-word}` and `${VAR?word}`, the same as `${VAR:-word}`.
- Error messages start with a lowercase letter, and say "end of input" for unterminated expansions. An empty `${}` gives `empty expansion ${}, expected a variable name` ([#1](https://github.com/mfridman/interpolate/issues/1)). The `$VAR: message` errors from required expansions are unchanged.

### Fixed

- Substrings with a length near the maximum `int` value no longer panic.
- Invalid UTF-8 bytes in the input are kept unchanged. Before, they became U+FFFD.
- Calling `Parse` again on the same `Parser` parses the input again. Before, it returned an empty expression.

## [v0.0.2]

The last release before the changes above. See [buildkite/interpolate](https://github.com/buildkite/interpolate) for the history before the fork.

[Unreleased]: https://github.com/mfridman/interpolate/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/mfridman/interpolate/compare/v0.0.2...v0.1.0
[v0.0.2]: https://github.com/mfridman/interpolate/releases/tag/v0.0.2
