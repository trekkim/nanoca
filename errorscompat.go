package nanoca

import "errors"

// asType is a Go 1.25 compatible replacement for errors.AsType (added in Go 1.26).
func asType[T any](err error) (T, bool) {
	var t T
	ok := errors.As(err, &t)
	return t, ok
}
