package resolver

import (
	"context"
	"errors"
	"fmt"

	"github.com/TrueCloudLab/frostfs-sdk-go/pool"
)

// NeoFSResolver represents virtual connection to the NeoFS network.
// It implements resolver.NeoFS.
type NeoFSResolver struct {
	pool *pool.Pool
}

// NewNeoFSResolver creates new NeoFSResolver using provided pool.Pool.
func NewNeoFSResolver(p *pool.Pool) *NeoFSResolver {
	return &NeoFSResolver{pool: p}
}

// SystemDNS implements resolver.NeoFS interface method.
func (x *NeoFSResolver) SystemDNS(ctx context.Context) (string, error) {
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
