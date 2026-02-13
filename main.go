package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"berkut-scc/config"
	"berkut-scc/core/appbootstrap"
	"berkut-scc/core/utils"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}

	logger := utils.NewLogger()
	rt, err := appbootstrap.InitRuntime(context.Background(), cfg, logger)
	if err != nil {
		logger.Fatalf("%v", err)
	}
	defer rt.DB.Close()

	rt.StartBackground(context.Background())
	srv := rt.Server
	go func() {
		if err := srv.Start(); err != nil {
			logger.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rt.StopBackground(ctx); err != nil {
		logger.Errorf("background shutdown: %v", err)
	}
	if err := srv.Stop(ctx); err != nil {
		logger.Errorf("graceful shutdown: %v", err)
	}
}
