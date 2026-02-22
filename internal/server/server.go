package server

import (
	"log"
	"net/http"

	"github.com/glennswest/rosecicd/internal/buildmgr"
	"github.com/glennswest/rosecicd/internal/config"
	"github.com/glennswest/rosecicd/internal/ui"
)

func Run(cfg *config.Config, mgr *buildmgr.Manager) error {
	mux := http.NewServeMux()
	ui.Register(mux, cfg, mgr)
	log.Printf("rosecicd listening on %s", cfg.Server.Addr)
	return http.ListenAndServe(cfg.Server.Addr, mux)
}
