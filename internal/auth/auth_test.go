package auth

import (
	"context"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestCheckPassword_plainMatch(t *testing.T) {
	if !CheckPassword("secret", "secret", "") {
		t.Error("CheckPassword(secret, secret, \"\"): want true")
	}
	if CheckPassword("wrong", "secret", "") {
		t.Error("CheckPassword(wrong, secret, \"\"): want false")
	}
}

func TestCheckPassword_bcrypt(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("mypass"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	if !CheckPassword("mypass", "", string(hash)) {
		t.Error("CheckPassword with valid bcrypt: want true")
	}
	if CheckPassword("wrong", "", string(hash)) {
		t.Error("CheckPassword with wrong password and bcrypt: want false")
	}
}

func TestCreateSession_GetSession_roundtrip(t *testing.T) {
	ctx := context.Background()
	id, err := CreateSession(ctx, "testuser", "admin")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if id == "" {
		t.Fatal("CreateSession: want non-empty ID")
	}
	username, role, ok := GetSession(ctx, id)
	if !ok {
		t.Fatal("GetSession: want ok=true")
	}
	if username != "testuser" || role != "admin" {
		t.Errorf("GetSession: want testuser/admin, got %q/%q", username, role)
	}
}

func TestGetSession_emptyID_returnsFalse(t *testing.T) {
	ctx := context.Background()
	_, _, ok := GetSession(ctx, "")
	if ok {
		t.Error("GetSession with empty ID: want ok=false")
	}
}

func TestDeleteSession_removesSession(t *testing.T) {
	ctx := context.Background()
	id, err := CreateSession(ctx, "deluser", "viewer")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	DeleteSession(ctx, id)
	_, _, ok := GetSession(ctx, id)
	if ok {
		t.Error("GetSession after DeleteSession: want ok=false")
	}
}
