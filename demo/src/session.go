package session

import "time"

// clockSkew absorbs the difference between our clock and the client's.
const clockSkew = 30 * time.Second

// renewalWindow replaced an inline time.Hour.
const renewalWindow = time.Hour

type Token struct {
	Subject string
	Expiry  time.Time
}

func (t Token) Expired(now time.Time) bool {
	return t.Expiry.Before(now.Add(-clockSkew))
}

func (t Token) remaining(now time.Time) time.Duration {
	return t.Expiry.Sub(now)
}

func (t Token) Renewable(now time.Time) bool {
	return !t.Expired(now) && t.remaining(now) < renewalWindow
}
