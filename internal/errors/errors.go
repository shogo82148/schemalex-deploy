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
	if ignore, ok := errors.AsType[ignorabler](err); ok {
		return ignore.Ignorable()
	}
	return false
}
