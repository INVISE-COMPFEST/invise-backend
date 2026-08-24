package main

import (
	"context"
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"invise-backend/internal/bootstrap/config"
	"invise-backend/internal/bootstrap/server"
	"invise-backend/internal/bootstrap/valkey"
)

func main() {
	cfg := config.Load()

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=%s",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.Name, cfg.DB.SSLMode, cfg.DB.Path)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal("failed to connect to database:", err)
	}

	rdb := valkey.NewClient(cfg.Valkey)
	if err := valkey.Ping(context.Background(), rdb); err != nil {
		log.Fatal("failed to connect to valkey:", err)
	}

	srv := server.New(cfg, db, rdb)
	addr := ":" + cfg.App.Port
	log.Printf("starting server on %s", addr)
	if err := srv.Listen(addr); err != nil {
		log.Fatal("server error:", err)
	}
}
