// Command promogo runs the PromoGo loyalty backend.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/MirzaDgtu/PromoGo/internal/app"
	"github.com/MirzaDgtu/PromoGo/internal/config"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bootstrap-admin":
			runBootstrapAdmin(os.Args[2:])
			return
		default:
			fmt.Fprintf(os.Stderr, "unknown command %q\n\nusage:\n  promogo                  run the HTTP server\n  promogo bootstrap-admin  create the first platform_admin (see -h)\n", os.Args[1])
			os.Exit(2)
		}
	}
	runServer()
}

func runServer() {
	cfg, err := config.Load(resolveConfigPath())
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

func resolveConfigPath() string {
	path := "configs/config.yaml"
	if v := os.Getenv("PROMOGO_CONFIG_FILE"); v != "" {
		path = v
	}
	return path
}
