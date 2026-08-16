package users

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestPasswordHashUsesBcrypt(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "$2") {
		t.Fatalf("password hash is not bcrypt: %q", hash)
	}
	if !verifyPassword("correct horse battery staple", hash) {
		t.Fatal("bcrypt password did not verify")
	}
	if verifyPassword("wrong password", hash) {
		t.Fatal("incorrect password verified")
	}
}

func TestValidatePasswordMigratesLegacyHash(t *testing.T) {
	store := NewUserStore(t.TempDir())
	user, _, err := store.CreateUser("legacy-user", "temporary", RoleUser)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	legacyPassword := "legacy-password"
	legacyDigest := sha256.Sum256([]byte(legacyPassword))
	user.PasswordHash = hex.EncodeToString(legacyDigest[:])

	validated, ok := store.ValidatePassword("legacy-user", legacyPassword)
	if !ok || validated == nil {
		t.Fatal("legacy password was not accepted")
	}
	if isLegacyPasswordHash(validated.PasswordHash) {
		t.Fatal("legacy password hash was not upgraded")
	}
	if !strings.HasPrefix(validated.PasswordHash, "$2") {
		t.Fatalf("upgraded password hash is not bcrypt: %q", validated.PasswordHash)
	}
}
