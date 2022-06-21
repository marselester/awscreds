package awscreds

import (
	"time"

	"github.com/go-kit/log"
)

// Config represents optional settings.
type Config struct {
	logger log.Logger
	period time.Duration
}

// Option sets up a Config.
type Option func(*Config)

// WithLogger sets a structured logger.
func WithLogger(l log.Logger) Option {
	return func(c *Config) {
		c.logger = l
	}
}

// WithPeriod sets up a period between credentials updates.
func WithPeriod(p time.Duration) Option {
	return func(c *Config) {
		c.period = p
	}
}
