package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Kafka    KafkaConfig
	Redis    RedisConfig
	JWT      JWTConfig
	Storage  StorageConfig
}

type ServerConfig struct {
	Port            string
	ShutdownTimeout time.Duration
	MaxRequests     int
	RequestTimeout  time.Duration
	CacheExpiration time.Duration
	Environment     string
}

type DatabaseConfig struct {
	URL string
}

type KafkaConfig struct {
	Broker         string
	Topic          string
	Group          string
	RetryMax       int
	RetryBackoff   time.Duration
	ProcessingTime time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

type StorageConfig struct {
	TempDir string        `env:"STORAGE_TEMP_DIR" envDefault:"/tmp/taskmaster"`
	MaxSize int64         `env:"STORAGE_MAX_SIZE" envDefault:"10485760"` // 10MB
	TTL     time.Duration `env:"STORAGE_TTL" envDefault:"24h"`
}

func LoadConfig() *Config {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
		// Continue execution with fallback values
	} else {
		log.Println("Successfully loaded .env file")
	}

	return &Config{
		Server: ServerConfig{
			Port:            loadEnv("PORT", ":8080"),
			ShutdownTimeout: time.Duration(loadEnvAsInt("SERVER_SHUTDOWN_TIMEOUT", 5)) * time.Second,
			MaxRequests:     loadEnvAsInt("SERVER_MAX_REQUESTS", 100),
			RequestTimeout:  time.Duration(loadEnvAsInt("SERVER_REQUEST_TIMEOUT", 60)) * time.Second,
			CacheExpiration: time.Duration(loadEnvAsInt("SERVER_CACHE_EXPIRATION", 10)) * time.Second,
			Environment:     loadEnv("GO_ENV", "development"),
		},
		Database: DatabaseConfig{
			URL: loadEnv("DATABASE_URL", "xyz"),
		},
		Kafka: KafkaConfig{
			Broker:         loadEnv("KAFKA_BROKER", "localhost:9092"),
			Topic:          loadEnv("KAFKA_TOPIC", "jobs"),
			Group:          loadEnv("KAFKA_GROUP", "job-workers"),
			RetryMax:       loadEnvAsInt("KAFKA_RETRY_MAX", 5),
			RetryBackoff:   time.Duration(loadEnvAsInt("KAFKA_RETRY_BACKOFF", 500)) * time.Millisecond,
			ProcessingTime: time.Duration(loadEnvAsInt("KAFKA_PROCESSING_TIME", 10)) * time.Second,
		},
		Redis: RedisConfig{
			Addr:     loadEnv("REDIS_ADDR", "localhost:6379"),
			Password: loadEnv("REDIS_PASSWORD", ""),
			DB:       loadEnvAsInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Secret:     loadEnv("JWT_SECRET", "supersecretkey"),
			Expiration: time.Duration(loadEnvAsInt("JWT_EXPIRATION", 72)) * time.Hour,
		},
		Storage: StorageConfig{
			TempDir: loadEnv("STORAGE_TEMP_DIR", "/tmp/taskmaster"),
			MaxSize: loadEnvAsInt64("STORAGE_MAX_SIZE", 10485760),                    // 10MB
			TTL:     time.Duration(loadEnvAsInt("STORAGE_TTL", 86400)) * time.Second, // 24h
		},
	}
}

func loadEnv(key, defaultVal string) string {
	log.Printf("Loading environment variable: %s", key)

	if value, exists := os.LookupEnv(key); exists {
		log.Printf("Found %s in environment with value: %s", key, value)
		return value
	}

	log.Printf("Environment variable %s not found, using default value: %s", key, defaultVal)
	return defaultVal
}

func loadEnvAsInt(key string, defaultVal int) int {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultVal
}

func loadEnvAsInt64(key string, defaultVal int64) int64 {
	if value, exists := os.LookupEnv(key); exists {
		if intVal, err := strconv.ParseInt(value, 10, 64); err == nil {
			return intVal
		}
	}
	return defaultVal
}
