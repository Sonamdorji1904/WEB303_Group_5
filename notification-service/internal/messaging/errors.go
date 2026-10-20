package messaging

import "errors"

// ErrPermanent marks a failure that retrying cannot fix (malformed JSON,
// unknown event type, missing required fields). Such messages go straight to
// the dead-letter queue instead of being retried.
var ErrPermanent = errors.New("permanent processing error")

// Permanent wraps err so errors.Is(err, ErrPermanent) is true.
func Permanent(err error) error {
	return &permanentError{err}
}

type permanentError struct{ err error }

func (p *permanentError) Error() string { return p.err.Error() }
func (p *permanentError) Unwrap() error { return p.err }
func (p *permanentError) Is(target error) bool {
	return target == ErrPermanent
}
