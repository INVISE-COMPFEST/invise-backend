package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"

	"invise-backend/internal/bootstrap/config"
)

const migrationsDir = "db/migrations"

func main() {
	cfg := config.Load()
	flag.Parse()

	args := flag.Args()
	command := "up"
	if len(args) > 0 {
		command = args[0]
	}

	if command == "create" {
		if len(args) < 2 {
			log.Fatal("migration name required. Usage: go run ./cmd/migrate create <name>")
		}
		if err := goose.Create(nil, migrationsDir, args[1], "sql"); err != nil {
			log.Fatal("failed to create migration:", err)
		}
		return
	}

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s search_path=%s",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.Name, cfg.DB.SSLMode, cfg.DB.Path)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal("failed to open database:", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal(err)
	}

	switch command {
	case "up":
		if err := goose.Up(db, migrationsDir); err != nil {
			log.Fatal("migration up failed:", err)
		}
	case "down":
		if err := goose.Down(db, migrationsDir); err != nil {
			log.Fatal("migration down failed:", err)
		}
	case "status":
		if err := goose.Status(db, migrationsDir); err != nil {
			log.Fatal("migration status failed:", err)
		}
	case "reset":
		if err := goose.Reset(db, migrationsDir); err != nil {
			log.Fatal("migration reset failed:", err)
		}
	case "version":
		if err := goose.Version(db, migrationsDir); err != nil {
			log.Fatal("migration version failed:", err)
		}
	default:
		log.Fatalf("unknown migration command: %s (supported: up, down, status, reset, version, create)", command)
	}
}
