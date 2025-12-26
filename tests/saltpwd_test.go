package tests

import (
	"service-platform/internal/pkg/fun"
	"testing"
)

// TestGenerateSaltedPassword tests the GenerateSaltedPassword and IsPasswordMatched functions
func TestGenerateSaltedPassword(t *testing.T) {
	pwd := "password123"
	saltedPwd := fun.GenerateSaltedPassword(pwd)
	if len(saltedPwd) == 0 {
		t.Errorf("Expected non-empty salted password, got empty string")
	}

	t.Logf("generated salted pwd from %s = %s", pwd, saltedPwd)

	isMatched := fun.IsPasswordMatched(pwd, saltedPwd)
	if !isMatched {
		t.Errorf("Expected password to match, but it did not")
	}
}
