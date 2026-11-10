package interpolate

// Interpolate takes a set of environment and interpolates it into the provided string using shell
// script expansions
func Interpolate(env Env, str string) (string, error) {
	if env == nil {
		env = NewSliceEnv(nil)
	}
	expr, err := newParser(str).Parse()
	if err != nil {
		return "", err
	}
	return expr.Expand(env)
}

// Identifiers parses the identifiers from any expansions in the provided string
func Identifiers(str string) ([]string, error) {
	expr, err := newParser(str).Parse()
	if err != nil {
		return nil, err
	}
	return expr.Identifiers(), nil
}
