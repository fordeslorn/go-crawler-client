package auth

import (
	"crypto/rsa"
	"crypto/x509"
	_ "embed"
	"encoding/pem"
	"errors"

	"github.com/golang-jwt/jwt/v5"
)

//go:embed public.pem
var publicKeyPEM []byte

// TaskClaims defines the custom claims for crawler tasks
type TaskClaims struct {
	UserID string `json:"user_id"`
	Mode   string `json:"mode"`
	jwt.RegisteredClaims
}

// TokenValidator handles JWT validation
type TokenValidator struct {
	publicKey *rsa.PublicKey
}

// NewTokenValidator creates a new TokenValidator
func NewTokenValidator() (*TokenValidator, error) {
	tv := &TokenValidator{}

	// Load Public Key
	block, _ := pem.Decode(publicKeyPEM)
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing the public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	var ok bool
	tv.publicKey, ok = pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}

	return tv, nil
}

// ValidateTaskToken validates a JWT token and returns the claims
func (tv *TokenValidator) ValidateTaskToken(tokenString string) (*TaskClaims, error) {
	if tv.publicKey == nil {
		return nil, errors.New("public key not loaded")
	}

	token, err := jwt.ParseWithClaims(tokenString, &TaskClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return tv.publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TaskClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}
