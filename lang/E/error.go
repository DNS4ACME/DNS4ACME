package E //nolint:revive //We want this to be called E

import (
	"errors"
	"github.com/dns4acme/dns4acme/lang"
	"log/slog"
)

type Error interface {
	error

	// Iterator is an iter.Seq-compatible iterator to loop over all cause errors that implement Error.
	Iterator(yield func(Error) bool)

	GetCode() string
	GetMessage() string

	Wrap(err error) Error
	WrapIfError(err error, attrs ...slog.Attr) Error
	Unwrap() error

	WithAttr(attr slog.Attr) Error
	GetAttrs() *lang.LinkedList[slog.Attr]
	GetAttr(key string) *slog.Attr
}

func New(code string, message string) Error {
	return &structuredError{
		code:    code,
		message: message,
	}
}

type structuredError struct {
	code    string
	message string
	cause   error
	attrs   *lang.LinkedList[slog.Attr]
}

func (e *structuredError) WrapIfError(err error, attrs ...slog.Attr) Error {
	if err == nil {
		return nil
	}
	err2 := e.Wrap(err)
	for _, attr := range attrs {
		err2 = err2.WithAttr(attr)
	}
	return err2
}

func (e *structuredError) GetMessage() string {
	return e.message
}

func (e *structuredError) Iterator(yield func(Error) bool) {
	if e == nil {
		return
	}
	if !yield(e) {
		return
	}
	if e.cause == nil {
		return
	}
	var next Error
	if !errors.As(e.cause, &next) {
		return
	}
	next.Iterator(yield)
}

func (e *structuredError) Wrap(err error) Error {
	if e.cause != nil {
		panic("Bug: trying to wrap an error that already has a cause!")
	}
	return &structuredError{
		code:    e.code,
		message: e.message,
		cause:   err,
		attrs:   e.attrs,
	}
}

func (e *structuredError) Unwrap() error {
	return e.cause
}

func (e *structuredError) Error() string {
	if e.cause != nil {
		return e.code + ": " + e.message + " (" + e.cause.Error() + ")"
	}
	return e.code + ": " + e.message
}

func (e *structuredError) String() string {
	if e.cause != nil {
		return e.code + ": " + e.message + " (" + e.cause.Error() + ")"
	}
	return e.code + ": " + e.message
}

func (e *structuredError) GetCode() string {
	return e.code
}

func (e *structuredError) WithAttr(attr slog.Attr) Error {
	return &structuredError{
		code:    e.code,
		message: e.message,
		cause:   e.cause,
		attrs: &lang.LinkedList[slog.Attr]{
			Item: attr,
			Next: e.attrs,
		},
	}
}

func (e *structuredError) GetAttrs() *lang.LinkedList[slog.Attr] {
	return e.attrs
}

func (e *structuredError) GetAttrsRecursive() []slog.Attr {
	var attrs []slog.Attr
	for err := range e.Iterator {
		attrs = append(attrs, err.GetAttrs().Slice()...)
	}
	return attrs
}

func (e *structuredError) GetAttr(key string) *slog.Attr {
	for attr := range e.attrs.Iterator {
		if attr.Key == key {
			return &attr
		}
	}
	return nil
}

func (e *structuredError) GetAttrRecursive(key string) *slog.Attr {
	for err := range e.Iterator {
		if attr := err.GetAttr(key); attr != nil {
			return attr
		}
	}
	return nil
}
