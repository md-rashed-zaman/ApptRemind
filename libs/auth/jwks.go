package auth

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"sync"
	"time"
)

var ErrKeyNotFound = errors.New("jwks key not found")

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwks struct {
	Keys []jwk `json:"keys"`
}

type JWKSClient struct {
	url     string
	ttl     time.Duration
	mu      sync.Mutex
	expires time.Time
	keys    map[string]*rsa.PublicKey
}

func NewJWKSClient(url string, ttl time.Duration) *JWKSClient {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &JWKSClient{url: url, ttl: ttl, keys: map[string]*rsa.PublicKey{}}
}

func (c *JWKSClient) Get(keyID string) (*rsa.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if time.Now().Before(c.expires) {
		if key, ok := c.keys[keyID]; ok {
			return key, nil
		}
	}

	if err := c.refresh(); err != nil {
		if key, ok := c.keys[keyID]; ok {
			return key, nil
		}
		return nil, err
	}

	if key, ok := c.keys[keyID]; ok {
		return key, nil
	}
	return nil, ErrKeyNotFound
}

func (c *JWKSClient) refresh() error {
	resp, err := http.Get(c.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("jwks endpoint returned non-200")
	}

	var data jwks
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	keys := map[string]*rsa.PublicKey{}
	for _, k := range data.Keys {
		if k.Kty != "RSA" || k.N == "" || k.E == "" || k.Kid == "" {
			continue
		}
		pub, err := jwkToPublicKey(k)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}

	c.keys = keys
	c.expires = time.Now().Add(c.ttl)
	return nil
}

func jwkToPublicKey(k jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes).Int64()
	if e > int64(^uint(0)>>1) {
		return nil, errors.New("invalid jwk exponent")
	}

	return &rsa.PublicKey{
		N: n,
		E: int(e),
	}, nil
}
