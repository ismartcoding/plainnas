package graph

import (
	"context"
	"fmt"
)

func setDeviceNameModel(ctx context.Context, name string) (bool, error) {
	hostname := sanitizeHostname(name)
	if hostname == "" {
		return false, fmt.Errorf("device_name_invalid")
	}
	if err := setLinuxHostname(hostname); err != nil {
		return false, err
	}
	if err := restartAvahiDaemon(); err != nil {
		return false, err
	}
	return true, nil
}
