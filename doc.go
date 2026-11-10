// Package interpolate expands shell-style parameters, such as $NAME and ${NAME:-default}, in
// strings. It implements POSIX parameter expansion and some bash extensions, and reads values
// from an [Env].
//
// Supported expansions:
//
//	$NAME or ${NAME}        the value of NAME, or an empty string if NAME is unset
//	${NAME:-word}           word if NAME is unset or empty
//	${NAME-word}            word if NAME is unset
//	${NAME:?message}        an error if NAME is unset or empty
//	${NAME?message}         an error if NAME is unset
//	${NAME:+word}           word if NAME is set and not empty, otherwise an empty string
//	${NAME+word}            word if NAME is set, otherwise an empty string
//	${#NAME}                the length of the value in characters
//	${NAME:offset}          the value from offset
//	${NAME:offset:length}   at most length characters of the value from offset
//
// Write $$ or \$ for a literal dollar sign, and \} for a literal closing brace in a word. A $
// that is not followed by a letter, an underscore, or { is kept as-is, so $1 and $(command) do
// not change.
//
// [Interpolate] and [Identifiers] are safe for concurrent use if the [Env] is.
package interpolate
