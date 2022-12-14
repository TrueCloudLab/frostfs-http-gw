package resolver

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/TrueCloudLab/frostfs-sdk-go/container"
	cid "github.com/TrueCloudLab/frostfs-sdk-go/container/id"
	"github.com/TrueCloudLab/frostfs-sdk-go/ns"
)

const (
	NNSResolver = "nns"
	DNSResolver = "dns"
)

// ErrNoResolvers returns when trying to resolve container without any resolver.
var ErrNoResolvers = errors.New("no resolvers")

// NeoFS represents virtual connection to the NeoFS network.
type NeoFS interface {
	// SystemDNS reads system DNS network parameters of the NeoFS.
	//
	// Returns exactly on non-zero value. Returns any error encountered
	// which prevented the parameter to be read.
	SystemDNS(context.Context) (string, error)
}

type Config struct {
	NeoFS      NeoFS
	RPCAddress string
}

type ContainerResolver struct {
	mu        sync.RWMutex
	resolvers []*Resolver
}

type Resolver struct {
	Name    string
	resolve func(context.Context, string) (*cid.ID, error)
}

func (r *Resolver) SetResolveFunc(fn func(context.Context, string) (*cid.ID, error)) {
	r.resolve = fn
}

func (r *Resolver) Resolve(ctx context.Context, name string) (*cid.ID, error) {
	return r.resolve(ctx, name)
}

func NewContainerResolver(resolverNames []string, cfg *Config) (*ContainerResolver, error) {
	resolvers, err := createResolvers(resolverNames, cfg)
	if err != nil {
		return nil, err
	}

	return &ContainerResolver{
		resolvers: resolvers,
	}, nil
}

func createResolvers(resolverNames []string, cfg *Config) ([]*Resolver, error) {
	resolvers := make([]*Resolver, len(resolverNames))
	for i, name := range resolverNames {
		cnrResolver, err := newResolver(name, cfg)
		if err != nil {
			return nil, err
		}
		resolvers[i] = cnrResolver
	}

	return resolvers, nil
}

func (r *ContainerResolver) Resolve(ctx context.Context, cnrName string) (*cid.ID, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var err error
	for _, resolver := range r.resolvers {
		cnrID, resolverErr := resolver.Resolve(ctx, cnrName)
		if resolverErr != nil {
			resolverErr = fmt.Errorf("%s: %w", resolver.Name, resolverErr)
			if err == nil {
				err = resolverErr
			} else {
				err = fmt.Errorf("%s: %w", err.Error(), resolverErr)
			}
			continue
		}
		return cnrID, nil
	}

	if err != nil {
		return nil, err
	}

	return nil, ErrNoResolvers
}

func (r *ContainerResolver) UpdateResolvers(resolverNames []string, cfg *Config) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.equals(resolverNames) {
		return nil
	}

	resolvers, err := createResolvers(resolverNames, cfg)
	if err != nil {
		return err
	}

	r.resolvers = resolvers

	return nil
}

func (r *ContainerResolver) equals(resolverNames []string) bool {
	if len(r.resolvers) != len(resolverNames) {
		return false
	}

	for i := 0; i < len(resolverNames); i++ {
		if r.resolvers[i].Name != resolverNames[i] {
			return false
		}
	}
	return true
}

func newResolver(name string, cfg *Config) (*Resolver, error) {
	switch name {
	case DNSResolver:
		return NewDNSResolver(cfg.NeoFS)
	case NNSResolver:
		return NewNNSResolver(cfg.RPCAddress)
	default:
		return nil, fmt.Errorf("unknown resolver: %s", name)
	}
}

func NewDNSResolver(neoFS NeoFS) (*Resolver, error) {
	if neoFS == nil {
		return nil, fmt.Errorf("pool must not be nil for DNS resolver")
	}

	var dns ns.DNS

	resolveFunc := func(ctx context.Context, name string) (*cid.ID, error) {
		domain, err := neoFS.SystemDNS(ctx)
		if err != nil {
			return nil, fmt.Errorf("read system DNS parameter of the NeoFS: %w", err)
		}

		domain = name + "." + domain
		cnrID, err := dns.ResolveContainerName(domain)
		if err != nil {
			return nil, fmt.Errorf("couldn't resolve container '%s' as '%s': %w", name, domain, err)
		}
		return &cnrID, nil
	}

	return &Resolver{
		Name:    DNSResolver,
		resolve: resolveFunc,
	}, nil
}

func NewNNSResolver(rpcAddress string) (*Resolver, error) {
	var nns ns.NNS

	if err := nns.Dial(rpcAddress); err != nil {
		return nil, fmt.Errorf("could not dial nns: %w", err)
	}

	resolveFunc := func(_ context.Context, name string) (*cid.ID, error) {
		var d container.Domain
		d.SetName(name)

		cnrID, err := nns.ResolveContainerDomain(d)
		if err != nil {
			return nil, fmt.Errorf("couldn't resolve container '%s': %w", name, err)
		}
		return &cnrID, nil
	}

	return &Resolver{
		Name:    NNSResolver,
		resolve: resolveFunc,
	}, nil
}
