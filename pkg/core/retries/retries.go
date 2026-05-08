package retries

import "github.com/hovanhoa/llmgateway/pkg/core/errors"

// RetryControl provides controls to the function being retried that allow it
// to hook into the retry lifecycle.
type RetryControl struct {
	b *Backoff
}

func NewRetryControl(b *Backoff) *RetryControl {
	return &RetryControl{b}
}

// StopRetries prevents the function from being retried any more times. This is
// useful in case a particular type of error does not make sense to retry.
func (r *RetryControl) StopRetries() {
	r.b.n = r.b.MaxRetries + 1
}

// HasMoreRetries returns true if the backoff will continue to retry.
func (r *RetryControl) HasMoreRetries() bool {
	return !r.b.FinalAttempt()
}

// WithBackoff retries the given method using the given backoff specification until
// either the function does not return an error or the maximum number of attempts is
// reached, whichever comes first.
func WithBackoff(b *Backoff, fn func(r *RetryControl) error) (err error) {
	r := &RetryControl{b}
	for b.Next() {
		tryErr := fn(r)
		if tryErr == nil {
			return nil
		}
		if err != nil {
			err = errors.Wrap(err, "%s", tryErr.Error())
		} else {
			err = tryErr
		}
		if !b.FinalAttempt() {
			err = b.Sleep()
			if err != nil {
				// If sleep function returns an error, we should stop retrying immediately
				return
			}
		}
	}
	return
}
