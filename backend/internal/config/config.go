package config

import (
    "os"
    "strconv"
)

type Config struct {
    HTTPPort       string
    DatabaseURL    string
    StorageDir     string
    RateLimitRPS   int
    UserQuotaBytes int64
}

func FromEnv() Config {
    return Config{
        HTTPPort:       getenv("PORT", "8080"),
        DatabaseURL:    getenv("DATABASE_URL", "postgres://postgres:postgres@db:5432/filevault?sslmode=disable"),
        StorageDir:     getenv("STORAGE_DIR", "/data"),
        RateLimitRPS:   getenvInt("RATE_LIMIT_RPS", 2),
        UserQuotaBytes: getenvInt64("USER_QUOTA_BYTES", 10*1024*1024),
    }
}

func getenv(k, def string) string {
    if v := os.Getenv(k); v != "" {
        return v
    }
    return def
}

func getenvInt(k string, def int) int {
    if v := os.Getenv(k); v != "" {
        if i, err := strconv.Atoi(v); err == nil {
            return i
        }
    }
    return def
}

func getenvInt64(k string, def int64) int64 {
    if v := os.Getenv(k); v != "" {
        if i, err := strconv.ParseInt(v, 10, 64); err == nil {
            return i
        }
    }
    return def
}


