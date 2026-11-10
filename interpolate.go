package interpolate

// Expand returns s with each parameter expansion replaced by its value, and each escape such as
// $$ replaced by its text. Values come from env. A nil env has no variables.
//
// Expand returns an error if s is not valid, and an [*UnsetVariableError] if a variable that must
// have a value does not have one.
func Expand(s string, env Env) (string, error) {
	if env == nil {
		env = mapEnv{}
	}
	expr, err := newParser(s).Parse()
	if err != nil {
		return "", err
	}
	return expr.expand(env)
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

// UnsetVariableError is the error for a variable that must have a value but does not. It comes
// from ${NAME?message} and ${NAME:?message}.
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
