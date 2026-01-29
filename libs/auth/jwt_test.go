package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

func TestHS256RoundTrip(t *testing.T) {
	claims := Claims{
		Sub:        "user-1",
		BusinessID: "biz-1",
		Role:       "owner",
		Iat:        time.Now().Unix(),
		Exp:        time.Now().Add(1 * time.Hour).Unix(),
	}
	secret := "test-secret"

	token, err := SignHS256(claims, secret)
	if err != nil {
		t.Fatalf("SignHS256 failed: %v", err)
	}
	parsed, err := ParseAndVerifyHS256(token, secret)
	if err != nil {
		t.Fatalf("ParseAndVerifyHS256 failed: %v", err)
	}
	if parsed.Sub != claims.Sub || parsed.BusinessID != claims.BusinessID || parsed.Role != claims.Role {
		t.Fatalf("claims mismatch: got %+v", parsed)
	}
	if _, err := ParseAndVerifyHS256(token, "wrong-secret"); err == nil {
		t.Fatal("expected verification error with wrong secret")
	}
}

func TestRS256Verify(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey failed: %v", err)
	}
	claims := Claims{
		Sub:        "user-2",
		BusinessID: "biz-2",
		Role:       "admin",
		Iat:        time.Now().Unix(),
		Exp:        time.Now().Add(1 * time.Hour).Unix(),
	}

	token, err := signRS256(claims, key, "kid-1")
	if err != nil {
		t.Fatalf("rs256 Sign failed: %v", err)
	}
	parsed, err := VerifyRS256(token, &key.PublicKey)
	if err != nil {
		t.Fatalf("VerifyRS256 failed: %v", err)
	}
	if parsed.Sub != claims.Sub || parsed.BusinessID != claims.BusinessID || parsed.Role != claims.Role {
		t.Fatalf("claims mismatch: got %+v", parsed)
	}
}

func signRS256(claims Claims, key *rsa.PrivateKey, kid string) (string, error) {
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	}
	if kid != "" {
		header["kid"] = kid
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	payloadJSON, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	headerEnc := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadEnc := base64.RawURLEncoding.EncodeToString(payloadJSON)
	unsigned := headerEnc + "." + payloadEnc
	hash := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	signature := base64.RawURLEncoding.EncodeToString(sig)
	return unsigned + "." + signature, nil
}
