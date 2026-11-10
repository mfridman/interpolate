package interpolate

import (
	"runtime"
	"strings"
)

// Env is an interface for getting environment variables by name and returning a boolean indicating
// whether the variable was found.
type Env interface {
	Get(key string) (string, bool)
}

// NewSliceEnv creates an Env from a slice of environment variables in the form "key=value".
//
// This can be used with [os.Environ] to create an Env.
func NewSliceEnv(env []string) Env {
	envMap := mapEnv{}
	for _, entry := range env {
		if key, value, ok := strings.Cut(entry, "="); ok {
			envMap[normalizeKeyName(key)] = value
		}
	}
	return envMap
}

// NewMapEnv creates an Env from a map of environment variables.
func NewMapEnv(env map[string]string) Env {
	envMap := mapEnv{}
	for k, v := range env {
		envMap[normalizeKeyName(k)] = v
	}
	return envMap
}

type mapEnv map[string]string

func (m mapEnv) Get(key string) (string, bool) {
	val, ok := m[normalizeKeyName(key)]
	return val, ok
}

// Windows isn't case sensitive for env
func normalizeKeyName(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}
