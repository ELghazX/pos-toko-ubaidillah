package env

import "os"

func GetString(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback

}
