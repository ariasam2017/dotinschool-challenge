// Command server is the dotinschool-challenge platform: one static binary that serves the
// embedded Vue frontend and a small REST API backed by a single SQLite file.
package main

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/dotinschool/dotinschool-challenge/internal/config"
	"github.com/dotinschool/dotinschool-challenge/internal/database"
	"github.com/dotinschool/dotinschool-challenge/internal/service"
	"github.com/dotinschool/dotinschool-challenge/internal/web"
)

// version is stamped at build time via -ldflags "-X main.version=...".
// scripts/docker-build.sh sets it from `git describe`. Logged once at start-up
// so `dotinschool-challenge update` can prove which build the container is running.
var version = "dev"

func main() {
	log := newLogger()
	log.Infof("dotinschool-challenge version %s", version)

	wd, err := os.Getwd()
	if err != nil {
		log.WithError(err).Fatal("could not determine working directory")
	}

	cfg := config.Load(wd, log)

	db, err := database.Open(cfg, log)
	if err != nil {
		log.WithError(err).Fatal("could not open database")
	}
	defer database.Close(db)

	if err := database.EnsureDefaults(db); err != nil {
		log.WithError(err).Fatal("could not seed configuration defaults")
	}
	if err := database.Seed(db); err != nil {
		log.WithError(err).Fatal("could not seed demo data")
	}

	services := service.New(cfg, db, log)
	services.RestoreAISidecarIfMissing()

	// Background CPU/RAM/Swap sampler for the admin "منابع سرور" page — one
	// sample per second, cached, so the endpoint answers instantly instead of
	// blocking on cpu.Percent for every poll.
	services.SysStats.Start(time.Second)
	defer services.SysStats.Stop()

	// The cron scheduler flips a scheduled event live the moment eventStartsAt
	// arrives, with no manual action from the admin.
	scheduler := service.NewScheduler(services.Settings, log)
	scheduler.Start()
	defer scheduler.Stop()

	router := web.NewRouter(web.Options{
		Config:   cfg,
		Services: services,
		Log:      log,
		Version:  version,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		scheme := "http"
		if cfg.UseTLS {
			scheme = "https"
		}
		base := cfg.PublicBaseURL()
		log.Infof("dotinschool-challenge server running (%s):", scheme)
		log.Infof("  User panel:  %s/", base)
		log.Infof("  Admin panel: %s%s/", base, cfg.AdminPath)
		log.Infof("  (bound to 0.0.0.0:%s)", cfg.Port)

		var listenErr error
		if cfg.UseTLS {
			srv.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
			listenErr = srv.ListenAndServeTLS(cfg.CertPath, cfg.KeyPath)
		} else {
			listenErr = srv.ListenAndServe()
		}
		if listenErr != nil && !errors.Is(listenErr, http.ErrServerClosed) {
			log.WithError(listenErr).Fatal("http server stopped unexpectedly")
		}
	}()

	// Graceful shutdown: SIGINT/SIGTERM drains in-flight requests before exit.
	// Backup-restore and self-update handlers also call os.Exit after replying,
	// relying on systemd/Docker to bring the process straight back up.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Info("shutdown signal received — draining connections")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.WithError(err).Warn("graceful shutdown timed out")
	}
	log.Info("bye")
}

func newLogger() *logrus.Logger {
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339,
	})
	if lvl, err := logrus.ParseLevel(os.Getenv("LOG_LEVEL")); err == nil {
		log.SetLevel(lvl)
	} else {
		log.SetLevel(logrus.InfoLevel)
	}
	return log
}
