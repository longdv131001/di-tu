// Package auth provides password hashing (PBKDF2-HMAC-SHA256) and signed session tokens.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Iterations can be lowered in tests and load tests.
var Iterations = 150000

func pbkdf2(password, salt []byte, iter, keyLen int) []byte {
	prf := hmac.New(sha256.New, password)
	var out []byte
	for block := 1; len(out) < keyLen; block++ {
		prf.Reset()
		prf.Write(salt)
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], uint32(block))
		prf.Write(b[:])
		u := prf.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iter; i++ {
			prf.Reset()
			prf.Write(u)
			u = prf.Sum(u[:0])
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}

func HashPassword(pw string) string {
	salt := make([]byte, 16)
	rand.Read(salt)
	key := pbkdf2([]byte(pw), salt, Iterations, 32)
	return fmt.Sprintf("pbkdf2$%d$%s$%s", Iterations, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

func CheckPassword(pw, encoded string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 4 || p[0] != "pbkdf2" {
		return false
	}
	iter, err := strconv.Atoi(p[1])
	if err != nil || iter < 1 || iter > 5_000_000 {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(p[2])
	want, err2 := base64.RawStdEncoding.DecodeString(p[3])
	if err1 != nil || err2 != nil {
		return false
	}
	return subtle.ConstantTimeCompare(pbkdf2([]byte(pw), salt, iter, len(want)), want) == 1
}

type Signer struct{ key []byte }

func NewSigner(secret string) *Signer { return &Signer{key: []byte(secret)} }

func RandomSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

type claims struct {
	UID int64 `json:"u"`
	Exp int64 `json:"e"`
}

var ErrToken = errors.New("invalid token")

func (s *Signer) Issue(uid int64, ttl time.Duration) string {
	b, _ := json.Marshal(claims{uid, time.Now().Add(ttl).Unix()})
	body := base64.RawURLEncoding.EncodeToString(b)
	return body + "." + s.sign(body)
}

func (s *Signer) sign(body string) string {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(body))
	return base64.RawURLEncoding.EncodeToString(m.Sum(nil))
}

func (s *Signer) Verify(tok string) (int64, error) {
	i := strings.IndexByte(tok, '.')
	if i < 0 || !hmac.Equal([]byte(s.sign(tok[:i])), []byte(tok[i+1:])) {
		return 0, ErrToken
	}
	raw, err := base64.RawURLEncoding.DecodeString(tok[:i])
	if err != nil {
		return 0, ErrToken
	}
	var c claims
	if json.Unmarshal(raw, &c) != nil || time.Now().Unix() > c.Exp || c.UID == 0 {
		return 0, ErrToken
	}
	return c.UID, nil
}
