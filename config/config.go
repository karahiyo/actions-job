package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v8"
)

type (
	Config struct {
		ServerConfig    ServerConfig
		GCPConfig       GCPConfig
		GitHubAppConfig GitHubAppConfig
	}

	ServerConfig struct {
		LogLevel       string        `env:"LOG_LEVEL"               envDefault:"info"`
		WebhookSecret  string        `env:"WEBHOOK_SECRET,required"`
		Port           int           `env:"PORT"                    envDefault:"8080"`
		DefaultTimeout time.Duration `env:"DEFAULT_TIMEOUT"         envDefault:"10s"`
	}

	GCPConfig struct {
		CloudRunAdminApiEndpoint string `env:"CLOUD_RUN_ADMIN_API_ENDPOINT" envDefault:"https://run.googleapis.com"`
	}

	GitHubAppConfig struct {
		PrivateKey     string        `env:"GH_APP_PRIVATE_KEY,required"`
		RequestTimeout time.Duration `env:"GH_REQUEST_TIMEOUT"              envDefault:"1s"`
		AppID          int64         `env:"GH_APP_ID,required"`
		InstallationID int64         `env:"GH_APP_INSTALLATION_ID,required"`
	}
)

var instance *Config

func Load() (*Config, error) {
	instance = new(Config)
	if err := env.Parse(instance); err != nil {
		return nil, fmt.Errorf("failed to parse Config: %w", err)
	}

	return instance, nil
}

func Get() *Config {
	return instance
}

func GetServerConfig() ServerConfig {
	return instance.ServerConfig
}

func GetGCPConfig() GCPConfig {
	return instance.GCPConfig
}

func GetGitHubAppConfig() GitHubAppConfig {
	return instance.GitHubAppConfig
}
