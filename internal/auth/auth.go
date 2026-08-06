package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	"github.com/koment-dev/koment/internal/store"
)

type Permission string

const (
	Read  Permission = "read"
	Write Permission = "write"
)

type Principal struct {
	Name         string
	Email        string
	Account      string
	Kind         store.AuthorKind
	Verification string
	Repositories map[string]bool
	Permissions  map[Permission]bool
}

func (p Principal) Can(repository string, permission Permission) bool {
	return p.Permissions[permission] && (p.Repositories == nil || p.Repositories[repository])
}

func (p Principal) Author() store.Author {
	source := store.FromSession
	if p.Kind == store.AuthorAgent && p.Verification == "bearer-sha256" {
		source = store.FromScopedAgent
	}
	if p.Kind == store.AuthorHuman && p.Verification == "trusted-proxy" {
		source = store.FromOIDCProxy
	}
	return store.Author{
		Name: p.Name, Email: p.Email, Account: p.Account, Kind: p.Kind,
		Source: source, Verified: p.Verification,
	}
}

type Token struct {
	Name         string       `yaml:"name"`
	SHA256       string       `yaml:"sha256"`
	Repositories []string     `yaml:"repositories"`
	Permissions  []Permission `yaml:"permissions"`
}

type Credentials struct {
	Version int     `yaml:"version"`
	Tokens  []Token `yaml:"tokens"`
}

type Configuration struct {
	AllowLoopback      bool
	TrustedProxies     []netip.Prefix
	HumanNameHeader    string
	HumanEmailHeader   string
	HumanAccountHeader string
	HumanRepositories  map[string]bool
	HumanCanWrite      bool
	CredentialStore    Credentials
}

type Authenticator struct {
	configuration Configuration
	tokens        []verifiedToken
}

type verifiedToken struct {
	digest    [sha256.Size]byte
	principal Principal
}

func LoadCredentials(path string) (Credentials, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Credentials{}, fmt.Errorf("reading credentials %s: %w", path, err)
	}
	var credentials Credentials
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&credentials); err != nil {
		return Credentials{}, fmt.Errorf("parsing credentials %s: %w", path, err)
	}
	return credentials, nil
}

func New(configuration Configuration) (*Authenticator, error) {
	if configuration.HumanNameHeader == "" {
		configuration.HumanNameHeader = "X-Forwarded-User"
	}
	if configuration.HumanEmailHeader == "" {
		configuration.HumanEmailHeader = "X-Forwarded-Email"
	}
	if configuration.HumanAccountHeader == "" {
		configuration.HumanAccountHeader = "X-Forwarded-Preferred-Username"
	}
	if len(configuration.CredentialStore.Tokens) > 0 && configuration.CredentialStore.Version != 1 {
		return nil, fmt.Errorf("credential version %d is not supported", configuration.CredentialStore.Version)
	}
	authenticator := &Authenticator{configuration: configuration}
	seenNames := map[string]bool{}
	for _, token := range configuration.CredentialStore.Tokens {
		if strings.TrimSpace(token.Name) == "" {
			return nil, errors.New("credential token has no name")
		}
		if seenNames[token.Name] {
			return nil, fmt.Errorf("duplicate credential name %s", token.Name)
		}
		seenNames[token.Name] = true
		decoded, err := hex.DecodeString(token.SHA256)
		if err != nil || len(decoded) != sha256.Size {
			return nil, fmt.Errorf("credential %s sha256 must be 64 hexadecimal characters", token.Name)
		}
		principal := Principal{
			Name: token.Name, Account: token.Name, Kind: store.AuthorAgent, Verification: "bearer-sha256",
			Repositories: make(map[string]bool, len(token.Repositories)), Permissions: make(map[Permission]bool),
		}
		for _, repository := range token.Repositories {
			if strings.TrimSpace(repository) == "" {
				return nil, fmt.Errorf("credential %s has an empty repository scope", token.Name)
			}
			principal.Repositories[repository] = true
		}
		if len(principal.Repositories) == 0 {
			return nil, fmt.Errorf("credential %s has no repository scope", token.Name)
		}
		for _, permission := range token.Permissions {
			if permission != Read && permission != Write {
				return nil, fmt.Errorf("credential %s has unknown permission %q", token.Name, permission)
			}
			principal.Permissions[permission] = true
		}
		if principal.Permissions[Write] {
			principal.Permissions[Read] = true
		}
		if !principal.Permissions[Read] {
			return nil, fmt.Errorf("credential %s has no permission", token.Name)
		}
		var digest [sha256.Size]byte
		copy(digest[:], decoded)
		authenticator.tokens = append(authenticator.tokens, verifiedToken{digest: digest, principal: principal})
	}
	return authenticator, nil
}

func (a *Authenticator) Authenticate(request *http.Request) (Principal, error) {
	address, err := remoteAddress(request.RemoteAddr)
	if err != nil {
		return Principal{}, err
	}
	forwarded := request.Header.Get(a.configuration.HumanNameHeader)
	if forwarded != "" {
		if !contains(a.configuration.TrustedProxies, address) {
			return Principal{}, errors.New("forwarded identity came from an untrusted proxy")
		}
		permissions := map[Permission]bool{Read: true}
		if a.configuration.HumanCanWrite {
			permissions[Write] = true
		}
		return Principal{
			Name: forwarded, Email: request.Header.Get(a.configuration.HumanEmailHeader),
			Account: request.Header.Get(a.configuration.HumanAccountHeader), Kind: store.AuthorHuman,
			Verification: "trusted-proxy", Repositories: cloneScope(a.configuration.HumanRepositories), Permissions: permissions,
		}, nil
	}
	if authorization := request.Header.Get("Authorization"); authorization != "" {
		return a.authenticateBearer(authorization)
	}
	if a.configuration.AllowLoopback && address.IsLoopback() {
		return Principal{
			Name: "Local user", Kind: store.AuthorHuman, Verification: "loopback",
			Permissions: map[Permission]bool{Read: true, Write: true},
		}, nil
	}
	return Principal{}, errors.New("authentication required")
}

func (a *Authenticator) authenticateBearer(authorization string) (Principal, error) {
	scheme, secret, found := strings.Cut(authorization, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || secret == "" || strings.Contains(secret, " ") {
		return Principal{}, errors.New("authorization must be one Bearer credential")
	}
	digest := sha256.Sum256([]byte(secret))
	matched := -1
	for index, token := range a.tokens {
		if subtle.ConstantTimeCompare(digest[:], token.digest[:]) == 1 {
			matched = index
		}
	}
	if matched < 0 {
		return Principal{}, errors.New("bearer credential is invalid")
	}
	return clonePrincipal(a.tokens[matched].principal), nil
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		principal, err := a.Authenticate(request)
		if err != nil {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="koment"`)
			http.Error(writer, err.Error(), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request.WithContext(WithPrincipal(request.Context(), principal)))
	})
}

type principalKey struct{}

func WithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, principal)
}

func FromContext(ctx context.Context) (Principal, bool) {
	principal, found := ctx.Value(principalKey{}).(Principal)
	return principal, found
}

func remoteAddress(remote string) (netip.Addr, error) {
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		host = remote
	}
	address, parseErr := netip.ParseAddr(strings.Trim(host, "[]"))
	if parseErr != nil {
		return netip.Addr{}, fmt.Errorf("request has invalid remote address %q", remote)
	}
	return address.Unmap(), nil
}

func contains(prefixes []netip.Prefix, address netip.Addr) bool {
	for _, prefix := range prefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func cloneScope(scope map[string]bool) map[string]bool {
	if scope == nil {
		return nil
	}
	cloned := make(map[string]bool, len(scope))
	for repository, allowed := range scope {
		cloned[repository] = allowed
	}
	return cloned
}

func clonePrincipal(principal Principal) Principal {
	principal.Repositories = cloneScope(principal.Repositories)
	permissions := make(map[Permission]bool, len(principal.Permissions))
	for permission, allowed := range principal.Permissions {
		permissions[permission] = allowed
	}
	principal.Permissions = permissions
	return principal
}
