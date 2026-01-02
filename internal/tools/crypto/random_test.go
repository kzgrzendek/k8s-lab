package crypto

import (
	"encoding/base64"
	"testing"
)

func TestGenerateRandomPassword(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"default length", 16},
		{"longer password", 32},
		{"minimum enforced", 8}, // Should be upgraded to 16
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			password, err := GenerateRandomPassword(tt.length)
			if err != nil {
				t.Fatalf("GenerateRandomPassword() error = %v", err)
			}

			// Check that password is not empty
			if password == "" {
				t.Error("GenerateRandomPassword() returned empty string")
			}

			// Check that password is valid base64
			_, err = base64.RawURLEncoding.DecodeString(password)
			if err != nil {
				t.Errorf("GenerateRandomPassword() returned invalid base64: %v", err)
			}

			// Check minimum length (base64 encoding expands the length)
			if len(password) < 20 {
				t.Errorf("GenerateRandomPassword() password too short: %d", len(password))
			}

			t.Logf("Generated password: %s (length: %d)", password, len(password))
		})
	}
}

func TestGenerateRandomPasswordUniqueness(t *testing.T) {
	// Generate multiple passwords and ensure they're unique
	passwords := make(map[string]bool)
	iterations := 100

	for i := 0; i < iterations; i++ {
		password, err := GenerateRandomPassword(16)
		if err != nil {
			t.Fatalf("GenerateRandomPassword() error = %v", err)
		}

		if passwords[password] {
			t.Errorf("GenerateRandomPassword() generated duplicate password: %s", password)
		}
		passwords[password] = true
	}

	if len(passwords) != iterations {
		t.Errorf("Expected %d unique passwords, got %d", iterations, len(passwords))
	}
}

func TestGenerateRandomBytes(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{"16 bytes", 16},
		{"32 bytes", 32},
		{"64 bytes", 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bytes, err := GenerateRandomBytes(tt.length)
			if err != nil {
				t.Fatalf("GenerateRandomBytes() error = %v", err)
			}

			if len(bytes) != tt.length {
				t.Errorf("GenerateRandomBytes() length = %d, want %d", len(bytes), tt.length)
			}
		})
	}
}

func TestGenerateRandomHex(t *testing.T) {
	length := 16
	hex, err := GenerateRandomHex(length)
	if err != nil {
		t.Fatalf("GenerateRandomHex() error = %v", err)
	}

	// Hex encoding doubles the length
	expectedLen := length * 2
	if len(hex) != expectedLen {
		t.Errorf("GenerateRandomHex() length = %d, want %d", len(hex), expectedLen)
	}

	// Check that it's valid hex (all characters are 0-9a-f)
	for _, c := range hex {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("GenerateRandomHex() contains invalid character: %c", c)
		}
	}
}
