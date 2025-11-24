package auth

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go-crawler-client/config"
	"io"
	"net/http"
	"os"
	"strings"
	"syscall"

	"golang.org/x/term"
)

type LoginResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token"`
}

// PerformLogin prompts the user for credentials and retrieves a long-lived token
func PerformLogin() error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("---------------------------------------------------------")
	fmt.Println("Authentication Required")
	fmt.Println("Please log in to generate a long-lived token for this client.")
	fmt.Println("---------------------------------------------------------")

	fmt.Print("Email: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Password: ")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return fmt.Errorf("failed to read password: %v", err)
	}
	password := string(bytePassword)
	fmt.Println() // Newline after password input

	loginURL := fmt.Sprintf("%s/api/v1/auth/crawler-login", config.GlobalConfig.ServerURL)

	payload := map[string]string{
		"email":    email,
		"password": password,
	}
	jsonPayload, _ := json.Marshal(payload)

	resp, err := http.Post(loginURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var loginResp LoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return fmt.Errorf("failed to parse response: %v", err)
	}

	if !loginResp.Success {
		return fmt.Errorf("login failed: %s", loginResp.Message)
	}

	fmt.Println("Login successful! Token acquired.")
	return config.UpdateToken(loginResp.Token)
}
