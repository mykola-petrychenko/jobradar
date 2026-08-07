package httpclient

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// retryableError marks a failure worth repeating: either a network-level
// failure (cause is set) or an HTTP status the server may recover from.
type retryableError struct {
	status     int
	retryAfter string
	cause      error
}

func (e *retryableError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("network failure: %v", e.cause)
	}
	return fmt.Sprintf("server returned status %d", e.status)
}

func (e *retryableError) Unwrap() error { return e.cause }

const maxRetryWait = 60 * time.Second

func (e *retryableError) wait(attempt int) time.Duration {
	if d, ok := parseRetryAfter(e.retryAfter); ok {
		return min(d, maxRetryWait)
	}
	backoff := time.Duration(1<<attempt) * time.Second
	return min(backoff, maxRetryWait)
}

func parseRetryAfter(v string) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d, true
		}
		return 0, true
	}
	return 0, false
}
