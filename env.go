package interpolate

import (
	"maps"
	"runtime"
	"slices"
	"strings"
)

// Env is a source of variable values.
type Env interface {
	// Lookup returns the value of the variable with the given name, and whether the variable is
	// set. A variable that is set can have an empty value.
	Lookup(name string) (value string, ok bool)
}

// LookupFunc is a function that is an [Env]. For example, LookupFunc(os.LookupEnv) reads the
// environment of the process.
type LookupFunc func(name string) (value string, ok bool)

// Lookup returns f(name).
func (f LookupFunc) Lookup(name string) (string, bool) {
	return f(name)
}

// NewMapEnv returns an Env with the variables in m. It copies m.
//
// On Windows, names are case insensitive. If keys in m differ only in case, the last key in
// sorted order wins.
func NewMapEnv(m map[string]string) Env {
	return newMapEnv(m, runtime.GOOS == "windows")
}

// NewListEnv returns an Env with the variables in pairs, which use the "NAME=value" form of
// [os.Environ]. Pairs without "=" are ignored. If a name occurs more than once, the last pair
// wins.
//
// On Windows, names are case insensitive.
func NewListEnv(pairs ...string) Env {
	return newListEnv(pairs, runtime.GOOS == "windows")
}

func newMapEnv(m map[string]string, foldCase bool) mapEnv {
	env := mapEnv{values: make(map[string]string, len(m)), foldCase: foldCase}
	// Sort the keys, so the result does not depend on map order when keys differ only in case.
	for _, name := range slices.Sorted(maps.Keys(m)) {
		env.values[env.key(name)] = m[name]
	}
	return env
}

func newListEnv(pairs []string, foldCase bool) mapEnv {
	env := mapEnv{values: make(map[string]string, len(pairs)), foldCase: foldCase}
	for _, pair := range pairs {
		if name, value, ok := strings.Cut(pair, "="); ok {
			env.values[env.key(name)] = value
		}
	}
	return env
}

// mapEnv is an Env with variables in a map. With foldCase, names are case insensitive.
type mapEnv struct {
	values   map[string]string
	foldCase bool
}

func (e mapEnv) Lookup(name string) (string, bool) {
	value, ok := e.values[e.key(name)]
	return value, ok
}

func (e mapEnv) key(name string) string {
	if e.foldCase {
		return strings.ToUpper(name)
	}
	return name
}
