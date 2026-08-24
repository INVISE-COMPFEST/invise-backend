package main

import (
	"fmt"
	"log"

	"invise-backend/internal/bootstrap/config"
)

func main() {
	cfg := config.Load()

	log.Printf("Starting app in %s mode on port %s", cfg.App.Env, cfg.App.Port)
	fmt.Printf("Server config loaded: env=%s port=%s db=%s@%s:%s/%s\n",
		cfg.App.Env, cfg.App.Port, cfg.DB.User, cfg.DB.Host, cfg.DB.Port, cfg.DB.Name)
}
