package session

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStoreReadsAndRefreshesSessions(t *testing.T) {
	sessions := NewStore()
	first := Token{Subject: "first", Expiry: time.Unix(1, 0)}
	refreshed := Token{Subject: "refreshed", Expiry: time.Unix(2, 0)}
	sessions.Put("known", first)
	sessions.Put("known", refreshed)

	got, err := sessions.Get("known")
	if err != nil {
		t.Fatal(err)
	}
	if got != refreshed || sessions.Len() != 1 {
		t.Fatalf("refresh should replace one session, got %+v across %d entries", got, sessions.Len())
	}
	if _, err := sessions.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestStoreSupportsConcurrentReadersAndWriters(t *testing.T) {
	sessions := NewStore()
	var writers sync.WaitGroup
	for index := range 32 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			id := fmt.Sprintf("session-%d", index)
			sessions.Put(id, Token{Subject: id})
			if _, err := sessions.Get(id); err != nil {
				t.Errorf("reading %s: %v", id, err)
			}
		}()
	}
	writers.Wait()
	if sessions.Len() != 32 {
		t.Fatalf("want 32 sessions, got %d", sessions.Len())
	}
}
