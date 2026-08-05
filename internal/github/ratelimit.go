package github

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// RateLimit is what GitHub said about the budget a response spent from. Every
// response carries it, including the successful ones, so a caller can see a
// limit approaching instead of only discovering it at zero.
type RateLimit struct {
	Limit     int
	Remaining int
	Used      int
	Resource  string

	// Reset is when the primary budget refills.
	Reset time.Time

	// RetryAfter is what a secondary limit asks for instead. GitHub sends it
	// on abuse detection, where Reset is absent and the only honest answer to
	// "when may I retry" is the number of seconds it just gave us.
	RetryAfter time.Duration
}

func rateLimitFrom(header http.Header) (RateLimit, bool) {
	limit := RateLimit{
		Limit:     headerInt(header, "X-RateLimit-Limit"),
		Remaining: headerInt(header, "X-RateLimit-Remaining"),
		Used:      headerInt(header, "X-RateLimit-Used"),
		Resource:  header.Get("X-RateLimit-Resource"),
	}
	if seconds := headerInt(header, "X-RateLimit-Reset"); seconds > 0 {
		limit.Reset = time.Unix(int64(seconds), 0).UTC()
	}
	if seconds := headerInt(header, "Retry-After"); seconds > 0 {
		limit.RetryAfter = time.Duration(seconds) * time.Second
	}
	return limit, limit.Limit > 0 || !limit.Reset.IsZero() || limit.RetryAfter > 0
}

func headerInt(header http.Header, name string) int {
	value, err := strconv.Atoi(header.Get(name))
	if err != nil {
		return 0
	}
	return value
}

// Exhausted reports whether the budget is spent. A secondary limit reports no
// remaining count at all, so Retry-After alone is enough to mean "stop".
func (r RateLimit) Exhausted() bool {
	return (r.Limit > 0 && r.Remaining <= 0) || r.RetryAfter > 0
}

// RecoversAt is the earliest a caller should try again. It never returns a
// time in the past for an exhausted budget, because a caller that treats
// "already reset" as "go ahead" is the caller that spends the next window in
// one burst.
func (r RateLimit) RecoversAt(now time.Time) time.Time {
	if r.RetryAfter > 0 {
		return now.Add(r.RetryAfter)
	}
	if r.Reset.After(now) {
		return r.Reset
	}
	return now
}

func (r RateLimit) String() string {
	if r.Limit == 0 && r.RetryAfter == 0 {
		return ""
	}
	described := fmt.Sprintf("%d of %d remaining", r.Remaining, r.Limit)
	if r.Resource != "" {
		described += " for " + r.Resource
	}
	if r.RetryAfter > 0 {
		return described + fmt.Sprintf("; retry after %s", r.RetryAfter)
	}
	if !r.Reset.IsZero() {
		return described + fmt.Sprintf("; resets at %s", r.Reset.Format(time.RFC3339))
	}
	return described
}

// RetryAfter reports when a request refused for rate limiting may be repeated.
// A caller on a timer needs this to stop asking: retrying a spent budget on
// the usual interval is what turns one exhausted window into several.
func RetryAfter(err error) (time.Time, bool) {
	var responseError *apiError
	if !errors.As(err, &responseError) || !responseError.limit.Exhausted() {
		return time.Time{}, false
	}
	return responseError.limit.RecoversAt(time.Now().UTC()), true
}
