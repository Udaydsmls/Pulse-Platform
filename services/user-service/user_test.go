package main

import "testing"

func TestPasswordHashing(t *testing.T) {
	hash, err := hashPassword("correct-horse-battery")
	if err != nil {
		t.Fatalf("hashPassword() error = %v", err)
	}

	if hash == "correct-horse-battery" {
		t.Error("password was stored in plain text")
	}
	if !checkPassword(hash, "correct-horse-battery") {
		t.Error("checkPassword() rejected the correct password")
	}
	if checkPassword(hash, "wrong-password") {
		t.Error("checkPassword() accepted the wrong password")
	}
}
