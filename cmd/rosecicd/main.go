package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/glennswest/rosecicd/internal/buildmgr"
	"github.com/glennswest/rosecicd/internal/config"
	"github.com/glennswest/rosecicd/internal/poller"
	"github.com/glennswest/rosecicd/internal/server"
)

func main() {
	cfgPath := flag.String("config", "/etc/rosecicd/config.yaml", "config file path")
	flag.Parse()

	// Try specified config, fall back to embedded default
	path := *cfgPath
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fallback := path + ".default"
		if _, err := os.Stat(fallback); err == nil {
			log.Printf("config %s not found, using default %s", path, fallback)
			path = fallback
		}
	}

	cfg, err := config.Load(path)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// Ensure cache dir exists
	if cfg.Build.CacheDir != "" {
		os.MkdirAll(cfg.Build.CacheDir, 0755)
	}

	mgr := buildmgr.New(cfg)

	// Start GitHub poller
	p := poller.New(cfg, mgr)
	p.Start()

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("shutting down...")
		p.Stop()
		mgr.Stop()
		os.Exit(0)
	}()

	// Start HTTP server
	if err := server.Run(cfg, mgr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
