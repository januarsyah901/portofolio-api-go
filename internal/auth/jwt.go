// Package auth holds credential primitives (JWT, bcrypt, API tokens) and the three
// route guards used by every module: JWTOnly, AdminOrRead and AdminOrWrite.
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// TokenTTL matches the Express version's expiresIn: '1d'.
const TokenTTL = 24 * time.Hour

// Claims mirrors the payload signed by jsonwebtoken in the Express API: { userId }.
type Claims struct {
	UserID int `json:"userId"`
	jwt.RegisteredClaims
}

// JWT signs and verifies admin session tokens with HS256.
type JWT struct {
	secret []byte
	now    func() time.Time
}

// NewJWT returns a signer/verifier for the given secret.
func NewJWT(secret string) *JWT {
	return &JWT{secret: []byte(secret), now: time.Now}
}

// Sign issues a token for the user, valid for TokenTTL.
func (j *JWT) Sign(userID int) (string, error) {
	now := j.now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(TokenTTL)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(j.secret)
}

// ErrInvalidToken is returned for any malformed, expired or wrongly signed token.
var ErrInvalidToken = errors.New("invalid token")

// Verify checks the signature and expiry and returns the user id.
func (j *JWT) Verify(token string) (int, error) {
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		return j.secret, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithTimeFunc(j.now),
		jwt.WithExpirationRequired(),
	)
	if err != nil || !parsed.Valid || claims.UserID == 0 {
		return 0, ErrInvalidToken
	}
	return claims.UserID, nil
}

// HashPassword hashes with bcrypt cost 10 (the bcryptjs default used by the Express seed).
func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(b), err
}

// dummyHash is compared against when a user does not exist so login timing does not
// reveal which usernames are valid.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("timing-equaliser"), 10)

// CheckPassword reports whether password matches hash. Pass hash="" for an unknown
// user; the comparison still runs against a dummy hash.
func CheckPassword(hash, password string) bool {
	if hash == "" {
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
