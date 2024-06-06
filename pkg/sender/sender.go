package sender

import (
	"bytes"
	"context"
	"net/http"

	"github.com/open-component-model/ocm-controller/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Notifier interface {
	Notify(ctx context.Context, sub v1alpha1.Subscribable)
}

type Sender struct{}

func (s *Sender) Notify(ctx context.Context, sub v1alpha1.Subscribable) {
	logger := log.FromContext(ctx)

	// implement timeout
	go func() {
		payloads := sub.Payloads()
		if len(payloads) == 0 {
			return
		}

		for _, payload := range payloads {
			logger = logger.WithValues("payload", payload.Payload)
			logger.Info("sending payload")

			req, err := http.NewRequest(http.MethodPost, payload.Destination, bytes.NewBufferString(payload.Payload))
			if err != nil {
				logger.Error(err, "unable to create request")

				continue
			}

			func() {
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					logger.Error(err, "unable to send request")
				}
				defer resp.Body.Close()

				logger.Info("received response")
			}()
		}
	}()
}
