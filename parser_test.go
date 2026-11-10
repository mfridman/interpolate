package interpolate_test

import (
	"reflect"
	"testing"

	"github.com/mfridman/interpolate"
)

func TestParser(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		input string
		want  interpolate.Expression
	}{
		{
			input: `Buildkite... ${HELLO_WORLD} ${ANOTHER_VAR:-🏖}`,
			want: interpolate.Expression{
				{Text: "Buildkite... "},
				{Expansion: interpolate.VariableExpansion{
					Identifier: "HELLO_WORLD",
				}},
				{Text: " "},
				{Expansion: interpolate.EmptyValueExpansion{
					Identifier: "ANOTHER_VAR",
					Content: interpolate.Expression{{
						Text: "🏖",
					}},
				}},
			},
		},
		{
			input: `${TEST1:- ${TEST2:-$TEST3}}`,
			want: interpolate.Expression{
				{Expansion: interpolate.EmptyValueExpansion{
					Identifier: "TEST1",
					Content: interpolate.Expression{
						{Text: " "},
						{Expansion: interpolate.EmptyValueExpansion{
							Identifier: "TEST2",
							Content: interpolate.Expression{
								{Expansion: interpolate.VariableExpansion{
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
			want: interpolate.Expression{
				{Expansion: interpolate.UnsetValueExpansion{
					Identifier: "HELLO_WORLD",
					Content: interpolate.Expression{{
						Text: "blah",
					}},
				}},
			},
		},
		{
			input: `\\${HELLO_WORLD-blah}`,
			want: interpolate.Expression{
				{Text: `\\`},
				{Expansion: interpolate.UnsetValueExpansion{
					Identifier: "HELLO_WORLD",
					Content: interpolate.Expression{{
						Text: "blah",
					}},
				}},
			},
		},
		{
			input: `\${HELLO_WORLD-blah}`,
			want: interpolate.Expression{
				{Text: `$`},
				{Text: `{HELLO_WORLD-blah}`},
			},
		},
		{
			input: `Test \\\${HELLO_WORLD-blah}`,
			want: interpolate.Expression{
				{Text: `Test `},
				{Text: `\\`},
				{Text: `$`},
				{Text: `{HELLO_WORLD-blah}`},
			},
		},
		{
			input: `${HELLO_WORLD:1}`,
			want: interpolate.Expression{
				{Expansion: interpolate.SubstringExpansion{
					Identifier: "HELLO_WORLD",
					Offset:     1,
				}},
			},
		},
		{
			input: `${HELLO_WORLD: -1}`,
			want: interpolate.Expression{
				{Expansion: interpolate.SubstringExpansion{
					Identifier: "HELLO_WORLD",
					Offset:     -1,
				}},
			},
		},
		{
			input: `${HELLO_WORLD:-1}`,
			want: interpolate.Expression{
				{Expansion: interpolate.EmptyValueExpansion{
					Identifier: "HELLO_WORLD",
					Content: interpolate.Expression{{
						Text: "1",
					}},
				}},
			},
		},
		{
			input: `${HELLO_WORLD:1:7}`,
			want: interpolate.Expression{
				{Expansion: interpolate.SubstringExpansion{
					Identifier: "HELLO_WORLD",
					Offset:     1,
					Length:     7,
					HasLength:  true,
				}},
			},
		},
		{
			input: `${HELLO_WORLD:1:-7}`,
			want: interpolate.Expression{
				{Expansion: interpolate.SubstringExpansion{
					Identifier: "HELLO_WORLD",
					Offset:     1,
					Length:     -7,
					HasLength:  true,
				}},
			},
		},
		{
			input: `${HELLO_WORLD?Required}`,
			want: interpolate.Expression{
				{Expansion: interpolate.RequiredExpansion{
					Identifier: "HELLO_WORLD",
					Message: interpolate.Expression{
						{Text: "Required"},
					},
				}},
			},
		},
		{
			input: `${HELLO_WORLD:?Required}`,
			want: interpolate.Expression{
				{Expansion: interpolate.RequiredNonEmptyExpansion{
					Identifier: "HELLO_WORLD",
					Message: interpolate.Expression{
						{Text: "Required"},
					},
				}},
			},
		},
		{
			input: `${HELLO_WORLD+alt}`,
			want: interpolate.Expression{
				{Expansion: interpolate.AlternateExpansion{
					Identifier: "HELLO_WORLD",
					Content: interpolate.Expression{
						{Text: "alt"},
					},
				}},
			},
		},
		{
			input: `${HELLO_WORLD:+alt}`,
			want: interpolate.Expression{
				{Expansion: interpolate.AlternateNonEmptyExpansion{
					Identifier: "HELLO_WORLD",
					Content: interpolate.Expression{
						{Text: "alt"},
					},
				}},
			},
		},
		{
			input: `${#HELLO_WORLD}`,
			want: interpolate.Expression{
				{Expansion: interpolate.LengthExpansion{
					Identifier: "HELLO_WORLD",
				}},
			},
		},
		{
			input: `$`,
			want: interpolate.Expression{
				{Text: `$`},
			},
		},
		{
			input: `\`,
			want: interpolate.Expression{
				{Text: `\`},
			},
		},
		{
			input: "price: $5 \xff",
			want: interpolate.Expression{
				{Text: "price: "},
				{Text: "$5 \xff"},
			},
		},
		{
			input: `${HELLO_WORLD::7}`,
			want: interpolate.Expression{
				{Expansion: interpolate.SubstringExpansion{
					Identifier: "HELLO_WORLD",
					Length:     7,
					HasLength:  true,
				}},
			},
		},
		{
			input: `$(echo hello world)`,
			want: interpolate.Expression{
				{Text: `$(echo hello world)`},
			},
		},
	} {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()

			got, err := interpolate.NewParser(tc.input).Parse()
			if err != nil {
				t.Fatalf("NewParser(%q).Parse() error = %v", tc.input, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("NewParser(%q).Parse() = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
