// Package config provides application configuration loading and validation.
package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

// Config holds the complete application configuration.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	External ExternalConfig
	Worker   WorkerConfig
	Cache    CacheConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port         int  `mapstructure:"port"`
	ServeSwagger bool `mapstructure:"serve_swagger"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	Name         string `mapstructure:"name"`
	SSLMode      string `mapstructure:"sslmode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	DSN          string
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
}

// ExternalConfig holds settings for the external exchange rate provider.
type ExternalConfig struct {
	Provider string `mapstructure:"provider"`
	BaseURL  string `mapstructure:"base_url"`
	APIKey   string `mapstructure:"api_key"`
	Timeout  int    `mapstructure:"timeout_sec"`
}

// WorkerConfig holds background worker settings.
type WorkerConfig struct {
	Concurrency int `mapstructure:"concurrency"`
}

// CacheConfig holds caching and task retry settings.
type CacheConfig struct {
	TTLSec         int `mapstructure:"ttl_sec"`
	TaskMaxRetry   int `mapstructure:"task_max_retry"`
	TaskTimeoutSec int `mapstructure:"task_timeout_sec"`
}

// LoadConfig reads configuration from config files, environment variables, and defaults.
func LoadConfig() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		fmt.Printf("No .env file found or error loading it: %v\n", err)
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Config search paths
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("./internal/config")

	viper.SetEnvPrefix("QUOTESVC")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// default values
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.serve_swagger", true)
	viper.SetDefault("database.host", "db")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.name", "quotesdb")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max_open_conns", 10)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("redis.addr", "redis:6380")
	viper.SetDefault("redis.password", "")
	viper.SetDefault("external.provider", "exchangerate_host")
	viper.SetDefault("external.base_url", "https://api.exchangerate.host")
	viper.SetDefault("external.api_key", "")
	viper.SetDefault("external.timeout_sec", 5)
	viper.SetDefault("worker.concurrency", 1)
	viper.SetDefault("cache.ttl_sec", 3600)
	viper.SetDefault("cache.task_max_retry", 3)
	viper.SetDefault("cache.task_timeout_sec", 30)

	if err := viper.ReadInConfig(); err != nil {
		// It's okay if no config file, we have defaults and env
		fmt.Printf("Config file not found: %v\n", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	if cfg.Database.MaxOpenConns <= 0 {
		cfg.Database.MaxOpenConns = 10
	}
	if cfg.Database.MaxIdleConns <= 0 {
		cfg.Database.MaxIdleConns = 5
	}

	cfg.Database.DSN = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Database.User, cfg.Database.Password,
		cfg.Database.Host, cfg.Database.Port,
		cfg.Database.Name, cfg.Database.SSLMode)

	return &cfg, nil
}

// Validate checks that all required configuration fields are set and valid.
func (c *Config) Validate() error {
	var errs []error
	if c.Server.Port <= 0 {
		errs = append(errs, fmt.Errorf("server.port must be positive, got %d", c.Server.Port))
	}

	if c.Database.Host == "" {
		errs = append(errs, fmt.Errorf("database.host is required"))
	}
	if c.Database.Port <= 0 {
		errs = append(errs, fmt.Errorf("database.port must be positive, got %d", c.Database.Port))
	}
	if c.Database.User == "" {
		errs = append(errs, fmt.Errorf("database.user is required"))
	}
	if c.Database.Name == "" {
		errs = append(errs, fmt.Errorf("database.name is required"))
	}

	if c.Redis.Addr == "" {
		errs = append(errs, fmt.Errorf("redis.addr is required"))
	}

	if c.External.APIKey == "" {
		errs = append(errs, fmt.Errorf("external.api_key is required (set QUOTESVC_EXTERNAL_API_KEY)"))
	} else if c.External.APIKey == "ignored" {
		c.External.APIKey = ""
	}
	if c.External.Timeout <= 0 {
		errs = append(errs, fmt.Errorf("external.timeout_sec must be positive, got %d", c.External.Timeout))
	}

	if c.Worker.Concurrency <= 0 {
		errs = append(errs, fmt.Errorf("worker.concurrency must be positive, got %d", c.Worker.Concurrency))
	}

	return errors.Join(errs...)
}
