package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"go-crawler-client/config"
	"go-crawler-client/internal/api"
	"go-crawler-client/internal/auth"
	"go-crawler-client/internal/service"
	"go-crawler-client/internal/socket"
)

func main() {
	// Load Config
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Init Token Validator
	var err error
	api.TokenValidator, err = auth.NewTokenValidator("public.pem")
	if err != nil {
		log.Printf("Warning: Failed to load public key: %v. Token validation will fail.", err)
		// We might want to exit here if security is mandatory
		// log.Fatalf("Failed to initialize token validator: %v", err)
	} else {
		log.Println("Token validator initialized successfully.")
	}

	// Init Task Manager (and directories)
	service.InitTaskManager()

	// Start WebSocket Client
	if config.GlobalConfig.Token != "" {
		wsClient := socket.NewClient(config.GlobalConfig.ServerURL, config.GlobalConfig.Token)
		go wsClient.Connect()
	} else {
		log.Println("Warning: No token provided in user_config.json. WebSocket connection disabled. You will not be able to receive tasks from the backend.")
	}

	// Setup Router (Optional, if we want to keep local API)
	_ = api.SetupRouter()

	// Start Server (Optional, for health check or local debugging)
	// Since we use WebSocket, we don't strictly need to listen on a port for incoming commands.
	// But keeping it for now might be useful.
	// port := config.GlobalConfig.Port
	// srv := &http.Server{
	// 	Addr:    fmt.Sprintf(":%d", port),
	// 	Handler: r,
	// }

	// go func() {
	// 	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
	// 		log.Fatalf("listen: %s\n", err)
	// 	}
	// }()
	// log.Printf("\033[34mServer started on port %d\033[0m", port)

	// Register Client (Deprecated)
	// go registerClient(port)

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down client...")

	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()
	// if err := srv.Shutdown(ctx); err != nil {
	// 	log.Fatal("Server forced to shutdown: ", err)
	// }

	log.Println("Client exiting")
}

// func registerClient(port int) {
// 	// ...existing code...
// 	// Retry logic could be added here
// 	resp, err := client.R().
// 		SetBody(req).
// 		Post(registerURL)

// 	if err != nil {
// 		log.Printf("Failed to register client: %v", err)
// 		return
// 	}

// 	if resp.IsError() {
// 		log.Printf("Failed to register client, status: %s", resp.Status())
// 		return
// 	}

// 	log.Printf("\033[32mClient registered successfully with server at %s\033[0m", serverURL)
// }
