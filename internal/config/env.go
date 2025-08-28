package config

import (
	"github.com/joho/godotenv"
)

func init() {
	// Load .env file silently if it exists
	// This runs before any other init functions
	_ = godotenv.Load()
}
