package interpolate

// Expand returns s with each parameter expansion replaced by its value, and each escape such as
// $$ replaced by its text. Values come from env. A nil env has no variables.
//
// Expand returns an error if s is not valid, and an [*UnsetVariableError] if a variable that must
// have a value does not have one.
func Expand(s string, env Env, opts ...Option) (string, error) {
	var cfg config
	for _, opt := range opts {
		opt.apply(&cfg)
	}
	if env == nil {
		env = mapEnv{}
	}
	expr, err := newParser(s).Parse()
	if err != nil {
		return "", err
	}
	return expr.expand(&expandContext{env: env, noUnset: cfg.noUnset})
}

// Variables returns the names of the variables that s refers to, in the order of their first use,
// with no repeats. It includes the variables in the words and messages of expansions, such as
// FALLBACK in ${NAME:-$FALLBACK}, and does not include escaped references such as $$NAME.
//
// Variables returns an error if s is not valid.
func Variables(s string) ([]string, error) {
	expr, err := newParser(s).Parse()
	if err != nil {
		return nil, err
	}
	names := []string{}
	seen := make(map[string]bool)
	for _, name := range expr.variables() {
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names, nil
}

// Option configures [Expand].
type Option interface {
	apply(*config)
}

// WithNoUnset makes a reference to an unset variable an error, the same as set -u (nounset) in
// bash. It applies to $NAME, ${NAME}, ${#NAME}, ${NAME:offset} and ${NAME:offset:length}.
// Expansions that handle an unset variable, such as ${NAME:-word} and ${NAME+word}, do not
// change. The error is an [*UnsetVariableError] with the message "not set".
func WithNoUnset() Option {
	return noUnsetOption{}
}

// UnsetVariableError is the error for a variable that must have a value but does not. It comes
// from ${NAME?message}, from ${NAME:?message}, and from a reference to an unset variable with
// [WithNoUnset].
type UnsetVariableError struct {
	// Name is the name of the variable.
	Name string
	// Message is the expanded message from the expansion, or a default message such as "not set".
	Message string
}

// Error returns the error in the form "$NAME: message".
func (e *UnsetVariableError) Error() string {
	return "$" + e.Name + ": " + e.Message
}

type config struct {
	noUnset bool
}

type noUnsetOption struct{}

func (noUnsetOption) apply(cfg *config) {
	cfg.noUnset = true
}
