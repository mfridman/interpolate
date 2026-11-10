package interpolate

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// expandContext holds the values and settings for one call to Expand.
type expandContext struct {
	env     Env
	noUnset bool
}

// value returns the value of name for a plain reference, such as $NAME or ${NAME:1}. With
// noUnset, an unset variable is an error.
func (c *expandContext) value(name string) (string, error) {
	value, ok := c.env.Lookup(name)
	if !ok && c.noUnset {
		return "", &UnsetVariableError{Name: name, Message: "not set"}
	}
	return value, nil
}

// unsetError returns the error for a required expansion. It uses the expansion of message, or
// defaultMessage if message expands to an empty string.
func (c *expandContext) unsetError(name string, message expression, defaultMessage string) error {
	msg, err := message.expand(c)
	if err != nil {
		return err
	}
	if msg == "" {
		msg = defaultMessage
	}
	return &UnsetVariableError{Name: name, Message: msg}
}

// expression is a list of text and expansions.
type expression []expressionItem

// expand expands all expansions in the expression, and joins the results with the text.
func (e expression) expand(c *expandContext) (string, error) {
	var buf strings.Builder
	for _, item := range e {
		if item.Expansion == nil {
			buf.WriteString(item.Text)
			continue
		}
		result, err := item.Expansion.expand(c)
		if err != nil {
			return "", err
		}
		buf.WriteString(result)
	}
	return buf.String(), nil
}

// variables returns the variable names referenced by all expansions in the expression, in order.
// A name can occur more than once.
func (e expression) variables() []string {
	var names []string
	for _, item := range e {
		if item.Expansion != nil {
			names = append(names, item.Expansion.variables()...)
		}
	}
	return names
}

// expressionItem is either text or an expansion, never both.
type expressionItem struct {
	Text      string
	Expansion expansion
}

func (i expressionItem) String() string {
	if i.Expansion != nil {
		return fmt.Sprintf("%#v", i.Expansion)
	}
	return fmt.Sprintf("%q", i.Text)
}

// expansion is one parameter expansion, such as $NAME or ${NAME:-word}.
type expansion interface {
	// expand returns the value of the expansion.
	expand(c *expandContext) (string, error)

	// variables returns the variable names referenced by the expansion, including the names in
	// its word or message.
	variables() []string
}

// variableExpansion is $NAME or ${NAME}.
type variableExpansion struct {
	Identifier string
}

func (e variableExpansion) variables() []string {
	return []string{e.Identifier}
}

func (e variableExpansion) expand(c *expandContext) (string, error) {
	return c.value(e.Identifier)
}

// emptyValueExpansion is ${NAME:-word}: the value, or word if the variable is unset or empty.
type emptyValueExpansion struct {
	Identifier string
	Content    expression
}

func (e emptyValueExpansion) variables() []string {
	return append([]string{e.Identifier}, e.Content.variables()...)
}

func (e emptyValueExpansion) expand(c *expandContext) (string, error) {
	if value, _ := c.env.Lookup(e.Identifier); value != "" {
		return value, nil
	}
	return e.Content.expand(c)
}

// unsetValueExpansion is ${NAME-word}: the value, or word if the variable is unset.
type unsetValueExpansion struct {
	Identifier string
	Content    expression
}

func (e unsetValueExpansion) variables() []string {
	return append([]string{e.Identifier}, e.Content.variables()...)
}

func (e unsetValueExpansion) expand(c *expandContext) (string, error) {
	if value, ok := c.env.Lookup(e.Identifier); ok {
		return value, nil
	}
	return e.Content.expand(c)
}

// requiredExpansion is ${NAME?message}: the value, or an error if the variable is unset.
type requiredExpansion struct {
	Identifier string
	Message    expression
}

func (e requiredExpansion) variables() []string {
	return append([]string{e.Identifier}, e.Message.variables()...)
}

func (e requiredExpansion) expand(c *expandContext) (string, error) {
	value, ok := c.env.Lookup(e.Identifier)
	if !ok {
		return "", c.unsetError(e.Identifier, e.Message, "not set")
	}
	return value, nil
}

// requiredNonEmptyExpansion is ${NAME:?message}: the value, or an error if the variable is unset
// or empty.
type requiredNonEmptyExpansion struct {
	Identifier string
	Message    expression
}

func (e requiredNonEmptyExpansion) variables() []string {
	return append([]string{e.Identifier}, e.Message.variables()...)
}

func (e requiredNonEmptyExpansion) expand(c *expandContext) (string, error) {
	value, _ := c.env.Lookup(e.Identifier)
	if value == "" {
		return "", c.unsetError(e.Identifier, e.Message, "empty or not set")
	}
	return value, nil
}

// alternateExpansion is ${NAME+word}: word if the variable is set, otherwise an empty string.
type alternateExpansion struct {
	Identifier string
	Content    expression
}

func (e alternateExpansion) variables() []string {
	return append([]string{e.Identifier}, e.Content.variables()...)
}

func (e alternateExpansion) expand(c *expandContext) (string, error) {
	if _, ok := c.env.Lookup(e.Identifier); !ok {
		return "", nil
	}
	return e.Content.expand(c)
}

// alternateNonEmptyExpansion is ${NAME:+word}: word if the variable is set and not empty,
// otherwise an empty string.
type alternateNonEmptyExpansion struct {
	Identifier string
	Content    expression
}

func (e alternateNonEmptyExpansion) variables() []string {
	return append([]string{e.Identifier}, e.Content.variables()...)
}

func (e alternateNonEmptyExpansion) expand(c *expandContext) (string, error) {
	if value, _ := c.env.Lookup(e.Identifier); value == "" {
		return "", nil
	}
	return e.Content.expand(c)
}

// lengthExpansion is ${#NAME}: the number of characters in the value.
type lengthExpansion struct {
	Identifier string
}

func (e lengthExpansion) variables() []string {
	return []string{e.Identifier}
}

func (e lengthExpansion) expand(c *expandContext) (string, error) {
	value, err := c.value(e.Identifier)
	if err != nil {
		return "", err
	}
	return strconv.Itoa(utf8.RuneCountInString(value)), nil
}

// substringExpansion is ${NAME:offset} or ${NAME:offset:length}.
type substringExpansion struct {
	Identifier string
	Offset     int
	Length     int
	HasLength  bool
}

func (e substringExpansion) variables() []string {
	return []string{e.Identifier}
}

func (e substringExpansion) expand(c *expandContext) (string, error) {
	value, err := c.value(e.Identifier)
	if err != nil {
		return "", err
	}
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
