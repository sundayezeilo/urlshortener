// Package errx provides application error kinds that map cleanly to HTTP status codes.
// While some kinds (Unauthorized, Forbidden) are HTTP-centric, they're included for
// convenience since this is primarily a web application.

package errx

import (
	"errors"
	"fmt"
)

type Kind uint8

const (
	Unknown Kind = iota
	NotFound
	Conflict
	Invalid
	Unauthorized
	Forbidden
	Unavailable
	Internal
)

type Error struct {
	Op   string
	Kind Kind
	Err  error
}

func E(op string, kind Kind, err error) error {
	if err == nil {
		return nil
	}
	return &Error{
		Op:   op,
		Kind: kind,
		Err:  err,
	}
}

// String returns the string representation of the error kind.
func (k Kind) String() string {
	switch k {
	case Unknown:
		return "Unknown"
	case NotFound:
		return "NotFound"
	case Conflict:
		return "Conflict"
	case Invalid:
		return "Invalid"
	case Unauthorized:
		return "Unauthorized"
	case Forbidden:
		return "Forbidden"
	case Unavailable:
		return "Unavailable"
	case Internal:
		return "Internal"
	default:
		return fmt.Sprintf("Kind(%d)", k)
	}
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Op
	}
	if e.Op == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

func KindOf(err error) Kind {
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return Unknown
}

func OpOf(err error) string {
	var e *Error
	if errors.As(err, &e) {
		return e.Op
	}
	return ""
}
