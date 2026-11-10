package interpolate

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// An expansion is something that takes in ENV and returns a string or an error
type expansion interface {
	// Expand expands the expansion using variables from env.
	Expand(env Env) (string, error)

	// Identifiers returns any variable names referenced within the expansion.
	Identifiers() []string
}

// variableExpansion represents either $VAR or ${VAR}, our simplest expansion
type variableExpansion struct {
	Identifier string
}

func (e variableExpansion) Identifiers() []string {
	return []string{e.Identifier}
}

func (e variableExpansion) Expand(env Env) (string, error) {
	val, _ := env.Get(e.Identifier)
	return val, nil
}

// emptyValueExpansion returns either the value of an env, or a default value if it's unset or null
type emptyValueExpansion struct {
	Identifier string
	Content    expression
}

func (e emptyValueExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Content.Identifiers()...)
}

func (e emptyValueExpansion) Expand(env Env) (string, error) {
	val, _ := env.Get(e.Identifier)
	if val == "" {
		return e.Content.Expand(env)
	}
	return val, nil
}

// unsetValueExpansion returns either the value of an env, or a default value if it's unset
type unsetValueExpansion struct {
	Identifier string
	Content    expression
}

func (e unsetValueExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Content.Identifiers()...)
}

func (e unsetValueExpansion) Expand(env Env) (string, error) {
	val, ok := env.Get(e.Identifier)
	if !ok {
		return e.Content.Expand(env)
	}
	return val, nil
}

// substringExpansion returns a substring (or slice) of the env. This is ${VAR:offset} or
// ${VAR:offset:length}.
type substringExpansion struct {
	Identifier string
	Offset     int
	Length     int
	HasLength  bool
}

func (e substringExpansion) Identifiers() []string {
	return []string{e.Identifier}
}

func (e substringExpansion) Expand(env Env) (string, error) {
	value, _ := env.Get(e.Identifier)
	// Offsets and lengths count characters, not bytes, the same as bash in a UTF-8 locale. Each
	// invalid UTF-8 byte counts as one character and is kept unchanged.
	size := utf8.RuneCountInString(value)

	// A negative offset counts from the end. An offset before the start gives an empty string,
	// the same as bash. The order of operations prevents integer overflow for extreme offsets
	// and lengths.
	from := e.Offset
	if from < 0 {
		if from < -size {
			return "", nil
		}
		from += size
	}
	from = min(from, size)

	// A positive length counts from the offset. A negative length counts back from the end, and
	// cannot go before the offset.
	to := size
	if e.HasLength {
		if e.Length >= 0 {
			to = from + min(e.Length, size-from)
		} else {
			to = max(size+e.Length, from)
		}
	}

	start := byteOffset(value, from)
	end := start + byteOffset(value[start:], to-from)
	return value[start:end], nil
}

// byteOffset returns the byte offset of the character at index in s. Each invalid UTF-8 byte
// counts as one character, the same as [utf8.RuneCountInString].
func byteOffset(s string, index int) int {
	offset := 0
	for range index {
		_, size := utf8.DecodeRuneInString(s[offset:])
		offset += size
	}
	return offset
}

// requiredExpansion returns an env value, or an error if it is unset. This is ${VAR?message}.
type requiredExpansion struct {
	Identifier string
	Message    expression
}

func (e requiredExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Message.Identifiers()...)
}

func (e requiredExpansion) Expand(env Env) (string, error) {
	val, ok := env.Get(e.Identifier)
	if !ok {
		return "", requiredError(env, e.Identifier, e.Message, "not set")
	}
	return val, nil
}

// requiredNonEmptyExpansion returns an env value, or an error if it is unset or empty. This is
// ${VAR:?message}.
type requiredNonEmptyExpansion struct {
	Identifier string
	Message    expression
}

func (e requiredNonEmptyExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Message.Identifiers()...)
}

func (e requiredNonEmptyExpansion) Expand(env Env) (string, error) {
	val, _ := env.Get(e.Identifier)
	if val == "" {
		return "", requiredError(env, e.Identifier, e.Message, "empty or not set")
	}
	return val, nil
}

// alternateExpansion returns the expansion of Content if the env is set, and an empty string
// otherwise. This is ${VAR+word}.
type alternateExpansion struct {
	Identifier string
	Content    expression
}

func (e alternateExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Content.Identifiers()...)
}

func (e alternateExpansion) Expand(env Env) (string, error) {
	if _, ok := env.Get(e.Identifier); !ok {
		return "", nil
	}
	return e.Content.Expand(env)
}

// alternateNonEmptyExpansion returns the expansion of Content if the env is set and not
// empty, and an empty string otherwise. This is ${VAR:+word}.
type alternateNonEmptyExpansion struct {
	Identifier string
	Content    expression
}

func (e alternateNonEmptyExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Content.Identifiers()...)
}

func (e alternateNonEmptyExpansion) Expand(env Env) (string, error) {
	if val, _ := env.Get(e.Identifier); val == "" {
		return "", nil
	}
	return e.Content.Expand(env)
}

// lengthExpansion returns the number of characters in an env value, or 0 if it is unset. This is
// ${#VAR}.
type lengthExpansion struct {
	Identifier string
}

func (e lengthExpansion) Identifiers() []string {
	return []string{e.Identifier}
}

func (e lengthExpansion) Expand(env Env) (string, error) {
	val, _ := env.Get(e.Identifier)
	return strconv.Itoa(utf8.RuneCountInString(val)), nil
}

// requiredError returns the error for a required expansion. It uses the expansion of message, or
// defaultMessage if message expands to an empty string.
func requiredError(env Env, identifier string, message expression, defaultMessage string) error {
	msg, err := message.Expand(env)
	if err != nil {
		return err
	}
	if msg == "" {
		msg = defaultMessage
	}
	return fmt.Errorf("$%s: %s", identifier, msg)
}

// expression is a collection of either Text or Expansions
type expression []expressionItem

// Identifiers returns the variable names referenced by all expansions in the expression, in
// order. A name can occur more than once.
func (e expression) Identifiers() []string {
	identifiers := []string{}
	for _, item := range e {
		if item.Expansion != nil {
			identifiers = append(identifiers, item.Expansion.Identifiers()...)
		}
	}
	return identifiers
}

// Expand expands all expansions in the expression using variables from env, and joins the results
// with the text.
func (e expression) Expand(env Env) (string, error) {
	var buf strings.Builder

	for _, item := range e {
		if item.Expansion != nil {
			result, err := item.Expansion.Expand(env)
			if err != nil {
				return "", err
			}
			buf.WriteString(result)
		} else {
			buf.WriteString(item.Text)
		}
	}

	return buf.String(), nil
}

// expressionItem models either an Expansion or Text. Either/Or, never both.
type expressionItem struct {
	Text string
	// -- or --
	Expansion expansion
}

func (i expressionItem) String() string {
	if i.Expansion != nil {
		return fmt.Sprintf("%#v", i.Expansion)
	}
	return fmt.Sprintf("%q", i.Text)
}
