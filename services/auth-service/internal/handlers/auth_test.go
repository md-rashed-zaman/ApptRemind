package handlers

import "testing"

func TestPasswordHashing(t *testing.T) {
	password := "pass123"
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if err := verifyPassword(hash, password); err != nil {
		t.Fatalf("verifyPassword should succeed: %v", err)
	}
	if err := verifyPassword(hash, "wrong-pass"); err == nil {
		t.Fatal("verifyPassword should fail for wrong password")
	}
}
