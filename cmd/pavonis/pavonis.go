package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/constants"
	"github.com/Fallen-Breath/pavonis/internal/server"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func initLogging() {
	log.SetFormatter(&log.TextFormatter{
		ForceColors:   true,
		FullTimestamp: true,
	})
	log.SetOutput(os.Stdout)
}

func main() {
	flagConfig := flag.String("c", "config.yml", "Path to the config yaml file")
	flagShowHelp := flag.Bool("h", false, "Show help and exit")
	flagShowVersion := flag.Bool("v", false, "Show version and exit")
	flag.Parse()

	if *flagShowHelp {
		flag.Usage()
		return
	}
	if *flagShowVersion {
		fmt.Printf("Pavonis v%s\n", constants.Version)
		return
	}

	initLogging()
	cfg := config.LoadConfigOrDie(*flagConfig)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	proxyServer := server.NewServer(cfg)
	srv := &http.Server{
		Addr:    cfg.Listen,
		Handler: proxyServer,
	}

	log.Infof("Starting Pavonis v%s on %s", constants.Version, cfg.Listen)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	sig := <-ch
	log.Infof("Received signal %s, shutting down Pavonis...", sig)
	if err := srv.Close(); err != nil {
		log.Warnf("Pavonis shutdown failed: %v", err)
	}
	proxyServer.Shutdown()

	log.Infof("Pavonis stopped")
}
