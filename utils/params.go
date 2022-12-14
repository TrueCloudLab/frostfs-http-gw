package utils

import (
	"github.com/TrueCloudLab/frostfs-http-gw/resolver"
	"github.com/TrueCloudLab/frostfs-sdk-go/pool"
	"github.com/TrueCloudLab/frostfs-sdk-go/user"
	"go.uber.org/zap"
)

type AppParams struct {
	Logger   *zap.Logger
	Pool     *pool.Pool
	Owner    *user.ID
	Resolver *resolver.ContainerResolver
}
