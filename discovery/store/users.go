package store

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	Username           string `json:"username"`
	PasswordHash       string `json:"password_hash"`
	MustChangePassword bool   `json:"must_change_password"`
}

var usersFilePath = "users.json"

var (
	users      = make(map[string]User)
	usersMutex = &sync.Mutex{}
)

func LoadUsers() {
	usersMutex.Lock()
	defer usersMutex.Unlock()

	data, err := os.ReadFile(usersFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Println("No users file found, starting fresh.")
			return
		}
		log.Printf("Error reading users file: %v", err)
		return
	}

	if err := json.Unmarshal(data, &users); err != nil {
		log.Printf("Error unmarshaling users: %v", err)
	}
	log.Printf("Loaded %d users from %s", len(users), usersFilePath)
}

func SaveUsers() error {
	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users: %w", err)
	}

	if err := os.WriteFile(usersFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write users file: %w", err)
	}
	return nil
}

func VerifyPassword(username, password string) bool {
	usersMutex.Lock()
	user, exists := users[username]
	usersMutex.Unlock()

	if !exists {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	return err == nil
}

func MustChangePassword(username string) bool {
	usersMutex.Lock()
	user, exists := users[username]
	usersMutex.Unlock()

	if !exists {
		return false
	}

	return user.MustChangePassword
}

func UpdatePassword(username, password string, mustChange bool) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	usersMutex.Lock()
	users[username] = User{
		Username:           username,
		PasswordHash:       string(hash),
		MustChangePassword: mustChange,
	}
	usersMutex.Unlock()

	return SaveUsers()
}

func generateRandomPassword(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "admin12345" // safe fallback
	}
	for i := range b {
		b[i] = chars[b[i]%byte(len(chars))]
	}
	return string(b)
}

func ResetPasswordAndGet(username string) string {
	genPass := generateRandomPassword(12)
	if err := UpdatePassword(username, genPass, true); err != nil {
		log.Printf("Error resetting password for user %s: %v", username, err)
	}
	return genPass
}

func InitDefaultUser() {
	usersMutex.Lock()
	hasUsers := len(users) > 0
	usersMutex.Unlock()

	if !hasUsers {
		defaultUser := os.Getenv("BASIC_AUTH_USER")
		if defaultUser == "" {
			defaultUser = "admin"
		}
		defaultPass := os.Getenv("BASIC_AUTH_PASS")
		mustChange := false
		if defaultPass == "" {
			defaultPass = generateRandomPassword(12)
			mustChange = true
		}
		if err := UpdatePassword(defaultUser, defaultPass, mustChange); err != nil {
			log.Printf("Failed to initialize default user: %v", err)
			return
		}

		log.Printf("==================================================")
		log.Printf("Initialized default user: %s", defaultUser)
		if mustChange {
			log.Printf("Generated initial password: %s", defaultPass)
			log.Printf("Please log in and change this password immediately.")
		} else {
			log.Printf("Password configured from BASIC_AUTH_PASS environment variable.")
		}
		log.Printf("==================================================")
	}
}
