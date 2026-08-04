package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func digest(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

func testAuthenticator(t *testing.T) *Authenticator {
	t.Helper()
	authenticator, err := New(Configuration{
		TrustedProxies: []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		HumanCanWrite:  true,
		CredentialStore: Credentials{Version: 1, Tokens: []Token{{
			Name: "release-agent", SHA256: digest("correct-horse-battery-staple"),
			Repositories: []string{"api"}, Permissions: []Permission{Write},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return authenticator
}

func TestBearerCredentialsAreHashedScopedAndAttributed(t *testing.T) {
	request := httptest.NewRequest("GET", "https://koment.example/r/api/", nil)
	request.RemoteAddr = "198.51.100.10:1234"
	request.Header.Set("Authorization", "Bearer correct-horse-battery-staple")
	principal, err := testAuthenticator(t).Authenticate(request)
	if err != nil {
		t.Fatal(err)
	}
	if !principal.Can("api", Read) || !principal.Can("api", Write) || principal.Can("web", Read) {
		t.Fatalf("scope = repositories %#v permissions %#v", principal.Repositories, principal.Permissions)
	}
	if author := principal.Author(); author.Name != "release-agent" || author.Verified != "bearer-sha256" {
		t.Fatalf("author = %#v", author)
	}
}

func TestInvalidBearerCredentialIsRejected(t *testing.T) {
	request := httptest.NewRequest("GET", "https://koment.example/", nil)
	request.RemoteAddr = "198.51.100.10:1234"
	request.Header.Set("Authorization", "Bearer wrong")
	if _, err := testAuthenticator(t).Authenticate(request); err == nil {
		t.Fatal("invalid credential was accepted")
	}
}

func TestForwardedIdentityIsAcceptedOnlyFromATrustedProxy(t *testing.T) {
	for _, fixture := range []struct {
		remote string
		valid  bool
	}{{"10.1.2.3:443", true}, {"198.51.100.10:443", false}} {
		request := httptest.NewRequest("GET", "https://koment.example/", nil)
		request.RemoteAddr = fixture.remote
		request.Header.Set("X-Forwarded-User", "Jan Puc")
		principal, err := testAuthenticator(t).Authenticate(request)
		if fixture.valid && (err != nil || principal.Name != "Jan Puc" || !principal.Can("anything", Write)) {
			t.Fatalf("trusted proxy: principal = %#v, err = %v", principal, err)
		}
		if !fixture.valid && err == nil {
			t.Fatal("untrusted forwarded identity was accepted")
		}
	}
}

func TestLoopbackBypassMustBeExplicitlyEnabled(t *testing.T) {
	request := httptest.NewRequest("GET", "http://localhost/", nil)
	request.RemoteAddr = "127.0.0.1:1234"
	if _, err := testAuthenticator(t).Authenticate(request); err == nil {
		t.Fatal("loopback bypass was enabled implicitly")
	}
	authenticator, err := New(Configuration{AllowLoopback: true})
	if err != nil {
		t.Fatal(err)
	}
	principal, err := authenticator.Authenticate(request)
	if err != nil || !principal.Can("api", Write) {
		t.Fatalf("explicit loopback: principal = %#v, err = %v", principal, err)
	}
}

func TestCredentialValidationFailsClosed(t *testing.T) {
	cases := []Credentials{
		{Version: 2, Tokens: []Token{{Name: "agent", SHA256: digest("x"), Repositories: []string{"api"}, Permissions: []Permission{Read}}}},
		{Version: 1, Tokens: []Token{{Name: "agent", SHA256: "not-a-hash", Repositories: []string{"api"}, Permissions: []Permission{Read}}}},
		{Version: 1, Tokens: []Token{{Name: "agent", SHA256: digest("x"), Permissions: []Permission{Read}}}},
		{Version: 1, Tokens: []Token{{Name: "agent", SHA256: digest("x"), Repositories: []string{"api"}, Permissions: []Permission{"admin"}}}},
	}
	for _, credentials := range cases {
		if _, err := New(Configuration{CredentialStore: credentials}); err == nil {
			t.Errorf("accepted invalid credentials: %#v", credentials)
		}
	}
}
