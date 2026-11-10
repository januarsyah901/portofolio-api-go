package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// API token format and limits, identical to src/lib/apiToken.ts.
const (
	TokenPrefix     = "pf_"
	MaxActiveTokens = 10
	ScopeRead       = "read"
	ScopeWrite      = "write"
)

// HashAPIToken returns the hex SHA-256 of a plaintext token. Only this hash is stored.
func HashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GeneratedToken is a fresh API token. Token is shown to the user once; store Hash.
type GeneratedToken struct {
	Token  string
	Hash   string
	Prefix string
}

// GenerateAPIToken creates "pf_" + 32 random bytes in base64url (no padding).
func GenerateAPIToken() (GeneratedToken, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return GeneratedToken{}, err
	}
	token := TokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return GeneratedToken{Token: token, Hash: HashAPIToken(token), Prefix: token[:8]}, nil
}

// IsAPIToken reports whether a credential is an API token rather than a JWT.
func IsAPIToken(credential string) bool {
	return strings.HasPrefix(credential, TokenPrefix)
}

// ExtractBearer returns the credential from "Bearer <credential>".
// The scheme is case-insensitive; anything else (extra parts, other schemes) yields "".
func ExtractBearer(header string) string {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

// NormalizeTokenScope validates a scope. A missing scope (present=false) defaults to "read".
func NormalizeTokenScope(scope string, present bool) (string, bool) {
	if !present {
		return ScopeRead, true
	}
	if scope == ScopeRead || scope == ScopeWrite {
		return scope, true
	}
	return "", false
}

// CanWrite reports whether a token scope may modify data.
func CanWrite(scope string) bool { return scope == ScopeWrite }

// NormalizeTokenName trims the name and accepts 1-60 characters.
func NormalizeTokenName(name string) (string, bool) {
	trimmed := strings.TrimSpace(name)
	n := len([]rune(trimmed))
	return trimmed, n >= 1 && n <= 60
}
