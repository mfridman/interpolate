package interpolate

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// This is a recursive descent parser for our grammar. Because it can contain nested expressions
// like ${LLAMAS:-${ROCK:-true}} we can't use regular expressions. The simplest possible alternative
// is a recursive parser like this. It parses a chunk and then calls a function to parse that
// further and so on and so forth. It results in a tree of objects that represent the things we've
// parsed (an AST). This means that the logic for how expansions work lives in those objects, and
// the logic for how we go from plain text to parsed objects lives here.
//
// To keep things simple, we do our "lexing" or "scanning" just as a few functions at the end of the
// file rather than as a dedicated lexer that emits tokens. This matches the simplicity of the
// format we are parsing relatively well
//
// Below is an EBNF grammar for the language. The parser was built by basically turning this into
// functions and structs named the same reading the string bite by bite (peekRune and nextRune)

/*
EscapedBackslash  = "\\"
EscapedDollar     = ( "\$" | "$$")
Identifier        = ( letter | "_" ) { letter | digit | "_" }
Expansion         = "$" ( Identifier | Brace )
LiteralDollar     = "$" (not followed by "{" or a letter or "_")
Brace             = "{" ( Length | Identifier [ Operation ] ) "}"
EscapedBrace      = "\}" (only in the word of a brace expansion)
Text              = { EscapedBackslash | EscapedDollar | EscapedBrace | LiteralDollar | all characters except "$" }
Expression        = { Text | Expansion }
Length            = "#" Identifier
EmptyValue        = ":-" { Expression }
UnsetValue        = "-" { Expression }
Substring         = ":" number [ ":" number ]
RequiredNonEmpty  = ":?" { Expression }
Required          = "?" { Expression }
AlternateNonEmpty = ":+" { Expression }
Alternate         = "+" { Expression }
Operation         = EmptyValue | UnsetValue | Substring | RequiredNonEmpty | Required | AlternateNonEmpty | Alternate
*/

const (
	eof = -1

	// maxNestingDepth is the maximum number of brace expansions that can be nested inside each
	// other, as in ${A:-${B:-${C}}}. The parser is recursive, so the limit protects against stack
	// exhaustion from untrusted input.
	maxNestingDepth = 1000
)

// Parser takes a string and parses out a tree of structs that represent text and Expansions
type Parser struct {
	input string // the string we are scanning
	pos   int    // the current position
	depth int    // the current number of nested brace expansions
}

// NewParser returns a new instance of a Parser
func NewParser(str string) *Parser {
	return &Parser{
		input: str,
		pos:   0,
	}
}

// Parse expansions out of the internal text and return them as a tree of Expressions
func (p *Parser) Parse() (Expression, error) {
	return p.parseExpression()
}

func (p *Parser) parseExpression(stop ...rune) (Expression, error) {
	var expr Expression
	var stopStr = string(stop)

	for {
		c := p.peekRune()
		if c == eof || strings.ContainsRune(stopStr, c) {
			break
		}

		// check for our escaped characters first, as we assume nothing subsequently is escaped
		if strings.HasPrefix(p.input[p.pos:], `\\`) {
			p.pos += 2
			expr = append(expr, ExpressionItem{Text: `\\`})
			continue
		} else if stopStr != "" && strings.HasPrefix(p.input[p.pos:], `\}`) {
			// Inside a brace expansion, an escaped closing brace is text and does not end the
			// expansion, the same as bash.
			p.pos += 2
			expr = append(expr, ExpressionItem{Text: `}`})
			continue
		} else if strings.HasPrefix(p.input[p.pos:], `\$`) || strings.HasPrefix(p.input[p.pos:], `$$`) {
			p.pos += 2
			expr = append(expr, ExpressionItem{Text: `$`})
			continue
		}

		// A dollar sign starts an expansion only when a brace or a name follows it. Anything else,
		// such as "$1", "$ ", "$(" or a trailing "$", is literal text.
		if c == '$' && p.isExpansionStart() {
			expansion, err := p.parseExpansion()
			if err != nil {
				return nil, err
			}
			expr = append(expr, ExpressionItem{Expansion: expansion})
			continue
		}

		// Nibble a character, otherwise if it's a \ or a $ we can loop. Slice the input instead of
		// re-encoding runes, so invalid UTF-8 bytes pass through unchanged.
		start := p.pos
		_ = p.nextRune()

		// Scan as much as we can into text
		p.scanUntil(func(r rune) bool {
			return (r == '$' || r == '\\' || strings.ContainsRune(stopStr, r))
		})

		expr = append(expr, ExpressionItem{Text: p.input[start:p.pos]})
	}

	return expr, nil
}

func (p *Parser) parseExpansion() (Expansion, error) {
	if c := p.nextRune(); c != '$' {
		return nil, fmt.Errorf("expected expansion to start with $, got %s", describeRune(c))
	}

	// if we have an open brace, this is a brace expansion
	if c := p.peekRune(); c == '{' {
		return p.parseBraceExpansion()
	}

	identifier, err := p.scanIdentifier()
	if err != nil {
		return nil, err
	}

	return VariableExpansion{Identifier: identifier}, nil
}

func (p *Parser) parseBraceExpansion() (Expansion, error) {
	if c := p.nextRune(); c != '{' {
		return nil, fmt.Errorf("expected brace expansion to start with {, got %s", describeRune(c))
	}

	p.depth++
	defer func() { p.depth-- }()
	if p.depth > maxNestingDepth {
		return nil, fmt.Errorf("brace expansions nested deeper than %d levels", maxNestingDepth)
	}

	if c := p.peekRune(); c == '}' {
		return nil, errors.New("empty expansion ${}, expected a variable name")
	}

	// ${#parameter} is the length of the value, so it has a prefix and no operator.
	if c := p.peekRune(); c == '#' {
		_ = p.nextRune()
		return p.parseLengthExpansion()
	}

	identifier, err := p.scanIdentifier()
	if err != nil {
		return nil, err
	}

	if c := p.peekRune(); c == '}' {
		_ = p.nextRune()
		return VariableExpansion{Identifier: identifier}, nil
	}

	operator, err := p.scanOperator()
	if err != nil {
		return nil, err
	}

	var expansion Expansion
	if operator == ":" {
		expansion, err = p.parseSubstringExpansion(identifier)
		if err != nil {
			return nil, err
		}
	} else {
		// All other operators take a word: an expression (text and expansions) up until the end
		// of the brace.
		word, err := p.parseExpression('}')
		if err != nil {
			return nil, err
		}
		switch operator {
		case ":-":
			expansion = EmptyValueExpansion{Identifier: identifier, Content: word}
		case "-":
			expansion = UnsetValueExpansion{Identifier: identifier, Content: word}
		case ":?":
			expansion = RequiredNonEmptyExpansion{Identifier: identifier, Message: word}
		case "?":
			expansion = RequiredExpansion{Identifier: identifier, Message: word}
		case ":+":
			expansion = AlternateNonEmptyExpansion{Identifier: identifier, Content: word}
		case "+":
			expansion = AlternateExpansion{Identifier: identifier, Content: word}
		}
	}

	if c := p.nextRune(); c != '}' {
		return nil, fmt.Errorf("expected brace expansion to end with }, got %s", describeRune(c))
	}

	return expansion, nil
}

// scanOperator scans the operator after the identifier in a brace expansion. A colon on its own
// is a substring operator. A colon followed by "-", "?" or "+" also treats an empty value as
// unset.
func (p *Parser) scanOperator() (string, error) {
	switch op1 := p.nextRune(); op1 {
	case ':':
		if op2 := p.peekRune(); op2 == '-' || op2 == '?' || op2 == '+' {
			_ = p.nextRune()
			return string([]rune{op1, op2}), nil
		}
		return ":", nil
	case '-', '?', '+':
		return string(op1), nil
	default:
		return "", fmt.Errorf("expected an operator, got %s", describeRune(op1))
	}
}

func (p *Parser) parseLengthExpansion() (Expansion, error) {
	identifier, err := p.scanIdentifier()
	if err != nil {
		return nil, err
	}

	if c := p.nextRune(); c != '}' {
		return nil, fmt.Errorf("expected length expansion to end with }, got %s", describeRune(c))
	}

	return LengthExpansion{Identifier: identifier}, nil
}

func (p *Parser) parseSubstringExpansion(identifier string) (Expansion, error) {
	offset := p.scanUntil(func(r rune) bool {
		return r == ':' || r == '}'
	})

	// Bash rejects ${VAR:}, but treats a blank offset as 0 when it is followed by a length, as in
	// ${VAR::2}, or when it has only spaces, as in ${VAR: }.
	hasLength := p.peekRune() == ':'
	if offset == "" && !hasLength {
		return nil, fmt.Errorf("expected substring offset, got %s", describeRune(p.peekRune()))
	}

	offsetInt, err := parseSubstringNumber(offset)
	if err != nil {
		return nil, fmt.Errorf("parse substring offset: %w", err)
	}

	if !hasLength {
		return SubstringExpansion{Identifier: identifier, Offset: offsetInt}, nil
	}

	_ = p.nextRune()
	length := p.scanUntil(func(r rune) bool {
		return r == '}'
	})

	lengthInt, err := parseSubstringNumber(length)
	if err != nil {
		return nil, fmt.Errorf("parse substring length: %w", err)
	}

	return SubstringExpansion{Identifier: identifier, Offset: offsetInt, Length: lengthInt, HasLength: true}, nil
}

// parseSubstringNumber parses a substring offset or length. A blank value is 0, the same as bash.
func parseSubstringNumber(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

func (p *Parser) scanUntil(f func(rune) bool) string {
	start := p.pos
	for p.pos < len(p.input) {
		c, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if f(c) {
			break
		}
		p.pos += size
	}
	return p.input[start:p.pos]
}

func (p *Parser) scanIdentifier() (string, error) {
	if c := p.peekRune(); !isIdentifierStart(c) {
		return "", fmt.Errorf("expected identifier to start with a letter or underscore, got %s", describeRune(c))
	}
	notIdentifierChar := func(r rune) bool {
		return !isIdentifierChar(r)
	}
	return p.scanUntil(notIdentifierChar), nil
}

// isExpansionStart reports whether the dollar sign at the current position starts an expansion.
func (p *Parser) isExpansionStart() bool {
	next, _ := utf8.DecodeRuneInString(p.input[p.pos+1:])
	return next == '{' || isIdentifierStart(next)
}

// isIdentifierStart reports whether r can start a POSIX name: an ASCII letter or an underscore.
func isIdentifierStart(r rune) bool {
	return r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z')
}

// isIdentifierChar reports whether r can continue a POSIX name: an ASCII letter, an ASCII digit,
// or an underscore.
func isIdentifierChar(r rune) bool {
	return isIdentifierStart(r) || ('0' <= r && r <= '9')
}

func (p *Parser) nextRune() rune {
	if p.pos >= len(p.input) {
		return eof
	}
	c, size := utf8.DecodeRuneInString(p.input[p.pos:])
	p.pos += size
	return c
}

func (p *Parser) peekRune() rune {
	if p.pos >= len(p.input) {
		return eof
	}
	c, _ := utf8.DecodeRuneInString(p.input[p.pos:])
	return c
}

// describeRune returns a description of r for use in error messages.
func describeRune(r rune) string {
	if r == eof {
		return "end of input"
	}
	return fmt.Sprintf("%q", r)
}
