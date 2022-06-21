package awscreds

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/go-kit/log"
)

// NewRefresher creates a new credentials refresher
// and immediately tries to fetch credentials
// to avoid an initialization penalty in client's API request.
//
// Note, take the Credentials pointer from the session's Config, i.e., sess.Config.Credentials,
// to make sure the right credentials are refreshed.
//
// The default refresh period is 55 minutes in case a token expires every hour
// see https://docs.aws.amazon.com/eks/latest/userguide/kubernetes-versions.html.
func NewRefresher(creds *credentials.Credentials, opts ...Option) (*Refresher, error) {
	if creds == nil {
		return nil, fmt.Errorf("creds cannot be nil")
	}

	r := Refresher{
		conf: Config{
			logger: log.NewNopLogger(),
			period: 55 * time.Minute,
		},
		creds: creds,
	}
	for _, opt := range opts {
		opt(&r.conf)
	}

	if _, err := creds.Get(); err != nil {
		return nil, fmt.Errorf("failed to retrieve credentials value: %w", err)
	}
	return &r, nil
}

// Refresher periodically refreshes AWS credentials to avoid STS penalty within client's API requests.
type Refresher struct {
	conf  Config
	creds *credentials.Credentials
}

// Run periodically fetches the credentials to keep them always active.
// Note, the call is blocking waiting for context cancellation.
func (r *Refresher) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-time.After(r.conf.period):
			if _, err := r.creds.Get(); err != nil {
				r.conf.logger.Log("msg", "failed to refresh aws credentials", "err", err)
			}
		}
	}
}
