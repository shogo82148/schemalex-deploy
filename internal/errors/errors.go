package errors

import (
	"errors"
)

type ignorableErr struct {
	error
}

type ignorabler interface {
	Ignorable() bool
}

func (e ignorableErr) Error() string {
	if e.error != nil {
		return e.error.Error() + " (ignorable)"
	}
	return "(ignorable)"
}

func (e ignorableErr) Ignorable() bool {
	return true
}

func Ignorable(err error) error {
	return ignorableErr{error: err}
}

func IsIgnorable(err error) bool {
	if err == nil {
		return false
	}
	var ignore ignorabler
	if errors.As(err, &ignore) {
		return ignore.Ignorable()
	}
	return false
}
