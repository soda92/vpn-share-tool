package store

import (
	"os"
	"testing"
)

func TestUserManagement(t *testing.T) {
	// Setup env
	os.Setenv("BASIC_AUTH_USER", "admin")
	os.Setenv("BASIC_AUTH_PASS", "admin")
	defer func() {
		os.Unsetenv("BASIC_AUTH_USER")
		os.Unsetenv("BASIC_AUTH_PASS")
	}()

	origPath := usersFilePath
	usersFilePath = "users_test.json"
	defer func() {
		os.Remove(usersFilePath)
		usersFilePath = origPath
		// Clear in-memory state
		usersMutex.Lock()
		users = make(map[string]User)
		usersMutex.Unlock()
	}()

	// 1. Initial State
	LoadUsers()
	usersMutex.Lock()
	usersCount := len(users)
	usersMutex.Unlock()
	if usersCount != 0 {
		t.Errorf("Expected 0 users on fresh start, got %d", usersCount)
	}

	// 2. Init default user
	InitDefaultUser()
	if !VerifyPassword("admin", "admin") {
		t.Error("Expected default admin/admin to be valid")
	}
	if MustChangePassword("admin") {
		t.Error("Expected MustChangePassword to be false when configured via environment variable")
	}

	// 3. Update password
	err := UpdatePassword("admin", "newpassword", false)
	if err != nil {
		t.Fatalf("Failed to update password: %v", err)
	}

	// Verify old password fails, new password succeeds
	if VerifyPassword("admin", "admin") {
		t.Error("Old password should not work")
	}
	if !VerifyPassword("admin", "newpassword") {
		t.Error("New password should work")
	}

	// 4. Load from file again to verify persistence
	// Clear in-memory state first
	usersMutex.Lock()
	users = make(map[string]User)
	usersMutex.Unlock()

	LoadUsers()
	if !VerifyPassword("admin", "newpassword") {
		t.Error("Loaded password should match persisted password")
	}

	// 5. Test generated reset password
	genPass := ResetPasswordAndGet("admin")
	if !VerifyPassword("admin", genPass) {
		t.Error("Generated reset password should work")
	}
	if !MustChangePassword("admin") {
		t.Error("Expected MustChangePassword to be true after reset")
	}
}
