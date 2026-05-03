package main

import (
	"bytes"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestHashPasswordBcrypt_roundTrip(t *testing.T) {
	hash, err := hashPasswordBcrypt("secret123")
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword(hash, []byte("secret123")); err != nil {
		t.Fatal(err)
	}
}

func TestRunMain_argPassword(t *testing.T) {
	var out, errb bytes.Buffer
	code := runMain([]string{"hashpass", "hello"}, strings.NewReader(""), &out, &errb)
	if code != 0 {
		t.Fatal(errb.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(out.String()), "$2") {
		t.Fatal(out.String())
	}
}

func TestRunMain_emptyPasswordUsage(t *testing.T) {
	var out, errb bytes.Buffer
	code := runMain([]string{"hashpass"}, strings.NewReader("\n"), &out, &errb)
	if code != 1 {
		t.Fatalf("want exit 1, got %d out=%q err=%q", code, out.String(), errb.String())
	}
}
