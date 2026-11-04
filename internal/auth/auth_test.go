package auth

import (
	"testing"
	"time"
)

func TestPassword(t *testing.T) {
	Iterations = 1000
	h := HashPassword("s3cret!")
	if !CheckPassword("s3cret!", h) || CheckPassword("wrong", h) || CheckPassword("s3cret!", "junk") {
		t.Fatal("password check")
	}
	if HashPassword("s3cret!") == h {
		t.Fatal("salt must differ")
	}
}

func TestPBKDF2Vector(t *testing.T) {
	// RFC 7914 section 11 test vector for PBKDF2-HMAC-SHA256, c=1.
	got := pbkdf2([]byte("passwd"), []byte("salt"), 1, 64)
	want := "55ac046e56e3089fec1691c22544b605f94185216dde0465e68b9d57c20dacbc"
	if hex := toHex(got[:32]); hex != want {
		t.Fatalf("got %s", hex)
	}
}

func toHex(b []byte) string {
	const d = "0123456789abcdef"
	s := make([]byte, 0, len(b)*2)
	for _, c := range b {
		s = append(s, d[c>>4], d[c&15])
	}
	return string(s)
}

func TestToken(t *testing.T) {
	s := NewSigner("k")
	tok := s.Issue(42, time.Hour)
	if uid, err := s.Verify(tok); err != nil || uid != 42 {
		t.Fatal(uid, err)
	}
	if _, err := NewSigner("other").Verify(tok); err == nil {
		t.Fatal("wrong key accepted")
	}
	if _, err := s.Verify(tok + "x"); err == nil {
		t.Fatal("tampered accepted")
	}
	if _, err := s.Verify(s.Issue(1, -time.Second)); err == nil {
		t.Fatal("expired accepted")
	}
}
