package socket

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"go-crawler-client/config"
	"go-crawler-client/internal/crawler"
	"go-crawler-client/internal/model"
	"go-crawler-client/internal/service"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Client struct {
	BackendURL string
	Token      string // This is the User's Auth Token (JWT) to identify the WS connection
	Conn       *websocket.Conn
}

func NewClient(backendURL, token string) *Client {
	return &Client{
		BackendURL: backendURL,
		Token:      token,
	}
}

func (c *Client) Connect() {
	u, err := url.Parse(c.BackendURL)
	if err != nil {
		log.Fatal("Invalid Backend URL:", err)
	}
	u.Path = "/api/v1/ws"

	// Add token to query for authentication
	// The backend expects the JWT token in the query parameter "token" (or header, but query is easier for WS)
	// Wait, my backend implementation checks `c.GetString("userID")`.
	// This means the AuthMiddleware must have run.
	// The AuthMiddleware usually checks Authorization header.
	// Standard WS libraries support headers.

	// Let's try sending Authorization header first.
	// If that fails, we might need to adjust backend middleware to check query param.
	// But for now, let's assume we can send headers.

	// Handle scheme (http -> ws, https -> wss)
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}

	// Add token to query params as a fallback/primary method for WS
	q := u.Query()
	q.Set("token", c.Token)
	u.RawQuery = q.Encode()

	headers := http.Header{}
	headers.Add("Authorization", "Bearer "+c.Token)

	for {
		log.Printf("Connecting to %s...", u.String())
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), headers)
		if err != nil {
			log.Printf("Connection failed: %v. Retrying in 5s...", err)
			time.Sleep(5 * time.Second)
			continue
		}

		c.Conn = conn
		log.Println("\033[32mConnected to Backend via WebSocket!\033[0m")

		// Listen loop
		c.listen()

		// If listen returns, it means disconnected
		log.Println("Disconnected. Reconnecting...")
		time.Sleep(3 * time.Second)
	}
}

func (c *Client) listen() {
	defer c.Conn.Close()
	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			return
		}
		c.handleMessage(message)
	}
}

type Message struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type StartTaskPayload struct {
	PixivUserID string `json:"pixiv_user_id"`
	Cookie      string `json:"cookie"`
	Token       string `json:"token"` // Task Token
	Mode        string `json:"mode"`
}

func (c *Client) handleMessage(data []byte) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		log.Println("Invalid message format:", err)
		return
	}

	switch msg.Type {
	case "start_task":
		var payload StartTaskPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			log.Println("Invalid start_task payload:", err)
			return
		}
		c.handleStartTask(msg.ID, payload)
	case "get_status":
		c.handleGetStatus(msg.ID, msg.Payload)
	case "get_logs":
		c.handleGetLogs(msg.ID, msg.Payload)
	case "get_config":
		c.handleGetConfig(msg.ID)
	case "get_avatar":
		c.handleGetAvatar(msg.ID, msg.Payload)
	case "get_image":
		c.handleGetImage(msg.ID, msg.Payload)
	default:
		log.Println("Unknown message type:", msg.Type)
	}
}

func (c *Client) handleStartTask(reqID string, req StartTaskPayload) {
	log.Printf("Received Start Task: Mode=%s, User=%s", req.Mode, req.PixivUserID)

	// 1. Get User Info
	userInfo, err := crawler.GetUserInfo(req.PixivUserID, req.Cookie)
	if err != nil {
		log.Println("Failed to get user info:", err)
		c.sendResponse(reqID, map[string]interface{}{
			"success": false,
			"message": "Failed to get user info: " + err.Error(),
		})
		return
	}

	// 2. Generate Task ID
	taskID := uuid.New().String()

	// 3. Create Task
	task, err := service.GlobalTaskManager.AddTask(taskID, req.Mode, userInfo)
	if err != nil {
		log.Println("Failed to create task:", err)
		c.sendResponse(reqID, map[string]interface{}{
			"success": false,
			"message": "Failed to create task: " + err.Error(),
		})
		return
	}

	// 4. Start Crawler
	go crawler.StartCrawler(task, req.Cookie)

	// 5. Send Response
	c.sendResponse(reqID, model.StartTaskResponse{
		Status:   "running",
		TaskID:   taskID,
		UserInfo: userInfo,
	})
}

func (c *Client) sendResponse(reqID string, payload interface{}) {
	msg := Message{
		ID:      reqID,
		Type:    "response",
		Payload: nil,
	}
	// Marshal payload manually because Message.Payload is json.RawMessage
	data, _ := json.Marshal(payload)
	msg.Payload = data

	respData, _ := json.Marshal(msg)
	c.Conn.WriteMessage(websocket.TextMessage, respData)
}

func (c *Client) handleGetStatus(reqID string, payload json.RawMessage) {
	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return
	}

	task, ok := service.GlobalTaskManager.GetTask(req.TaskID)
	if !ok {
		c.sendResponse(reqID, map[string]string{"error": "Task not found"})
		return
	}
	c.sendResponse(reqID, task.GetSnapshot())
}

func (c *Client) handleGetLogs(reqID string, payload json.RawMessage) {
	var req struct {
		TaskID string `json:"task_id"`
		Tail   string `json:"tail"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return
	}

	task, ok := service.GlobalTaskManager.GetTask(req.TaskID)
	if !ok {
		c.sendResponse(reqID, map[string]string{"error": "Task not found"})
		return
	}

	tail := 50
	// simple atoi, ignore error
	if req.Tail != "" {
		if t, err := strconv.Atoi(req.Tail); err == nil {
			tail = t
		}
	}
	logs, total := task.Logger.GetLogs(tail)
	c.sendResponse(reqID, model.LogResponse{
		TaskID:     req.TaskID,
		Logs:       logs,
		TotalLines: total,
	})
}

func (c *Client) handleGetConfig(reqID string) {
	c.sendResponse(reqID, model.ConfigResponse{
		User: model.UserConfig{
			ServerURL: config.GlobalConfig.ServerURL,
			ProxyHost: config.GlobalConfig.ProxyHost,
			ProxyPort: config.GlobalConfig.ProxyPort,
		},
		Client: model.ClientConfig{
			BaseDir:   config.GetBaseDir(),
			StartedAt: time.Now().Format(time.RFC3339),
		},
	})
}

func (c *Client) handleGetAvatar(reqID string, payload json.RawMessage) {
	var req struct {
		PixivUserID string `json:"pixiv_user_id"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return
	}

	baseDir := config.GetBaseDir()
	// Assuming jpg for simplicity, in real app check file existence
	avatarPath := filepath.Join(baseDir, "crawl-datas", req.PixivUserID, ".avatars", req.PixivUserID+".jpg")

	// Check if file exists
	if _, err := os.Stat(avatarPath); os.IsNotExist(err) {
		c.sendResponse(reqID, map[string]string{"error": "Avatar not found"})
		return
	}

	// Read file
	data, err := os.ReadFile(avatarPath)
	if err != nil {
		c.sendResponse(reqID, map[string]string{"error": "Failed to read avatar"})
		return
	}

	// Encode to Base64
	encoded := base64.StdEncoding.EncodeToString(data)

	c.sendResponse(reqID, map[string]string{"data": encoded})
}

func (c *Client) handleGetImage(reqID string, payload json.RawMessage) {
	var req struct {
		PixivUserID string `json:"pixiv_user_id"`
		Filename    string `json:"filename"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return
	}

	baseDir := config.GetBaseDir()
	// Path: crawl-datas/<uid>/.download_imgs/<filename>
	// Note: The user mentioned "crawl-datas/crawl-datas/..." in their example.
	// But based on handleGetAvatar which uses "crawl-datas/<uid>/.avatars",
	// I assume the structure is consistent: "crawl-datas/<uid>/.download_imgs".
	// If the user has a nested "crawl-datas" folder, it might be because GetBaseDir() returns a path ending in crawl-datas?
	// Or maybe they just created a folder named crawl-datas inside crawl-datas.
	// I will stick to the pattern used in handleGetAvatar.
	imagePath := filepath.Join(baseDir, "crawl-datas", req.PixivUserID, ".download_imgs", req.Filename)

	// Check if file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		// Try checking if there is a double crawl-datas folder as per user example, just in case
		imagePath2 := filepath.Join(baseDir, "crawl-datas", "crawl-datas", req.PixivUserID, ".download_imgs", req.Filename)
		if _, err2 := os.Stat(imagePath2); os.IsNotExist(err2) {
			c.sendResponse(reqID, map[string]string{"error": "Image not found"})
			return
		} else {
			imagePath = imagePath2
		}
	}

	// Read file
	data, err := os.ReadFile(imagePath)
	if err != nil {
		c.sendResponse(reqID, map[string]string{"error": "Failed to read image"})
		return
	}

	// Encode to Base64
	encoded := base64.StdEncoding.EncodeToString(data)

	c.sendResponse(reqID, map[string]string{"data": encoded})
}
