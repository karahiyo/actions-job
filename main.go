package main

import (
	"context"
	"fmt"
	"net/http"

	"github.com/karahiyo/actions-job/config"
	"github.com/karahiyo/actions-job/handler"
	"github.com/karahiyo/actions-job/service"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func init() {
	zerolog.LevelFieldName = "severity"

	var err error
	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Msgf("failed to load config: %v", err)
	}

	logLevel, err := zerolog.ParseLevel(cfg.ServerConfig.LogLevel)
	if err != nil {
		log.Fatal().Msgf("failed to parse log level: %v", err)
	}
	zerolog.SetGlobalLevel(logLevel)
}

func main() {
	ctx := context.Background()

	log.Info().Msgf("starting HTTP server...")

	controller, err := service.NewController(ctx)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to initialize controller")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/github/events", handler.HandleWebhookEvents(controller))

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", config.GetServerConfig().Port),
		Handler:      mux,
		ReadTimeout:  config.GetServerConfig().DefaultTimeout,
		WriteTimeout: config.GetServerConfig().DefaultTimeout,
	}

	log.Info().Msgf("HTTP server started: port = %d", config.GetServerConfig().Port)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("Startup failed")
	}
}
