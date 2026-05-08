package auth

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hovanhoa/llmgateway/pkg/core/encoding"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"

	"github.com/hovanhoa/llmgateway/pkg/driver"
)

const (
	// Expiry is the amount of time a token is valid for
	Expiry = 7 * 24 * time.Hour
)

// Dependencies of the authentication service.
type Dependencies struct {
	// DB is a key-value database that stores the tokens.
	DB driver.KVStore

	// JWTPrivateKey is the private key used to sign JWT tokens.
	JWTPrivateKey *ecdsa.PrivateKey

	// JWTPublicKey is the public key used to verify JWT tokens.
	JWTPublicKey *ecdsa.PublicKey
}

// TokenService implements the authentication service.
type TokenService[Identity ~string] struct {
	deps Dependencies
}

// NewTokenService creates a new authentication service with the specified
// dependencies.
func NewTokenService[Identity ~string](deps Dependencies) *TokenService[Identity] {
	return &TokenService[Identity]{deps}
}

// Token is a string representation of a JWT token.
type TokenString string

type JWT struct {
	ID        string
	Token     TokenString
	ExpiresAt time.Time
}

// GenerateJWT creates a new JWT for the user.
func (s *TokenService[Identity]) GenerateJWT(
	ctx context.Context,
	userID string,
	userEmail string,
	userFullName string,
	agencyID string,
	agencyName string,
	subjectType Identity,
) (*JWT, error) {
	identifier := encoding.NewRandomIdentifier("jwt")
	issuedAt := time.Now()

	token := jwt.NewWithClaims(jwt.SigningMethodES384, &Claims[Identity]{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        identifier,
			ExpiresAt: jwt.NewNumericDate(issuedAt.Add(Expiry)),
			IssuedAt:  jwt.NewNumericDate(issuedAt),
			NotBefore: jwt.NewNumericDate(issuedAt),
			Subject:   userID,
		},
		SubjectType:      subjectType,
		SubjectEmail:     userEmail,
		SubjectFullName:  userFullName,
		OrganizationID:   agencyID,
		OrganizationName: agencyName,
	})

	tokenString, err := token.SignedString(s.deps.JWTPrivateKey)
	if err != nil {
		return nil, errors.Wrap(err, "jwt.SignedString")
	}

	err = s.deps.DB.Set(ctx, s.tokenKey(identifier), tokenString, Expiry)
	if err != nil {
		return nil, errors.Wrap(err, "DB.Set(%q, %q)", s.tokenKey(identifier), tokenString)
	}

	return &JWT{
		ID:        identifier,
		Token:     TokenString(tokenString),
		ExpiresAt: issuedAt.Add(Expiry),
	}, nil
}

func (s *TokenService[Identity]) VerifyJWT(ctx context.Context, tokenString TokenString) (*Claims[Identity], error) {
	// Create a JWT parser with validation options.
	parser := jwt.NewParser(
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithValidMethods([]string{jwt.SigningMethodES384.Alg()}),
	)

	// Parse and validate the token.
	token, err := parser.ParseWithClaims(string(tokenString), &Claims[Identity]{}, func(token *jwt.Token) (interface{}, error) {
		return s.deps.JWTPublicKey, nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "jwt.ParseWithClaims")
	}
	if token == nil || !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Extract the claims.
	claims, ok := token.Claims.(*Claims[Identity])
	if !ok || claims == nil || claims.Subject == "" {
		return nil, errors.New("invalid claims")
	}

	// Check that the token has not been revoked
	found, _, err := s.deps.DB.Get(ctx, s.tokenKey(claims.ID))
	if err != nil {
		return nil, errors.Wrap(err, "DB.Get(%q)", s.tokenKey(claims.ID))
	}
	if !found {
		return nil, errors.New("token revoked")
	}

	return claims, nil
}

// RevokeJWT removes the token stored for the given identifier.
func (s *TokenService[Identity]) RevokeJWT(ctx context.Context, identifier string) error {
	return errors.Wrap(s.deps.DB.Del(ctx, s.tokenKey(identifier)), "DB.Del(%q)", s.tokenKey(identifier))
}

func (s *TokenService[Identity]) tokenKey(identifier string) string {
	return fmt.Sprintf("/token/%s", identifier)
}
