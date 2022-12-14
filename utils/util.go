package utils

import (
	"context"

	"github.com/TrueCloudLab/frostfs-http-gw/resolver"
	cid "github.com/TrueCloudLab/frostfs-sdk-go/container/id"
)

// GetContainerID decode container id, if it's not a valid container id
// then trey to resolve name using provided resolver.
func GetContainerID(ctx context.Context, containerID string, resolver *resolver.ContainerResolver) (*cid.ID, error) {
	cnrID := new(cid.ID)
	err := cnrID.DecodeString(containerID)
	if err != nil {
		cnrID, err = resolver.Resolve(ctx, containerID)
	}
	return cnrID, err
}
