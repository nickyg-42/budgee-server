package util

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/plaid/plaid-go/v41/plaid"
)

// The code in this file was directly copied from plaid docs
// https://plaid.com/docs/api/webhooks/webhook-verification/

func jwkToECDSAPublicKey(jwk *plaid.JWKPublicKey) (*ecdsa.PublicKey, error) {
	if jwk == nil || jwk.X == "" || jwk.Y == "" ||
		jwk.Kty != "EC" ||
		jwk.Crv != "P-256" {
		return nil, errors.New("invalid/unsupported JWK")
	}
	xBytes, err := base64.RawURLEncoding.DecodeString(jwk.X)
	if err != nil {
		return nil, fmt.Errorf("decode x: %w", err)
	}
	yBytes, err := base64.RawURLEncoding.DecodeString(jwk.Y)
	if err != nil {
		return nil, fmt.Errorf("decode y: %w", err)
	}
	return &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(xBytes),
		Y:     new(big.Int).SetBytes(yBytes),
	}, nil
}

var (
	jwkCache = map[string]*plaid.JWKPublicKey{}
	maxAge   = 5 * time.Minute
)

func VerifyWebhook(ctx context.Context, client *plaid.APIClient, webhookBody []byte, headers map[string]string) (bool, error) {
	tokenString := getHeaderCI(headers, "Plaid-Verification")
	if tokenString == "" {
		return false, errors.New("missing Plaid-Verification header")
	}

	// Decode JWT header (unverified) to extract alg and kid
	parser := jwt.NewParser(jwt.WithLeeway(30 * time.Second))

	unverified, _, err := parser.ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		return false, fmt.Errorf("parse unverified token: %w", err)
	}
	if unverified.Method.Alg() != jwt.SigningMethodES256.Alg() {
		return false, fmt.Errorf("unexpected alg %q (want ES256)", unverified.Method.Alg())
	}
	kid, _ := unverified.Header["kid"].(string)
	if kid == "" {
		return false, errors.New("missing kid in JWT header")
	}

	// Get verification key for kid via /webhook_verification_key/get
	jwk, err := getJWK(ctx, client, kid)
	if err != nil {
		return false, fmt.Errorf("get JWK: %w", err)
	}
	pubKey, err := jwkToECDSAPublicKey(jwk)
	if err != nil {
		return false, fmt.Errorf("jwk->ecdsa: %w", err)
	}

	// Verify JWT signature
	claims := jwt.MapClaims{}
	token, err := parser.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodES256 {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pubKey, nil
	})
	if err != nil || !token.Valid {
		return false, fmt.Errorf("invalid token: %w", err)
	}

	// Verify that the webhook is not more than 5 minutes old
	iatVal, ok := claims["iat"]
	if !ok {
		return false, errors.New("missing iat")
	}
	var iat time.Time
	switch v := iatVal.(type) {
	case float64:
		iat = time.Unix(int64(v), 0)
	case int64:
		iat = time.Unix(v, 0)
	default:
		return false, errors.New("invalid iat type")
	}
	if time.Since(iat) > maxAge {
		return false, errors.New("token too old (>5m)")
	}

	// Verify body hash integrity
	wantHash, ok := claims["request_body_sha256"].(string)
	if !ok || wantHash == "" {
		return false, errors.New("missing request_body_sha256")
	}
	sum := sha256.Sum256(webhookBody)
	gotHex := strings.ToLower(hex.EncodeToString(sum[:]))
	if subtle.ConstantTimeCompare([]byte(gotHex), []byte(strings.ToLower(wantHash))) != 1 {
		return false, errors.New("body hash mismatch")
	}

	return true, nil
}

func getHeaderCI(h map[string]string, name string) string {
	lname := strings.ToLower(name)
	for k, v := range h {
		if strings.ToLower(k) == lname {
			return v
		}
	}
	return ""
}

func getJWK(ctx context.Context, client *plaid.APIClient, kid string) (*plaid.JWKPublicKey, error) {
	if key, ok := jwkCache[kid]; ok && key != nil {
		return key, nil
	}
	req := *plaid.NewWebhookVerificationKeyGetRequest(kid)
	resp, _, err := client.PlaidApi.WebhookVerificationKeyGet(ctx).
		WebhookVerificationKeyGetRequest(req).
		Execute()
	if err != nil {
		return nil, err
	}
	key := resp.GetKey()
	if key.Kid == kid {
		jwkCache[kid] = &key
	}
	return &key, nil
}
