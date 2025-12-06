package main

import (
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"log"
	"os"
)

func LoadEnv() {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ Warning: .env file not found, using system environment variables")
	}
}

func main() {

	// Load .env variables
	LoadEnv()

	// OPTIONAL: Log JWT_SECRET to confirm it loaded (remove in production)
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("❌ JWT_SECRET is missing in .env")
	}
	log.Println("🔐 JWT_SECRET loaded successfully")

	// Connect DB
	InitDB()

	// Start Gin
	r := gin.Default()

	// CORS
	r.Use(CORSMiddleware())

	// Routes
	SetupRoutes(r)

	// Start server
	log.Println("🚀 Server running on http://localhost:8080")
	r.Run(":8080") // do NOT add space or quotes incorrectly
}
