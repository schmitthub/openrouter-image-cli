package cmdutil

import "fmt"

// FlagErrorf returns a new FlagError that wraps an error produced by
// fmt.Errorf(format, args...).
func FlagErrorf(format string, args ...interface{}) error {
	return FlagErrorWrap(fmt.Errorf(format, args...))
}

// FlagErrorWrap returns a new FlagError that wraps the specified error.
func FlagErrorWrap(err error) error { return &FlagError{err} }

// A *FlagError indicates an error processing command-line flags or other arguments.
// Such errors cause the application to display the usage message.
type FlagError struct {
	// Note: not struct{error}: only *FlagError should satisfy error.
	err error
}

func (fe *FlagError) Error() string {
	return fe.err.Error()
}

func (fe *FlagError) Unwrap() error {
	return fe.err
}
