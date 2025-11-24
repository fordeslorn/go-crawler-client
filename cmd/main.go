package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go-crawler-client/config"
	"go-crawler-client/internal/api"
	"go-crawler-client/internal/auth"
	"go-crawler-client/internal/model"
	"go-crawler-client/internal/service"

	"github.com/go-resty/resty/v2"
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

	// Setup Router
	r := api.SetupRouter()

	// Start Server
	port := config.GlobalConfig.Port
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()
	log.Printf("\033[34mServer started on port %d\033[0m", port)

	// Register Client
	go registerClient(port)

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	log.Println("Server exiting")
}

func registerClient(port int) {
	// Wait a bit for server to start
	time.Sleep(2 * time.Second)

	client := resty.New()
	serverURL := config.GlobalConfig.ServerURL
	registerURL := fmt.Sprintf("%s/api/v1/crawler/client/register", serverURL)

	req := model.RegisterRequest{
		Port:    port,
		BaseDir: config.GetBaseDir(),
	}

	// Retry logic could be added here
	resp, err := client.R().
		SetBody(req).
		Post(registerURL)

	if err != nil {
		log.Printf("Failed to register client: %v", err)
		return
	}

	if resp.IsError() {
		log.Printf("Failed to register client, status: %s", resp.Status())
		return
	}

	log.Printf("\033[32mClient registered successfully with server at %s\033[0m", serverURL)
}
