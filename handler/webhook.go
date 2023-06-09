package handler

import (
	"errors"
	"net/http"
	"os"

	"github.com/google/go-github/v52/github"
	"github.com/karahiyo/actions-job/config"
	"github.com/karahiyo/actions-job/service"
	"github.com/rs/zerolog"
)

var ErrBadRequest = errors.New("bad request")

func HandleWebhookEvents(controller *service.Controller) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
		ctx = logger.WithContext(ctx)

		payload, err := github.ValidatePayload(r, []byte(config.GetServerConfig().WebhookSecret))
		if err != nil {
			logger.Warn().Err(err).Msg("Could not validate request body")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		logger.Info().Msgf("Received webhook event: payload=%s", string(payload))

		webhookEvent, err := github.ParseWebHook(github.WebHookType(r), payload)
		if err != nil {
			logger.Error().Err(err).Msg("Could not parse webhook")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		switch event := webhookEvent.(type) {
		case *github.PingEvent:
			w.WriteHeader(http.StatusOK)
			msg := "pong"

			if written, err := w.Write([]byte(msg)); err != nil {
				logger.Error().Err(err).Msgf("failed writing http response: %d", written)
				return
			}
			logger.Info().Msgf("handled ping event")

			return
		case *github.WorkflowJobEvent:
			if err := controller.ReceiveWorkflowJobEvent(ctx, event); err != nil {
				if errors.Is(err, service.ErrNonTargetEvent) {
					logger.Debug().Err(err).Msg("received non target event, return OK")
					w.WriteHeader(http.StatusAccepted)
					return
				}

				if errors.Is(err, service.ErrBadRequest) {
					logger.Warn().Err(err).Msg("received bad request")
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				logger.Error().Stack().Err(err).Msg("failed to process workflow_job event")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			return

		default:
			logger.Warn().Msgf("received not register event(%s), return NotFound", payload)
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}
}
