package retries

import (
	"math"
	"math/rand"
	"time"
)

// Backoff describes the parameters of an exponential backoff.
type Backoff struct {
	// MaxRetries is the maximum number of times the backoff should
	// be called. It should be enforced by the caller by checking Attempt().
	MaxRetries int

	// Interval is the constant of the exponential backoff, which provides
	// a way to describe how long to wait in each attempt.
	Interval time.Duration
	// MaxInterval provides an upper bound on the backoff time, to prevent
	// tasks from sleeping for too long.
	MaxInterval time.Duration
	// MinInterval provides a lower bound on the backoff time, ensuring that
	// at least this much time is waited before the next attempt.
	MinInterval time.Duration
	// MaxJitter specifies the maximum amount of jitter to add (or subtract,
	// uniformly), to prevent thundering herds on retries.
	MaxJitter time.Duration

	// SleepFunc is the function to use to sleep between retries.
	SleepFunc func(time.Duration) error

	n int
}

// NewBackoff creates a new default Backoff object with the time.Sleep
// SleepFunc.
func NewBackoff() *Backoff {
	return &Backoff{
		SleepFunc: func(t time.Duration) error {
			time.Sleep(t)
			return nil
		},
	}
}

// Max returns a Backoff object configured with the specified number of retries.
func Max(n int) *Backoff {
	b := NewBackoff().WithMaxRetries(n)
	return &b
}

// Return a backoff with a default configuration of 10 retries with 5s interval
func DefaultBackoff() *Backoff {
	backoff := NewBackoff().
		WithInterval(5 * time.Second).
		WithMaxInterval(1 * time.Hour).
		WithMinInterval(10 * time.Second).
		WithMaxRetries(10)

	return &backoff
}

// WithMaxRetries returns a new Backoff with the specified MaxRetries setting.
func (b Backoff) WithMaxRetries(maxRetries int) Backoff {
	b.MaxRetries = maxRetries
	return b
}

// WithInterval returns a new Backoff with the specified Interval setting.
func (b Backoff) WithInterval(interval time.Duration) Backoff {
	b.Interval = interval
	return b
}

// WithMaxInterval returns a new Backoff with the specified MaxInterval setting.
func (b Backoff) WithMaxInterval(maxInterval time.Duration) Backoff {
	b.MaxInterval = maxInterval
	return b
}

// WithMinInterval returns a new Backoff with the specified MinInterval setting.
func (b Backoff) WithMinInterval(minInterval time.Duration) Backoff {
	b.MinInterval = minInterval
	return b
}

// WithMaxJitter returns a new Backoff with the specified MaxJitter setting.
func (b Backoff) WithMaxJitter(maxJitter time.Duration) Backoff {
	b.MaxJitter = maxJitter
	return b
}

// WithSleepFunc returns a new Backoff with the specified SleepFunc setting.
func (b Backoff) WithSleepFunc(sleepFunc func(time.Duration) error) Backoff {
	b.SleepFunc = sleepFunc
	return b
}

// Next returns whether or not there is another attempt in this backoff,
// and increments the attempt number.
func (b *Backoff) Next() bool {
	defer func() { b.n++ }()
	return b.n <= b.MaxRetries
}

// FinalAttempt returns true if there is no next attempt.
func (b Backoff) FinalAttempt() bool {
	return b.n > b.MaxRetries
}

// Attempt returns the number of times the backoff has been called. This is
// automatically incremented when using `Sleep()`, but must be manually
// incremented using `Increment()` otherwise.
func (b Backoff) Attempt() int {
	return b.n
}

// Reset the number of attempts back to zero.
func (b *Backoff) Reset() {
	b.n = 0
}

// SleepDuration returns the amount of time the caller should sleep for the
// specified attempt number.
//
// The formula is ((Interval * 2^Attempt) + rand(-MaxJitter, MaxJitter)), bounded
// by MinInterval and MaxInterval.
func (b *Backoff) SleepDuration(attempt int) time.Duration {
	if b.Interval == 0 {
		b.Interval = 100 * time.Millisecond
	}
	if b.MaxInterval == 0 {
		b.MaxInterval = math.MaxInt64
	}

	// get the base duration
	duration := float64(b.Interval) * math.Pow(2, float64(attempt))
	// add jitter if any specified
	if b.MaxJitter > 0 {
		duration = duration + float64(rand.Int63n(int64(b.MaxJitter)*2)-int64(b.MaxJitter))
	}
	// bound duration between min and max interval
	duration = math.Max(float64(b.MinInterval), math.Min(duration, float64(b.MaxInterval)))

	return time.Duration(duration)
}

// Sleep computes the sleep duration for the current attempt number and
// sleeps that duration.
func (b *Backoff) Sleep() error {
	return b.SleepFunc(b.SleepDuration(b.Attempt()))
}

// Increment manually increments the attempt number. This only needs to
// be called if the user is not using `Next()`, which automatically increments
// the attempt number.
func (b *Backoff) Increment() {
	b.n++
}
