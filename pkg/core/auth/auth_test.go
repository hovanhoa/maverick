package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hovanhoa/llmgateway/pkg/driver/memkv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestServiceV2(t *testing.T) *TokenService[testIdentity] {
	// Generate test ECDSA key pair
	privateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	require.NoError(t, err)

	return NewTokenService[testIdentity](Dependencies{
		DB:            memkv.New(),
		JWTPrivateKey: privateKey,
		JWTPublicKey:  &privateKey.PublicKey,
	})
}

func TestGenerateJWT(t *testing.T) {
	t.Parallel()

	svc := newTestServiceV2(t)
	ctx := context.Background()

	t.Run("successful token generation", func(t *testing.T) {
		userID := "test_user_123"
		userEmail := "test@example.com"
		userFullName := "Test User"
		agencyID := "test_agency_123"
		agencyName := "Test Agency"
		subjectType := identityAgencyUser

		jwt, err := svc.GenerateJWT(ctx, userID, userEmail, userFullName, agencyID, agencyName, subjectType)
		require.NoError(t, err)
		require.NotNil(t, jwt)

		// Verify JWT structure
		assert.NotEmpty(t, jwt.ID)
		assert.NotEmpty(t, jwt.Token)
		assert.True(t, jwt.ExpiresAt.After(time.Now()))
		assert.True(t, jwt.ExpiresAt.Before(time.Now().Add(Expiry+time.Minute)))

		// Verify token is stored in DB
		found, tokenString, err := svc.deps.DB.Get(ctx, svc.tokenKey(jwt.ID))
		require.NoError(t, err)
		assert.True(t, found)
		assert.Equal(t, string(jwt.Token), tokenString)
	})

	t.Run("different user roles", func(t *testing.T) {
		testCases := []testIdentity{
			identityAdmin,
			identityAgencyUser,
			identityAgencyClient,
			identityUser,
		}

		for _, role := range testCases {
			t.Run(string(role), func(t *testing.T) {
				jwt, err := svc.GenerateJWT(ctx, "user_"+string(role), "test@example.com", "Test User", "test_agency_123", "Test Agency", role)
				require.NoError(t, err)
				require.NotNil(t, jwt)

				// Verify the token can be parsed and contains correct claims
				claims, err := svc.VerifyJWT(ctx, jwt.Token)
				require.NoError(t, err)
				assert.Equal(t, role, claims.SubjectType)
			})
		}
	})

	t.Run("empty user details", func(t *testing.T) {
		// Test with minimal required fields (userID cannot be empty for verification)
		jwt, err := svc.GenerateJWT(ctx, "anonymous_user", "", "", "", "", identityAnonymous)
		require.NoError(t, err)
		require.NotNil(t, jwt)

		claims, err := svc.VerifyJWT(ctx, jwt.Token)
		require.NoError(t, err)
		assert.Equal(t, "anonymous_user", claims.Subject)
		assert.Equal(t, "", claims.SubjectEmail)
		assert.Equal(t, "", claims.SubjectFullName)
		assert.Equal(t, "", claims.OrganizationID)
		assert.Equal(t, "", claims.OrganizationName)
		assert.Equal(t, identityAnonymous, claims.SubjectType)
	})

}

func TestVerifyJWT(t *testing.T) {
	t.Parallel()

	svc := newTestServiceV2(t)
	ctx := context.Background()

	t.Run("valid token", func(t *testing.T) {
		// Generate a valid token
		jwt, err := svc.GenerateJWT(ctx, "test_user", "test@example.com", "Test User", "test_agency_123", "Test Agency", identityAgencyUser)
		require.NoError(t, err)

		// Verify the token
		claims, err := svc.VerifyJWT(ctx, jwt.Token)
		require.NoError(t, err)
		require.NotNil(t, claims)

		// Verify claims
		assert.Equal(t, "test_user", claims.Subject)
		assert.Equal(t, "test@example.com", claims.SubjectEmail)
		assert.Equal(t, "Test User", claims.SubjectFullName)
		assert.Equal(t, identityAgencyUser, claims.SubjectType)
		assert.NotEmpty(t, claims.ID)
		assert.True(t, claims.ExpiresAt.After(time.Now()))
		assert.Equal(t, "test_agency_123", claims.OrganizationID)
		assert.Equal(t, "Test Agency", claims.OrganizationName)
	})

	t.Run("expired token", func(t *testing.T) {
		// Create a token that's already expired
		identifier := "expired_token"
		issuedAt := time.Now().Add(-2 * Expiry) // 2x expiry time ago
		expiresAt := time.Now().Add(-Expiry)    // expired 1x expiry time ago

		token := jwt.NewWithClaims(jwt.SigningMethodES384, &Claims[testIdentity]{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        identifier,
				ExpiresAt: jwt.NewNumericDate(expiresAt),
				IssuedAt:  jwt.NewNumericDate(issuedAt),
				NotBefore: jwt.NewNumericDate(issuedAt),
				Subject:   "test_user",
			},
			SubjectType:      identityAgencyUser,
			SubjectEmail:     "test@example.com",
			SubjectFullName:  "Test User",
			OrganizationID:   "test_agency_123",
			OrganizationName: "Test Agency",
		})

		tokenString, err := token.SignedString(svc.deps.JWTPrivateKey)
		require.NoError(t, err)

		// Store the token in DB
		err = svc.deps.DB.Set(ctx, svc.tokenKey(identifier), tokenString, Expiry)
		require.NoError(t, err)

		// Verify should fail due to expiration
		_, err = svc.VerifyJWT(ctx, TokenString(tokenString))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token is expired")
	})

	t.Run("revoked token", func(t *testing.T) {
		// Generate a valid token
		jwt, err := svc.GenerateJWT(ctx, "test_user", "test@example.com", "Test User", "test_agency_123", "Test Agency", identityAgencyUser)
		require.NoError(t, err)

		// Revoke the token
		err = svc.RevokeJWT(ctx, jwt.ID)
		require.NoError(t, err)

		// Verify should fail due to revocation
		_, err = svc.VerifyJWT(ctx, jwt.Token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token revoked")
	})

	t.Run("malformed token", func(t *testing.T) {
		// Test with invalid JWT format
		_, err := svc.VerifyJWT(ctx, "not.a.valid.jwt")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "jwt.ParseWithClaims")
	})

	t.Run("token with wrong signing method", func(t *testing.T) {
		// Create a token with wrong signing method
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims[testIdentity]{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        "test_id",
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(Expiry)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				NotBefore: jwt.NewNumericDate(time.Now()),
				Subject:   "test_user",
			},
			SubjectType:      identityAgencyUser,
			SubjectEmail:     "test@example.com",
			SubjectFullName:  "Test User",
			OrganizationID:   "test_agency_123",
			OrganizationName: "Test Agency",
		})

		tokenString, err := token.SignedString([]byte("secret"))
		require.NoError(t, err)

		_, err = svc.VerifyJWT(ctx, TokenString(tokenString))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "jwt.ParseWithClaims")
	})

	t.Run("token with missing required claims", func(t *testing.T) {
		// Create a token without required claims
		token := jwt.NewWithClaims(jwt.SigningMethodES384, &Claims[testIdentity]{
			RegisteredClaims: jwt.RegisteredClaims{
				ID: "test_id",
				// Missing ExpiresAt, IssuedAt, NotBefore, Subject
			},
			SubjectType:     identityAgencyUser,
			SubjectEmail:    "test@example.com",
			SubjectFullName: "Test User",
		})

		tokenString, err := token.SignedString(svc.deps.JWTPrivateKey)
		require.NoError(t, err)

		_, err = svc.VerifyJWT(ctx, TokenString(tokenString))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "jwt.ParseWithClaims")
	})

	t.Run("token with empty subject", func(t *testing.T) {
		// Create a token with empty subject
		identifier := "empty_subject_token"
		token := jwt.NewWithClaims(jwt.SigningMethodES384, &Claims[testIdentity]{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        identifier,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(Expiry)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				NotBefore: jwt.NewNumericDate(time.Now()),
				Subject:   "", // Empty subject
			},
			SubjectType:      identityAgencyUser,
			SubjectEmail:     "test@example.com",
			SubjectFullName:  "Test User",
			OrganizationID:   "test_agency_123",
			OrganizationName: "Test Agency",
		})

		tokenString, err := token.SignedString(svc.deps.JWTPrivateKey)
		require.NoError(t, err)

		// Store the token in DB
		err = svc.deps.DB.Set(ctx, svc.tokenKey(identifier), tokenString, Expiry)
		require.NoError(t, err)

		_, err = svc.VerifyJWT(ctx, TokenString(tokenString))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid claims")
	})

	t.Run("token not in database", func(t *testing.T) {
		// Create a valid token but don't store it in DB
		identifier := "not_in_db_token"
		token := jwt.NewWithClaims(jwt.SigningMethodES384, &Claims[testIdentity]{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        identifier,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(Expiry)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				NotBefore: jwt.NewNumericDate(time.Now()),
				Subject:   "test_user",
			},
			SubjectType:      identityAgencyUser,
			SubjectEmail:     "test@example.com",
			SubjectFullName:  "Test User",
			OrganizationID:   "test_agency_123",
			OrganizationName: "Test Agency",
		})

		tokenString, err := token.SignedString(svc.deps.JWTPrivateKey)
		require.NoError(t, err)

		// Verify should fail because token is not in DB
		_, err = svc.VerifyJWT(ctx, TokenString(tokenString))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token revoked")
	})

	t.Run("future token (not before)", func(t *testing.T) {
		// Create a token that's not valid yet
		identifier := "future_token"
		now := time.Now()
		notBefore := now.Add(1 * time.Hour) // Valid in 1 hour
		expiresAt := now.Add(2 * time.Hour) // Expires in 2 hours

		token := jwt.NewWithClaims(jwt.SigningMethodES384, &Claims[testIdentity]{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        identifier,
				ExpiresAt: jwt.NewNumericDate(expiresAt),
				IssuedAt:  jwt.NewNumericDate(now),
				NotBefore: jwt.NewNumericDate(notBefore),
				Subject:   "test_user",
			},
			SubjectType:      identityAgencyUser,
			SubjectEmail:     "test@example.com",
			SubjectFullName:  "Test User",
			OrganizationID:   "test_agency_123",
			OrganizationName: "Test Agency",
		})

		tokenString, err := token.SignedString(svc.deps.JWTPrivateKey)
		require.NoError(t, err)

		// Store the token in DB
		err = svc.deps.DB.Set(ctx, svc.tokenKey(identifier), tokenString, Expiry)
		require.NoError(t, err)

		// Verify should fail because token is not valid yet
		_, err = svc.VerifyJWT(ctx, TokenString(tokenString))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "jwt.ParseWithClaims")
	})
}

func TestRevokeJWT(t *testing.T) {
	t.Parallel()

	svc := newTestServiceV2(t)
	ctx := context.Background()

	t.Run("revoke existing token", func(t *testing.T) {
		// Generate a token
		jwt, err := svc.GenerateJWT(ctx, "test_user", "test@example.com", "Test User", "test_agency_123", "Test Agency", identityAgencyUser)
		require.NoError(t, err)

		// Verify token exists in DB
		found, _, err := svc.deps.DB.Get(ctx, svc.tokenKey(jwt.ID))
		require.NoError(t, err)
		assert.True(t, found)

		// Revoke the token
		err = svc.RevokeJWT(ctx, jwt.ID)
		require.NoError(t, err)

		// Verify token is removed from DB
		found, _, err = svc.deps.DB.Get(ctx, svc.tokenKey(jwt.ID))
		require.NoError(t, err)
		assert.False(t, found)

		// Verify that the token can no longer be verified
		_, err = svc.VerifyJWT(ctx, jwt.Token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token revoked")
	})

}

func TestJWTIntegration(t *testing.T) {
	t.Parallel()

	svc := newTestServiceV2(t)
	ctx := context.Background()

	t.Run("full JWT lifecycle", func(t *testing.T) {
		// 1. Generate JWT
		jwt, err := svc.GenerateJWT(ctx, "integration_user", "integration@example.com", "Integration User", "test_agency_123", "Test Agency", identityAgencyClient)
		require.NoError(t, err)
		require.NotNil(t, jwt)

		// 2. Verify JWT is valid
		claims, err := svc.VerifyJWT(ctx, jwt.Token)
		require.NoError(t, err)
		assert.Equal(t, "integration_user", claims.Subject)
		assert.Equal(t, "integration@example.com", claims.SubjectEmail)
		assert.Equal(t, "Integration User", claims.SubjectFullName)
		assert.Equal(t, identityAgencyClient, claims.SubjectType)

		// 3. Revoke JWT
		err = svc.RevokeJWT(ctx, jwt.ID)
		require.NoError(t, err)

		// 4. Verify JWT is no longer valid
		_, err = svc.VerifyJWT(ctx, jwt.Token)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token revoked")
	})

	t.Run("concurrent token operations", func(t *testing.T) {
		const numTokens = 10

		// Generate tokens concurrently
		tokens := make([]*JWT, numTokens)
		errors := make(chan error, numTokens)

		for i := 0; i < numTokens; i++ {
			go func(index int) {
				jwt, err := svc.GenerateJWT(ctx,
					fmt.Sprintf("concurrent_user_%d", index),
					fmt.Sprintf("user%d@example.com", index),
					fmt.Sprintf("User %d", index),
					"test_agency_123",
					"Test Agency",
					identityAgencyUser)
				if err != nil {
					errors <- err
					return
				}
				tokens[index] = jwt
				errors <- nil
			}(i)
		}

		// Wait for all generations to complete
		for i := 0; i < numTokens; i++ {
			err := <-errors
			require.NoError(t, err)
		}

		// Verify all tokens are valid
		for _, token := range tokens {
			claims, err := svc.VerifyJWT(ctx, token.Token)
			require.NoError(t, err)
			assert.NotEmpty(t, claims.Subject)
		}

		// Revoke all tokens concurrently
		for i := 0; i < numTokens; i++ {
			go func(index int) {
				err := svc.RevokeJWT(ctx, tokens[index].ID)
				errors <- err
			}(i)
		}

		// Wait for all revocations to complete
		for i := 0; i < numTokens; i++ {
			err := <-errors
			require.NoError(t, err)
		}

		// Verify all tokens are revoked
		for _, token := range tokens {
			_, err := svc.VerifyJWT(ctx, token.Token)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "token revoked")
		}
	})
}

func TestJWTSecurity(t *testing.T) {
	t.Parallel()

	svc := newTestServiceV2(t)
	ctx := context.Background()

	t.Run("tampered token", func(t *testing.T) {
		// Generate a valid token
		jwt, err := svc.GenerateJWT(ctx, "test_user", "test@example.com", "Test User", "test_agency_123", "Test Agency", identityAgencyUser)
		require.NoError(t, err)

		// Tamper with the token by changing the last character
		lastChar := jwt.Token[len(jwt.Token)-1]
		newChar := lastChar + 1
		tamperedToken := jwt.Token[:len(jwt.Token)-1] + TokenString(newChar)

		// Verify should fail
		_, err = svc.VerifyJWT(ctx, TokenString(tamperedToken))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "jwt.ParseWithClaims")
	})

	t.Run("token with different key", func(t *testing.T) {
		// Generate a token with a different key pair
		differentPrivateKey, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
		require.NoError(t, err)

		identifier := "different_key_token"
		token := jwt.NewWithClaims(jwt.SigningMethodES384, &Claims[testIdentity]{
			RegisteredClaims: jwt.RegisteredClaims{
				ID:        identifier,
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(Expiry)),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				NotBefore: jwt.NewNumericDate(time.Now()),
				Subject:   "test_user",
			},
			SubjectType:      identityAgencyUser,
			SubjectEmail:     "test@example.com",
			SubjectFullName:  "Test User",
			OrganizationID:   "test_agency_123",
			OrganizationName: "Test Agency",
		})

		tokenString, err := token.SignedString(differentPrivateKey)
		require.NoError(t, err)

		// Store the token in DB
		err = svc.deps.DB.Set(ctx, svc.tokenKey(identifier), tokenString, Expiry)
		require.NoError(t, err)

		// Verify should fail due to wrong key
		_, err = svc.VerifyJWT(ctx, TokenString(tokenString))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "jwt.ParseWithClaims")
	})
}
