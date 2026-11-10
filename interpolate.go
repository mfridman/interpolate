package interpolate

// Expand returns s with each parameter expansion replaced by its value, and each escape such as
// $$ replaced by its text. Values come from env. A nil env has no variables.
//
// Expand returns an error if s is not valid, or if a required expansion such as ${NAME?message}
// fails.
func Expand(s string, env Env) (string, error) {
	if env == nil {
		env = NewSliceEnv(nil)
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
