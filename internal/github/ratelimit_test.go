package github

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func header(pairs ...string) http.Header {
	built := http.Header{}
	for index := 0; index+1 < len(pairs); index += 2 {
		built.Set(pairs[index], pairs[index+1])
	}
	return built
}

func TestRateLimitReadsWhatGitHubReports(t *testing.T) {
	reset := time.Date(2026, 8, 5, 22, 26, 31, 0, time.UTC)
	limit, found := rateLimitFrom(header(
		"X-RateLimit-Limit", "5000",
		"X-RateLimit-Remaining", "0",
		"X-RateLimit-Used", "5000",
		"X-RateLimit-Resource", "core",
		"X-RateLimit-Reset", fmt.Sprint(reset.Unix()),
	))
	if !found {
		t.Fatal("headers carrying a budget were not recognised")
	}
	if limit.Limit != 5000 || limit.Remaining != 0 || limit.Used != 5000 || limit.Resource != "core" {
		t.Errorf("limit = %+v", limit)
	}
	if !limit.Reset.Equal(reset) {
		t.Errorf("reset = %s, want %s", limit.Reset, reset)
	}
	if !limit.Exhausted() {
		t.Error("a budget with nothing remaining is not exhausted")
	}
	if got := limit.RecoversAt(reset.Add(-time.Hour)); !got.Equal(reset) {
		t.Errorf("recovers at %s, want the reset %s", got, reset)
	}
}

func TestARemainingBudgetIsNotExhausted(t *testing.T) {
	limit, found := rateLimitFrom(header(
		"X-RateLimit-Limit", "5000", "X-RateLimit-Remaining", "4931",
	))
	if !found || limit.Exhausted() {
		t.Fatalf("limit = %+v, found = %v", limit, found)
	}
}

// A secondary limit reports no budget at all - only how long to wait - so
// Retry-After has to be enough on its own to mean stop.
func TestASecondaryLimitStopsOnRetryAfterAlone(t *testing.T) {
	limit, found := rateLimitFrom(header("Retry-After", "60"))
	if !found {
		t.Fatal("a Retry-After header was not recognised")
	}
	if !limit.Exhausted() {
		t.Error("Retry-After did not count as exhausted")
	}
	now := time.Date(2026, 8, 5, 22, 0, 0, 0, time.UTC)
	if got, want := limit.RecoversAt(now), now.Add(time.Minute); !got.Equal(want) {
		t.Errorf("recovers at %s, want %s", got, want)
	}
}

func TestAResetAlreadyPastDoesNotRecoverIntoThePast(t *testing.T) {
	now := time.Date(2026, 8, 5, 22, 0, 0, 0, time.UTC)
	limit := RateLimit{Limit: 5000, Reset: now.Add(-time.Hour)}
	if got := limit.RecoversAt(now); got.Before(now) {
		t.Errorf("recovers at %s, which is before now %s", got, now)
	}
}

func TestHeadersWithoutABudgetAreNotMistakenForOne(t *testing.T) {
	if _, found := rateLimitFrom(header("Content-Type", "application/json")); found {
		t.Error("an ordinary response was read as carrying a rate limit")
	}
}

func TestTheErrorSaysWhatIsLeftAndWhenItReturns(t *testing.T) {
	reset := time.Date(2026, 8, 5, 22, 26, 31, 0, time.UTC)
	err := &apiError{
		status:  http.StatusForbidden,
		message: "API rate limit exceeded for installation.",
		limit:   RateLimit{Limit: 5000, Remaining: 0, Resource: "core", Reset: reset},
	}
	for _, want := range []string{"Forbidden", "0 of 5000 remaining", "for core", reset.Format(time.RFC3339)} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err.Error(), want)
		}
	}
}

// RefreshAll wraps each repository failure and joins them, so the answer has
// to survive both without the caller knowing that is what happened.
func TestRetryAfterSurvivesWrappingAndJoining(t *testing.T) {
	reset := time.Now().UTC().Add(30 * time.Minute)
	limited := &apiError{
		status:  http.StatusForbidden,
		message: "API rate limit exceeded for installation.",
		limit:   RateLimit{Limit: 5000, Remaining: 0, Reset: reset},
	}
	joined := errors.Join(
		errors.New("refreshing other: something unrelated"),
		fmt.Errorf("refreshing koment: %w", limited),
	)
	until, found := RetryAfter(joined)
	if !found {
		t.Fatal("a rate-limited failure was lost inside the joined error")
	}
	if !until.Equal(reset) {
		t.Errorf("retry at %s, want %s", until, reset)
	}
}

func TestRetryAfterIgnoresFailuresThatAreNotRateLimits(t *testing.T) {
	notFound := &apiError{status: http.StatusNotFound, message: "Not Found"}
	for name, err := range map[string]error{
		"an ordinary error": errors.New("connection refused"),
		"another status":    notFound,
		"a budget with room": &apiError{
			status: http.StatusForbidden, message: "Forbidden",
			limit: RateLimit{Limit: 5000, Remaining: 4000},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if _, found := RetryAfter(err); found {
				t.Error("treated as a rate limit")
			}
		})
	}
}
