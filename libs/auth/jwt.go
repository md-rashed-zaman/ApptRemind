package auth

import (
	"crypto"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	Sub        string `json:"sub"`
	BusinessID string `json:"business_id"`
	Role       string `json:"role"`
	Exp        int64  `json:"exp"`
	Iat        int64  `json:"iat"`
}

type Header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

func ParseJWTNoVerify(token string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, ErrInvalidToken
	}

	return &claims, nil
}

func ParseHeader(token string) (*Header, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var header Header
	if err := json.Unmarshal(raw, &header); err != nil {
		return nil, ErrInvalidToken
	}
	return &header, nil
}

func SignHS256(claims Claims, secret string) (string, error) {
	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
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
	signature := hmacSHA256(unsigned, secret)
	return unsigned + "." + signature, nil
}

func ParseAndVerifyHS256(token, secret string) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	unsigned := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(hmacSHA256(unsigned, secret))) {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}

func hmacSHA256(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func VerifyRS256(token string, pubKey crypto.PublicKey) (*Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}
	unsigned := parts[0] + "." + parts[1]
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	rsaKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return nil, ErrInvalidToken
	}

	hash := sha256.Sum256([]byte(unsigned))
	if err := rsa.VerifyPKCS1v15(rsaKey, crypto.SHA256, hash[:], sig); err != nil {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}
	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if claims.Exp > 0 && time.Now().Unix() > claims.Exp {
		return nil, ErrInvalidToken
	}
	return &claims, nil
}
