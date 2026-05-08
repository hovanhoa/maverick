package errors

var (
	ErrQuitPolling     = New("quit polling")
	ErrContinuePolling = New("continue polling")
)
