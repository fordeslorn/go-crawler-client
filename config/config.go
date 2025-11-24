package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	ServerURL string `mapstructure:"server_url" json:"server_url"`
	ProxyHost string `mapstructure:"proxy_host" json:"proxy_host"`
	ProxyPort int    `mapstructure:"proxy_port" json:"proxy_port"`
	Port      int    `mapstructure:"port" json:"port"`
}

var GlobalConfig Config

func LoadConfig() error {
	viper.SetConfigName("user_config")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	// Set defaults
	viper.SetDefault("server_url", "http://localhost:8080")
	viper.SetDefault("proxy_host", "127.0.0.1")
	viper.SetDefault("proxy_port", 7890)
	viper.SetDefault("port", 8081)

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
	ex, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(ex)
}
