package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"invise-backend/db/seeds"
	"invise-backend/internal/bootstrap/config"
)

func main() {
	cfg := config.Load()

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=%s",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.Name, cfg.DB.SSLMode, cfg.DB.Path)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("failed to connect to database:", err)
	}

	log.Println("Running seeders...")
	if err := seeds.Run(db, cfg.Seeder.Email, cfg.Seeder.Password); err != nil {
		log.Fatal("seed failed:", err)
	}
	log.Println("Seeding complete!")
}
