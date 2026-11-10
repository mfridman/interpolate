package interpolate

import (
	"reflect"
	"testing"
)

func TestParserReuse(t *testing.T) {
	t.Parallel()

	parser := newParser("a $B c")
	first, err := parser.Parse()
	if err != nil {
		t.Fatalf("first Parse() error = %v", err)
	}
	second, err := parser.Parse()
	if err != nil {
		t.Fatalf("second Parse() error = %v", err)
	}
	if len(first) != 3 || !reflect.DeepEqual(first, second) {
		t.Errorf("Parse() = %v, then %v, want the same 3 items twice", first, second)
	}

	invalid := newParser("${FOO")
	for range 2 {
		if _, err := invalid.Parse(); err == nil {
			t.Error(`NewParser("${FOO").Parse() error = nil, want error on each call`)
		}
	}
}

func TestExpressionItemString(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		item expressionItem
		want string
	}{
		{item: expressionItem{Text: "a b"}, want: `"a b"`},
		{
			item: expressionItem{Expansion: variableExpansion{Identifier: "B"}},
			want: `interpolate.variableExpansion{Identifier:"B"}`,
		},
	} {
		if got := tc.item.String(); got != tc.want {
			t.Errorf("String() = %s, want %s", got, tc.want)
		}
	}
}

func TestParser(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		want  expression
	}{
		{
			input: `Buildkite... ${HELLO_WORLD} ${ANOTHER_VAR:-🏖}`,
			want: expression{
				{Text: "Buildkite... "},
				{Expansion: variableExpansion{
					Identifier: "HELLO_WORLD",
				}},
				{Text: " "},
				{Expansion: emptyValueExpansion{
					Identifier: "ANOTHER_VAR",
					Content: expression{{
						Text: "🏖",
					}},
				}},
			},
		},
		{
			input: `${TEST1:- ${TEST2:-$TEST3}}`,
			want: expression{
				{Expansion: emptyValueExpansion{
					Identifier: "TEST1",
					Content: expression{
						{Text: " "},
						{Expansion: emptyValueExpansion{
							Identifier: "TEST2",
							Content: expression{
								{Expansion: variableExpansion{
									Identifier: "TEST3",
								}},
							},
						}},
					},
				}},
			},
		},
		{
			input: `${HELLO_WORLD-blah}`,
			want: expression{
				{Expansion: unsetValueExpansion{
					Identifier: "HELLO_WORLD",
					Content: expression{{
						Text: "blah",
					}},
				}},
			},
		},
		{
			input: `\\${HELLO_WORLD-blah}`,
			want: expression{
				{Text: `\\`},
				{Expansion: unsetValueExpansion{
					Identifier: "HELLO_WORLD",
					Content: expression{{
						Text: "blah",
					}},
				}},
			},
		},
		{
			input: `\${HELLO_WORLD-blah}`,
			want: expression{
				{Text: `$`},
				{Text: `{HELLO_WORLD-blah}`},
			},
		},
		{
			input: `Test \\\${HELLO_WORLD-blah}`,
			want: expression{
				{Text: `Test `},
				{Text: `\\`},
				{Text: `$`},
				{Text: `{HELLO_WORLD-blah}`},
			},
		},
		{
			input: `${HELLO_WORLD:1}`,
			want: expression{
				{Expansion: substringExpansion{
					Identifier: "HELLO_WORLD",
					Offset:     1,
				}},
			},
		},
		{
			input: `${HELLO_WORLD: -1}`,
			want: expression{
				{Expansion: substringExpansion{
					Identifier: "HELLO_WORLD",
					Offset:     -1,
				}},
			},
		},
		{
			input: `${HELLO_WORLD:-1}`,
			want: expression{
				{Expansion: emptyValueExpansion{
					Identifier: "HELLO_WORLD",
					Content: expression{{
						Text: "1",
					}},
				}},
			},
		},
		{
			input: `${HELLO_WORLD:1:7}`,
			want: expression{
				{Expansion: substringExpansion{
					Identifier: "HELLO_WORLD",
					Offset:     1,
					Length:     7,
					HasLength:  true,
				}},
			},
		},
		{
			input: `${HELLO_WORLD:1:-7}`,
			want: expression{
				{Expansion: substringExpansion{
					Identifier: "HELLO_WORLD",
					Offset:     1,
					Length:     -7,
					HasLength:  true,
				}},
			},
		},
		{
			input: `${HELLO_WORLD?Required}`,
			want: expression{
				{Expansion: requiredExpansion{
					Identifier: "HELLO_WORLD",
					Message: expression{
						{Text: "Required"},
					},
				}},
			},
		},
		{
			input: `${HELLO_WORLD:?Required}`,
			want: expression{
				{Expansion: requiredNonEmptyExpansion{
					Identifier: "HELLO_WORLD",
					Message: expression{
						{Text: "Required"},
					},
				}},
			},
		},
		{
			input: `${HELLO_WORLD+alt}`,
			want: expression{
				{Expansion: alternateExpansion{
					Identifier: "HELLO_WORLD",
					Content: expression{
						{Text: "alt"},
					},
				}},
			},
		},
		{
			input: `${HELLO_WORLD:+alt}`,
			want: expression{
				{Expansion: alternateNonEmptyExpansion{
					Identifier: "HELLO_WORLD",
					Content: expression{
						{Text: "alt"},
					},
				}},
			},
		},
		{
			input: `${#HELLO_WORLD}`,
			want: expression{
				{Expansion: lengthExpansion{
					Identifier: "HELLO_WORLD",
				}},
			},
		},
		{
			input: `$`,
			want: expression{
				{Text: `$`},
			},
		},
		{
			input: `\`,
			want: expression{
				{Text: `\`},
			},
		},
		{
			input: "price: $5 \xff",
			want: expression{
				{Text: "price: "},
				{Text: "$5 \xff"},
			},
		},
		{
			input: `${HELLO_WORLD::7}`,
			want: expression{
				{Expansion: substringExpansion{
					Identifier: "HELLO_WORLD",
					Length:     7,
					HasLength:  true,
				}},
			},
		},
		{
			input: `$(echo hello world)`,
			want: expression{
				{Text: `$(echo hello world)`},
			},
		},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			got, err := newParser(tc.input).Parse()
			if err != nil {
				t.Fatalf("NewParser(%q).Parse() error = %v", tc.input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NewParser(%q).Parse() = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
