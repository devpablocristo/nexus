package godotenv

import (
	"os"

	"github.com/joho/godotenv"
)

// LoadIfExists loads ".env" if present. It never errors if the file is missing.
func LoadIfExists() error {
	if _, err := os.Stat(".env"); err == nil {
		return godotenv.Load(".env")
	}
	return nil
}
