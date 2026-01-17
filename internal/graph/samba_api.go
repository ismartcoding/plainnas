package graph

import (
	"context"
	"fmt"
	"strings"

	"ismartcoding/plainnas/internal/db"
	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/samba"
)

func setSambaSettings(ctx context.Context, input model.SambaSettingsInput) (bool, error) {
	prev := db.GetSambaSettings()

	desired := db.SambaSettings{
		Enabled:     input.Enabled,
		HasPassword: prev.HasPassword,
		Shares:      make([]db.SambaShare, 0, len(input.Shares)),
	}

	requiresPassword := false
	for _, sh := range input.Shares {
		d := db.SambaShare{
			Name:      sh.Name,
			SharePath: sh.SharePath,
			Auth:      db.SambaShareAuth(sh.Auth),
			ReadOnly:  sh.ReadOnly,
		}
		if d.Auth == db.SambaShareAuthPassword {
			requiresPassword = true
		}
		desired.Shares = append(desired.Shares, d)
	}

	if desired.Enabled && len(desired.Shares) == 0 {
		return false, fmt.Errorf("no shares configured")
	}
	if requiresPassword && !prev.HasPassword {
		return false, fmt.Errorf("password required")
	}

	if err := db.StoreSambaSettings(desired); err != nil {
		return false, err
	}

	// Use normalized settings after store.
	desired = db.GetSambaSettings()
	if err := samba.Apply(desired, ""); err != nil {
		return false, err
	}
	return true, nil
}

func setSambaUserPassword(ctx context.Context, password string) (bool, error) {
	if strings.TrimSpace(password) == "" {
		return false, fmt.Errorf("password required")
	}
	if err := samba.SetUserPassword(password); err != nil {
		return false, err
	}

	s := db.GetSambaSettings()
	s.HasPassword = true
	_ = db.StoreSambaSettings(s)

	if s.Enabled {
		_ = samba.Apply(s, "")
	}
	return true, nil
}

func sambaSettings(ctx context.Context) (*model.SambaSettings, error) {
	s := db.GetSambaSettings()
	service := samba.GetServiceStatus()
	shares := make([]*model.SambaShare, 0, len(s.Shares))
	for _, sh := range s.Shares {
		shares = append(shares, &model.SambaShare{
			Name:      sh.Name,
			SharePath: sh.SharePath,
			Auth:      model.SambaShareAuth(sh.Auth),
			ReadOnly:  sh.ReadOnly,
		})
	}
	return &model.SambaSettings{
		Enabled:        s.Enabled,
		Username:       "nas",
		HasPassword:    s.HasPassword,
		Shares:         shares,
		ServiceName:    service.Name,
		ServiceActive:  service.Active,
		ServiceEnabled: service.Enabled,
	}, nil
}
