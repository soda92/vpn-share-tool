package store

import (
	"os"
	"testing"
)

func TestUserManagement(t *testing.T) {
	// Setup
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

	// 3. Update password
	err := UpdatePassword("admin", "newpassword")
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
}
