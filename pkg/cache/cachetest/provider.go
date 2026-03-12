package cachetest

import (
	"context"

	"github.com/hanzoai/o11y/pkg/cache"
	"github.com/hanzoai/o11y/pkg/cache/memorycache"
	"github.com/hanzoai/o11y/pkg/factory/factorytest"
)

func New(config cache.Config) (cache.Cache, error) {
	cache, err := memorycache.New(context.TODO(), factorytest.NewSettings(), config)
	if err != nil {
		return nil, err
	}

	return cache, nil
}
