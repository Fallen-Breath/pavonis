package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/Fallen-Breath/pavonis/internal/config"
	"github.com/Fallen-Breath/pavonis/internal/constants"
	"github.com/Fallen-Breath/pavonis/internal/diagnostics"
	"github.com/Fallen-Breath/pavonis/internal/server"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func initLogging() {
	log.SetFormatter(&log.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
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
	log.Infof("Pavonis initializing ...")

	cfg := config.LoadConfigOrDie(*flagConfig)
	runPavonis(cfg)
}

func runPavonis(cfg *config.Config) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	pavonisServer, err := server.NewPavonisServer(cfg)
	if err != nil {
		log.Fatalf("Pavonis server init failed: %v", err)
	}

	httpServers := httpServerHolder{}

	mainHttpServer := pavonisServer.CreateHttpServer()
	httpServers.Add(mainHttpServer)
	log.Infof("Starting Pavonis v%s on %s", constants.Version, mainHttpServer.Addr)

	if cfg.Diagnostics.Enabled {
		diagnosticsHttpServer := diagnostics.NewServer(cfg.Diagnostics).CreateHttpServer()
		httpServers.Add(diagnosticsHttpServer)
		log.Debugf("Starting diagnostics http server on %s", diagnosticsHttpServer.Addr)
	}

	httpServers.Run(ch)

	pavonisServer.Shutdown()
	log.Infof("Pavonis stopped")
}

type httpServerHolder struct {
	servers []*http.Server
}

func (h *httpServerHolder) Add(server *http.Server) {
	h.servers = append(h.servers, server)
}

func (h *httpServerHolder) Run(stopCh chan os.Signal) {
	for _, httpServer := range h.servers {
		go func() {
			if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Fatalf("httpServer ListenAndServe failed: %v", err)
			}
		}()
	}

	sig := <-stopCh
	log.Infof("Received signal %s, shutting down...", sig)

	for _, httpServer := range h.servers {
		if err := httpServer.Close(); err != nil {
			log.Warnf("httpServer shutdown failed: %v", err)
		}
	}
}
