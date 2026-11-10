package interpolate

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Interpolate takes a set of environment and interpolates it into the provided string using shell
// script expansions
func Interpolate(env Env, str string) (string, error) {
	if env == nil {
		env = NewSliceEnv(nil)
	}
	expr, err := NewParser(str).Parse()
	if err != nil {
		return "", err
	}
	return expr.Expand(env)
}

// Identifiers parses the identifiers from any expansions in the provided string
func Identifiers(str string) ([]string, error) {
	expr, err := NewParser(str).Parse()
	if err != nil {
		return nil, err
	}
	return expr.Identifiers(), nil
}

// An expansion is something that takes in ENV and returns a string or an error
type Expansion interface {
	// Expand expands the expansion using variables from env.
	Expand(env Env) (string, error)

	// Identifiers returns any variable names referenced within the expansion.
	Identifiers() []string
}

// VariableExpansion represents either $VAR or ${VAR}, our simplest expansion
type VariableExpansion struct {
	Identifier string
}

func (e VariableExpansion) Identifiers() []string {
	return []string{e.Identifier}
}

func (e VariableExpansion) Expand(env Env) (string, error) {
	val, _ := env.Get(e.Identifier)
	return val, nil
}

// EmptyValueExpansion returns either the value of an env, or a default value if it's unset or null
type EmptyValueExpansion struct {
	Identifier string
	Content    Expression
}

func (e EmptyValueExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Content.Identifiers()...)
}

func (e EmptyValueExpansion) Expand(env Env) (string, error) {
	val, _ := env.Get(e.Identifier)
	if val == "" {
		return e.Content.Expand(env)
	}
	return val, nil
}

// UnsetValueExpansion returns either the value of an env, or a default value if it's unset
type UnsetValueExpansion struct {
	Identifier string
	Content    Expression
}

func (e UnsetValueExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Content.Identifiers()...)
}

func (e UnsetValueExpansion) Expand(env Env) (string, error) {
	val, ok := env.Get(e.Identifier)
	if !ok {
		return e.Content.Expand(env)
	}
	return val, nil
}

// SubstringExpansion returns a substring (or slice) of the env. This is ${VAR:offset} or
// ${VAR:offset:length}.
type SubstringExpansion struct {
	Identifier string
	Offset     int
	Length     int
	HasLength  bool
}

func (e SubstringExpansion) Identifiers() []string {
	return []string{e.Identifier}
}

func (e SubstringExpansion) Expand(env Env) (string, error) {
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

// RequiredExpansion returns an env value, or an error if it is unset. This is ${VAR?message}.
type RequiredExpansion struct {
	Identifier string
	Message    Expression
}

func (e RequiredExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Message.Identifiers()...)
}

func (e RequiredExpansion) Expand(env Env) (string, error) {
	val, ok := env.Get(e.Identifier)
	if !ok {
		return "", requiredError(env, e.Identifier, e.Message, "not set")
	}
	return val, nil
}

// RequiredNonEmptyExpansion returns an env value, or an error if it is unset or empty. This is
// ${VAR:?message}.
type RequiredNonEmptyExpansion struct {
	Identifier string
	Message    Expression
}

func (e RequiredNonEmptyExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Message.Identifiers()...)
}

func (e RequiredNonEmptyExpansion) Expand(env Env) (string, error) {
	val, _ := env.Get(e.Identifier)
	if val == "" {
		return "", requiredError(env, e.Identifier, e.Message, "empty or not set")
	}
	return val, nil
}

// AlternateExpansion returns the expansion of Content if the env is set, and an empty string
// otherwise. This is ${VAR+word}.
type AlternateExpansion struct {
	Identifier string
	Content    Expression
}

func (e AlternateExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Content.Identifiers()...)
}

func (e AlternateExpansion) Expand(env Env) (string, error) {
	if _, ok := env.Get(e.Identifier); !ok {
		return "", nil
	}
	return e.Content.Expand(env)
}

// AlternateNonEmptyExpansion returns the expansion of Content if the env is set and not
// empty, and an empty string otherwise. This is ${VAR:+word}.
type AlternateNonEmptyExpansion struct {
	Identifier string
	Content    Expression
}

func (e AlternateNonEmptyExpansion) Identifiers() []string {
	return append([]string{e.Identifier}, e.Content.Identifiers()...)
}

func (e AlternateNonEmptyExpansion) Expand(env Env) (string, error) {
	if val, _ := env.Get(e.Identifier); val == "" {
		return "", nil
	}
	return e.Content.Expand(env)
}

// LengthExpansion returns the number of characters in an env value, or 0 if it is unset. This is
// ${#VAR}.
type LengthExpansion struct {
	Identifier string
}

func (e LengthExpansion) Identifiers() []string {
	return []string{e.Identifier}
}

func (e LengthExpansion) Expand(env Env) (string, error) {
	val, _ := env.Get(e.Identifier)
	return strconv.Itoa(utf8.RuneCountInString(val)), nil
}

// requiredError returns the error for a required expansion. It uses the expansion of message, or
// defaultMessage if message expands to an empty string.
func requiredError(env Env, identifier string, message Expression, defaultMessage string) error {
	msg, err := message.Expand(env)
	if err != nil {
		return err
	}
	if msg == "" {
		msg = defaultMessage
	}
	return fmt.Errorf("$%s: %s", identifier, msg)
}

// Expression is a collection of either Text or Expansions
type Expression []ExpressionItem

// Identifiers returns the variable names referenced by all expansions in the expression, in
// order. A name can occur more than once.
func (e Expression) Identifiers() []string {
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
func (e Expression) Expand(env Env) (string, error) {
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

// ExpressionItem models either an Expansion or Text. Either/Or, never both.
type ExpressionItem struct {
	Text string
	// -- or --
	Expansion Expansion
}

func (i ExpressionItem) String() string {
	if i.Expansion != nil {
		return fmt.Sprintf("%#v", i.Expansion)
	}
	return fmt.Sprintf("%q", i.Text)
}
