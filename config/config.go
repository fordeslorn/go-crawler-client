package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	ServerURL string `mapstructure:"server_url" json:"server_url"`
	Token     string `mapstructure:"token" json:"token"` // Auth Token for WebSocket
	ProxyHost string `mapstructure:"proxy_host" json:"proxy_host"`
	ProxyPort int    `mapstructure:"proxy_port" json:"proxy_port"`
	BaseDir   string `mapstructure:"base_dir" json:"base_dir"`
}

var GlobalConfig Config

func LoadConfig() error {
	viper.SetConfigName("user_config")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	// Set defaults
	viper.SetDefault("server_url", "http://localhost:8080")
	viper.SetDefault("token", "")
	viper.SetDefault("proxy_host", "127.0.0.1")
	viper.SetDefault("proxy_port", 7890)
	viper.SetDefault("base_dir", "")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired or create default
			fmt.Println("Config file not found, using defaults")
			// Create default config file
			viper.WriteConfigAs("user_config.json")
		} else {
			// Config file was found but another error was produced
			return fmt.Errorf("fatal error config file: %w", err)
		}
	}

	// Unmarshal
	// automatically maps config values to the struct fields
	if err := viper.Unmarshal(&GlobalConfig); err != nil {
		return fmt.Errorf("unable to decode into struct: %w", err)
	}

	return nil
}

// GetBaseDir get the base directory of the application
func GetBaseDir() string {
	if GlobalConfig.BaseDir != "" {
		return GlobalConfig.BaseDir
	}

	ex, err := os.Executable()
	if err != nil {
		return "."
	}
	exPath := filepath.Dir(ex)

	// Heuristic to detect "go run" which builds into a temp directory
	// On Windows, temp dir is usually in AppData\Local\Temp
	// We check for "go-build" which is used by go run, or if the path is inside the system temp directory
	if strings.Contains(exPath, "go-build") || strings.Contains(strings.ToLower(exPath), strings.ToLower(os.TempDir())) {
		wd, err := os.Getwd()
		if err == nil {
			return wd
		}
	}

	return exPath
}
