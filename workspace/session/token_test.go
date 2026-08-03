package session

import (
	"testing"
	"time"
)

func TestExpiryAllowsForClockSkew(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	if (Token{Expiry: now.Add(-20 * time.Second)}).Expired(now) {
		t.Fatal("a token inside the clock-skew allowance should remain valid")
	}
	if !(Token{Expiry: now.Add(-31 * time.Second)}).Expired(now) {
		t.Fatal("a token beyond the clock-skew allowance should expire")
	}
}

func TestRenewalStartsOnlyInsideTheWindow(t *testing.T) {
	now := time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)
	if (Token{Expiry: now.Add(2 * time.Hour)}).Renewable(now) {
		t.Fatal("a token outside the renewal window should not renew")
	}
	if !(Token{Expiry: now.Add(30 * time.Minute)}).Renewable(now) {
		t.Fatal("a valid token inside the renewal window should renew")
	}
	if (Token{Expiry: now.Add(-time.Hour)}).Renewable(now) {
		t.Fatal("an expired token should not renew")
	}
}
