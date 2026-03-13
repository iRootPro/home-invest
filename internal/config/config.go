package config

import (
	"os"
)

type Config struct {
	DBPath        string
	ListenAddr    string
	SessionSecret string
	DefaultUser   string
	DefaultPass   string
	UploadsDir    string
}

func Load() *Config {
	return &Config{
		DBPath:        getEnv("BANKI_DB_PATH", "banki.db"),
		ListenAddr:    getEnv("BANKI_LISTEN_ADDR", ":8080"),
		SessionSecret: getEnv("BANKI_SESSION_SECRET", "change-me-in-production-32bytes!"),
		DefaultUser:   getEnv("BANKI_DEFAULT_USER", "admin"),
		DefaultPass:   getEnv("BANKI_DEFAULT_PASS", "admin"),
		UploadsDir:    getEnv("BANKI_UPLOADS_DIR", "./uploads"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
