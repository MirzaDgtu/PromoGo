// Command promogo runs the PromoGo loyalty backend.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/MirzaDgtu/PromoGo/internal/app"
	"github.com/MirzaDgtu/PromoGo/internal/config"
)

func main() {
	configPath := "configs/config.yaml"
	if v := os.Getenv("PROMOGO_CONFIG_FILE"); v != "" {
		configPath = v
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("initialize app: %v", err)
	}
	defer application.Close()

	if err := application.Run(ctx); err != nil {
		log.Fatalf("run app: %v", err)
	}
}
