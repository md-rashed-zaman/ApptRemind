package handlers

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"strings"

	"github.com/md-rashed-zaman/apptremind/libs/auth"
)

type TokenSigner interface {
	Sign(claims auth.Claims) (string, error)
	Verify(token string) (*auth.Claims, error)
	JWK() map[string]any
	JWKS() []map[string]any
	CanRotate() bool
	SetActiveKid(kid string) error
	RotateKey() string
}

type hs256Signer struct {
	secret string
}

func NewHS256Signer(secret string) TokenSigner {
	return &hs256Signer{secret: secret}
}

func (s *hs256Signer) Sign(claims auth.Claims) (string, error) {
	return auth.SignHS256(claims, s.secret)
}

func (s *hs256Signer) Verify(token string) (*auth.Claims, error) {
	return auth.ParseAndVerifyHS256(token, s.secret)
}

func (s *hs256Signer) JWK() map[string]any {
	return nil
}

func (s *hs256Signer) JWKS() []map[string]any {
	return nil
}

func (s *hs256Signer) CanRotate() bool {
	return false
}

func (s *hs256Signer) SetActiveKid(_ string) error {
	return errors.New("rotation not supported")
}

func (s *hs256Signer) RotateKey() string {
	return ""
}

type rs256Signer struct {
	privateKey *rsa.PrivateKey
	kid        string
	publicJWK  map[string]any
	publicKey  *rsa.PublicKey
}

func NewRS256Signer(pemBytes []byte, kid string) (TokenSigner, error) {
	key, err := parseRSAPrivateKey(pemBytes)
	if err != nil {
		return nil, err
	}
	if kid == "" {
		kid = keyIDFromPublicKey(&key.PublicKey)
	}
	return &rs256Signer{
		privateKey: key,
		kid:        kid,
		publicJWK:  buildPublicJWK(&key.PublicKey, kid),
		publicKey:  &key.PublicKey,
	}, nil
}

func ParseRS256KeySet(pemBlobs string) (map[string]*rsa.PrivateKey, error) {
	keys := map[string]*rsa.PrivateKey{}
	for _, block := range splitPEMBlocks(pemBlobs) {
		key, err := parseRSAPrivateKey([]byte(block))
		if err != nil {
			return nil, err
		}
		kid := keyIDFromPublicKey(&key.PublicKey)
		keys[kid] = key
	}
	if len(keys) == 0 {
		return nil, errors.New("no valid rsa keys found")
	}
	return keys, nil
}

func (s *rs256Signer) Sign(claims auth.Claims) (string, error) {
	header := map[string]string{
		"alg": "RS256",
		"typ": "JWT",
		"kid": s.kid,
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
	sig, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	signature := base64.RawURLEncoding.EncodeToString(sig)
	return unsigned + "." + signature, nil
}

func (s *rs256Signer) JWK() map[string]any {
	return s.publicJWK
}

func (s *rs256Signer) Verify(token string) (*auth.Claims, error) {
	return auth.VerifyRS256(token, s.publicKey)
}

func (s *rs256Signer) JWKS() []map[string]any {
	return []map[string]any{s.publicJWK}
}

func (s *rs256Signer) CanRotate() bool {
	return false
}

func (s *rs256Signer) SetActiveKid(_ string) error {
	return errors.New("rotation not supported")
}

func (s *rs256Signer) RotateKey() string {
	return ""
}

type RotatingSigner struct {
	activeKid string
	keys      map[string]*rs256Signer
	rotateKey string
}

func NewRotatingRS256Signer(keys map[string]*rsa.PrivateKey, activeKid string) (TokenSigner, error) {
	if len(keys) == 0 {
		return nil, errors.New("no keys provided")
	}
	s := &RotatingSigner{
		activeKid: activeKid,
		keys:      map[string]*rs256Signer{},
	}
	for kid, key := range keys {
		if kid == "" || key == nil {
			continue
		}
		s.keys[kid] = &rs256Signer{
			privateKey: key,
			kid:        kid,
			publicJWK:  buildPublicJWK(&key.PublicKey, kid),
			publicKey:  &key.PublicKey,
		}
	}
	if s.activeKid == "" {
		for kid := range s.keys {
			s.activeKid = kid
			break
		}
	}
	if s.activeKid == "" || s.keys[s.activeKid] == nil {
		return nil, errors.New("active kid not found")
	}
	return s, nil
}

func (s *RotatingSigner) Sign(claims auth.Claims) (string, error) {
	return s.keys[s.activeKid].Sign(claims)
}

func (s *RotatingSigner) Verify(token string) (*auth.Claims, error) {
	header, err := auth.ParseHeader(token)
	if err != nil {
		return nil, err
	}
	if header.Kid == "" {
		return nil, auth.ErrInvalidToken
	}
	key := s.keys[header.Kid]
	if key == nil {
		return nil, auth.ErrInvalidToken
	}
	return key.Verify(token)
}

func (s *RotatingSigner) JWK() map[string]any {
	return s.keys[s.activeKid].JWK()
}

func (s *RotatingSigner) JWKS() []map[string]any {
	out := make([]map[string]any, 0, len(s.keys))
	for _, key := range s.keys {
		out = append(out, key.JWK())
	}
	return out
}

func (s *RotatingSigner) CanRotate() bool {
	return true
}

func (s *RotatingSigner) SetActiveKid(kid string) error {
	if s.keys[kid] == nil {
		return errors.New("unknown kid")
	}
	s.activeKid = kid
	return nil
}

func (s *RotatingSigner) RotateKey() string {
	return s.rotateKey
}

func (s *RotatingSigner) SetRotateKey(key string) {
	s.rotateKey = key
}

func parseRSAPrivateKey(pemBytes []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("invalid pem")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
	}
	return nil, errors.New("unsupported private key")
}

func buildPublicJWK(pub *rsa.PublicKey, kid string) map[string]any {
	n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
	e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
	return map[string]any{
		"kty": "RSA",
		"kid": kid,
		"alg": "RS256",
		"use": "sig",
		"n":   n,
		"e":   e,
	}
}

func keyIDFromPublicKey(pub *rsa.PublicKey) string {
	sum := sha256.Sum256(pub.N.Bytes())
	return base64.RawURLEncoding.EncodeToString(sum[:8])
}

func splitPEMBlocks(raw string) []string {
	var blocks []string
	var current strings.Builder
	inBlock := false
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "-----BEGIN ") {
			inBlock = true
			current.Reset()
		}
		if inBlock {
			current.WriteString(line)
			current.WriteString("\n")
		}
		if strings.HasPrefix(line, "-----END ") && inBlock {
			inBlock = false
			blocks = append(blocks, current.String())
		}
	}
	return blocks
}
