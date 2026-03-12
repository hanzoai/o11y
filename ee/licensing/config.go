package licensing

import (
	"sync"
	"time"

	"github.com/hanzoai/o11y/pkg/errors"
	"github.com/hanzoai/o11y/pkg/licensing"
)

var (
	config licensing.Config
	once   sync.Once
)

// initializes the licensing configuration
func Config(pollInterval time.Duration, failureThreshold int) licensing.Config {
	once.Do(func() {
		config = licensing.Config{PollInterval: pollInterval, FailureThreshold: failureThreshold}
		if err := config.Validate(); err != nil {
			panic(errors.WrapInternalf(err, errors.CodeInternal, "invalid licensing config"))
		}
	})

	return config
}
