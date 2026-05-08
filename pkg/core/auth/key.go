package auth

import (
	"crypto/ecdsa"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/core/secrets"
)

func FetchJWTKeys() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	privateKeyString := secrets.Require("AUTH_JWT_PRIVATE_KEY")
	publicKeyString := secrets.Require("AUTH_JWT_PUBLIC_KEY")

	privateKey, err := jwt.ParseECPrivateKeyFromPEM([]byte(privateKeyString))
	if err != nil {
		return nil, nil, errors.Wrap(err, "jwt.ParseECPrivateKeyFromPEM")
	}

	publicKey, err := jwt.ParseECPublicKeyFromPEM([]byte(publicKeyString))
	if err != nil {
		return nil, nil, errors.Wrap(err, "jwt.ParseECPublicKeyFromPEM")
	}

	return privateKey, publicKey, nil
}
