// Package awscreds aims to tackle AWS SDK tail latency
// introduced by AWS client calling the STS endpoint.
package awscreds

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/client"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	v4 "github.com/aws/aws-sdk-go/aws/signer/v4"
	"github.com/go-kit/log"
)

// NewCreds allows to provide your own function to create Credentials needed in NewSwapper.
type NewCreds func() (*credentials.Credentials, error)

// New creates a new Credentials with a default credentials chain.
// STS regional endpoint is used to improve latency,
// see also https://github.com/aws/aws-sdk-go/issues/4385.
//
// You would probably want to supply your own version of this function to NewSwapper,
// but at least it serves as an example.
func New() (*credentials.Credentials, error) {
	s, err := session.NewSession(&aws.Config{
		STSRegionalEndpoint: endpoints.RegionalSTSEndpoint,
	})
	if err != nil {
		return nil, err
	}
	return s.Config.Credentials, nil
}

// NewSwapper creates a new credentials swapper
// and immediately tries to fetch credentials
// to avoid an initialization penalty in client's API request.
//
// The default refresh period is 55 minutes in case a token expires every hour
// see https://docs.aws.amazon.com/eks/latest/userguide/kubernetes-versions.html.
func NewSwapper(newcreds NewCreds, opts ...Option) (*Swapper, error) {
	if newcreds == nil {
		return nil, fmt.Errorf("newcreds cannot be nil")
	}

	s := Swapper{
		conf: Config{
			logger: log.NewNopLogger(),
			period: 55 * time.Minute,
		},
		newcreds: newcreds,
	}
	for _, opt := range opts {
		opt(&s.conf)
	}

	if err := s.refresh(); err != nil {
		return nil, err
	}
	return &s, nil
}

// Swapper allows to periodically swap AWS credentials with the new ones.
type Swapper struct {
	conf     Config
	newcreds func() (*credentials.Credentials, error)
	cache    atomic.Value
}

// refresh refreshes cached credentials.
func (s *Swapper) refresh() error {
	creds, err := s.newcreds()
	if err != nil {
		return fmt.Errorf("failed to create new credentials: %w", err)
	}
	if _, err = creds.Get(); err != nil {
		return fmt.Errorf("failed to retrieve credentials value: %w", err)
	}

	s.cache.Store(creds)
	return nil
}

// Run periodically retrieves credentials and caches the value.
// Cached credentials are not replaced if an error occurs.
// Note, the call is blocking waiting for context cancellation.
func (s *Swapper) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case <-time.After(s.conf.period):
			if err := s.refresh(); err != nil {
				s.conf.logger.Log("msg", "failed to refresh aws credentials", "err", err)
			}
		}
	}
}

// Attach allows to tune a request signer of the AWS client with opts functions.
// They are called each time a signer is constructed just before signing a request.
// For example, you can change a signer's flags or its credentials fetcher.
// This is particularly valuable when refreshing AWS credentials in the background.
//
// Note, make sure to pass opts appropriate to a service you're using.
// For example, S3 service should not have any escaping applied s.DisableURIPathEscaping = true,
// see https://github.com/aws/aws-sdk-go/blob/6a228d939132a3159d0dce221879efbedd842d61/service/s3/service.go#L78.
//
// On the other hand SignSDKRequestWithCurrentTime checks the flag as well
// https://github.com/aws/aws-sdk-go/blob/6a228d939132a3159d0dce221879efbedd842d61/aws/signer/v4/v4.go#L469,
// so that particular S3 option is redundant.
func (s *Swapper) Attach(c *client.Client, opts ...func(*v4.Signer)) bool {
	opts = append(opts, func(signer *v4.Signer) {
		v := s.cache.Load()
		creds, ok := v.(*credentials.Credentials)
		if creds != nil && ok {
			signer.Credentials = creds
		} else {
			s.conf.logger.Log("msg", "failed to swap aws credentials")
		}
	})

	return c.Handlers.Sign.SwapNamed(v4.BuildNamedHandler(
		v4.SignRequestHandler.Name,
		opts...,
	))
}
