package resolver

import (
	"context"
	"errors"
	"fmt"

	"github.com/TrueCloudLab/frostfs-sdk-go/pool"
)

// FrostFSResolver represents virtual connection to the FrostFS network.
// It implements resolver.FrostFS.
type FrostFSResolver struct {
	pool *pool.Pool
}

// NewFrostFSResolver creates new FrostFSResolver using provided pool.Pool.
func NewFrostFSResolver(p *pool.Pool) *FrostFSResolver {
	return &FrostFSResolver{pool: p}
}

// SystemDNS implements resolver.FrostFS interface method.
func (x *FrostFSResolver) SystemDNS(ctx context.Context) (string, error) {
	networkInfo, err := x.pool.NetworkInfo(ctx)
	if err != nil {
		return "", fmt.Errorf("read network info via client: %w", err)
	}

	domain := networkInfo.RawNetworkParameter("SystemDNS")
	if domain == nil {
		return "", errors.New("system DNS parameter not found or empty")
	}

	return string(domain), nil
}
