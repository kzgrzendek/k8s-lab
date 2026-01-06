// Package testutil provides testing utilities and helpers for NOVA tests.
package testutil

import (
	"context"
	"testing"
	"time"
)

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if err != nil {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected no error but got: %v. %v", err, msgAndArgs)
		} else {
			t.Fatalf("Expected no error but got: %v", err)
		}
	}
}

// AssertError fails the test if err is nil.
func AssertError(t *testing.T, err error, msgAndArgs ...interface{}) {
	t.Helper()
	if err == nil {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected error but got nil. %v", msgAndArgs)
		} else {
			t.Fatal("Expected error but got nil")
		}
	}
}

// AssertEqual fails the test if expected != actual.
func AssertEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if expected != actual {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected %v but got %v. %v", expected, actual, msgAndArgs)
		} else {
			t.Fatalf("Expected %v but got %v", expected, actual)
		}
	}
}

// AssertNotEqual fails the test if expected == actual.
func AssertNotEqual(t *testing.T, expected, actual interface{}, msgAndArgs ...interface{}) {
	t.Helper()
	if expected == actual {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected values to be different but both were %v. %v", expected, msgAndArgs)
		} else {
			t.Fatalf("Expected values to be different but both were %v", expected)
		}
	}
}

// AssertContains fails the test if the string doesn't contain the substring.
func AssertContains(t *testing.T, str, substr string, msgAndArgs ...interface{}) {
	t.Helper()
	if !contains(str, substr) {
		if len(msgAndArgs) > 0 {
			t.Fatalf("Expected string to contain %q but it didn't. String: %q. %v", substr, str, msgAndArgs)
		} else {
			t.Fatalf("Expected string to contain %q but it didn't. String: %q", substr, str)
		}
	}
}

// NewTestContext creates a context with a reasonable timeout for tests.
func NewTestContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

// NewTestContextWithCancel creates a cancellable context for tests.
func NewTestContextWithCancel() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

// contains checks if a string contains a substring (helper for AssertContains).
func contains(str, substr string) bool {
	for i := 0; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
