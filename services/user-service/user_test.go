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

func TestUserValidate(t *testing.T) {
	tests := []struct {
		name    string
		user    *User
		wantErr bool
	}{
		{
			name: "complete user",
			user: &User{Email: "a@example.com", Name: "A", PasswordHash: "hash"},
		},
		{
			name:    "no password",
			user:    &User{Email: "a@example.com", Name: "A"},
			wantErr: true,
		},
		{
			name:    "no email",
			user:    &User{Name: "A", PasswordHash: "hash"},
			wantErr: true,
		},
		{
			name:    "no name",
			user:    &User{Email: "a@example.com", PasswordHash: "hash"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.user.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
