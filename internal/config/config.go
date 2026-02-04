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
	Server           ServerConfig
	Database         DatabaseConfig
	Redis            RedisConfig
	ExchangeRateHost ExchangeRateHostConfig `mapstructure:"exchangerate_host"`
	Frankfurter      FrankfurterConfig      `mapstructure:"frankfurter"`
	Worker           WorkerConfig
	Cache            CacheConfig
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Port          int  `mapstructure:"port"`
	ServeSwagger  bool `mapstructure:"serve_swagger"`
	ServeAsynqmon bool `mapstructure:"serve_asynqmon"`
}

// DatabaseConfig holds PostgreSQL connection settings.
type DatabaseConfig struct {
	Host               string `mapstructure:"host"`
	Port               int    `mapstructure:"port"`
	User               string `mapstructure:"user"`
	Password           string `mapstructure:"password"`
	Name               string `mapstructure:"name"`
	SSLMode            string `mapstructure:"sslmode"`
	MaxOpenConns       int    `mapstructure:"max_open_conns"`
	MaxIdleConns       int    `mapstructure:"max_idle_conns"`
	ConnMaxLifetimeSec int    `mapstructure:"conn_max_lifetime_sec"`
	DSN                string
}

// RedisConfig holds connection settings for both Redis instances.
type RedisConfig struct {
	AsynqAddr string `mapstructure:"asynq_addr"` // Redis instance for Asynq task queue (required).
	CacheAddr string `mapstructure:"cache_addr"` // Redis instance for application cache (required).
}

// ExchangeRateHostConfig holds settings for the exchangerate.host provider.
type ExchangeRateHostConfig struct {
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
	Timeout int    `mapstructure:"timeout_sec"`
}

// FrankfurterConfig holds settings for the frankfurter provider.
type FrankfurterConfig struct {
	BaseURL string `mapstructure:"base_url"`
	Timeout int    `mapstructure:"timeout_sec"`
}

// WorkerConfig holds background worker and task queue settings.
type WorkerConfig struct {
	Concurrency      int `mapstructure:"concurrency"`
	MaxRetry         int `mapstructure:"max_retry"`
	TimeoutSec       int `mapstructure:"timeout_sec"`
	CheckIntervalSec int `mapstructure:"check_interval_sec"`
}

// CacheConfig holds caching settings.
type CacheConfig struct {
	LatestPriceTTLSec           int `mapstructure:"latest_price_ttl_sec"`
	ExchangeProviderPriceTTLSec int `mapstructure:"exchange_provider_price_ttl_sec"`
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
	viper.SetDefault("server.serve_asynqmon", true)
	viper.SetDefault("database.host", "db")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.user", "postgres")
	viper.SetDefault("database.password", "postgres")
	viper.SetDefault("database.name", "quotesdb")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("database.max_open_conns", 10)
	viper.SetDefault("database.max_idle_conns", 5)
	viper.SetDefault("database.conn_max_lifetime_sec", 300)
	viper.SetDefault("redis.asynq_addr", "redis_asynq:6380")
	viper.SetDefault("redis.cache_addr", "redis_cache:6381")
	viper.SetDefault("exchangerate_host.base_url", "https://api.exchangerate.host")
	viper.SetDefault("exchangerate_host.api_key", "")
	viper.SetDefault("exchangerate_host.timeout_sec", 5)
	viper.SetDefault("frankfurter.base_url", "https://api.frankfurter.dev/v1")
	viper.SetDefault("frankfurter.timeout_sec", 5)
	viper.SetDefault("worker.concurrency", 1)
	viper.SetDefault("worker.max_retry", 3)
	viper.SetDefault("worker.timeout_sec", 30)
	viper.SetDefault("worker.check_interval_sec", 5)
	viper.SetDefault("cache.latest_price_ttl_sec", 600)
	viper.SetDefault("cache.exchange_provider_price_ttl_sec", 300)

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
	if cfg.Database.ConnMaxLifetimeSec <= 0 {
		cfg.Database.ConnMaxLifetimeSec = 300
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

	if c.Redis.AsynqAddr == "" {
		errs = append(errs, fmt.Errorf("redis.asynq_addr is required (set QUOTESVC_REDIS_ASYNQ_ADDR)"))
	}
	if c.Redis.CacheAddr == "" {
		errs = append(errs, fmt.Errorf("redis.cache_addr is required (set QUOTESVC_REDIS_CACHE_ADDR)"))
	}

	if c.Worker.Concurrency <= 0 {
		errs = append(errs, fmt.Errorf("worker.concurrency must be positive, got %d", c.Worker.Concurrency))
	}
	if c.Worker.MaxRetry < 0 {
		errs = append(errs, fmt.Errorf("worker.max_retry must be non-negative, got %d", c.Worker.MaxRetry))
	}
	if c.Worker.TimeoutSec <= 0 {
		errs = append(errs, fmt.Errorf("worker.timeout_sec must be positive, got %d", c.Worker.TimeoutSec))
	}
	if c.Worker.CheckIntervalSec <= 0 {
		errs = append(errs, fmt.Errorf("worker.check_interval_sec must be positive, got %d", c.Worker.CheckIntervalSec))
	}

	if c.Cache.LatestPriceTTLSec <= 0 {
		errs = append(errs, fmt.Errorf("cache.latest_price_ttl_sec must be positive, got %d", c.Cache.LatestPriceTTLSec))
	}
	if c.Cache.ExchangeProviderPriceTTLSec <= 0 {
		errs = append(errs, fmt.Errorf("cache.exchange_provider_price_ttl_sec must be positive, got %d", c.Cache.ExchangeProviderPriceTTLSec))
	}

	return errors.Join(errs...)
}
