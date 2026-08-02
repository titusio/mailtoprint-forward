package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads a .env file from the working directory if present. Real
// environment variables always take precedence (godotenv.Load does not
// overwrite variables that are already set). A missing file is not an error.
func LoadDotEnv() {
	if _, err := os.Stat(".env"); err != nil {
		return // no .env file, use the real environment only
	}
	if err := godotenv.Load(); err != nil {
		log.Printf("failed to load .env: %v", err)
	}
}
