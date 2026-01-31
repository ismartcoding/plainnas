package graph

import (
	"context"
	"strings"
	"time"

	"ismartcoding/plainnas/internal/dlna"
	"ismartcoding/plainnas/internal/graph/model"
)

func dlnaRenderersModel(ctx context.Context) ([]*model.DlnaRenderer, error) {
	clientID, _ := ctx.Value(ContextKeyClientID).(string)
	clientID = strings.TrimSpace(clientID)
	if clientID != "" {
		dlna.StartRendererDiscovery(clientID)
	}

	rs := dlna.CachedRenderers()

	out := make([]*model.DlnaRenderer, 0, len(rs))
	for _, r := range rs {
		rr := r
		out = append(out, &model.DlnaRenderer{
			Udn:          rr.UDN,
			Name:         rr.Name,
			Manufacturer: nullIfEmpty(rr.Manufacturer),
			ModelName:    nullIfEmpty(rr.ModelName),
			Location:     rr.Location,
		})
	}
	return out, nil
}

func dlnaCastModel(ctx context.Context, rendererUdn string, url string, title string, mime string, typeArg model.DataType) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	mediaType := dlna.MediaTypeVideo
	switch typeArg {
	case model.DataTypeAudio:
		mediaType = dlna.MediaTypeAudio
	case model.DataTypeImage:
		mediaType = dlna.MediaTypeImage
	case model.DataTypeVideo:
		mediaType = dlna.MediaTypeVideo
	default:
		mediaType = dlna.MediaTypeVideo
	}

	if err := dlna.Cast(ctx, rendererUdn, url, title, mime, mediaType); err != nil {
		return false, err
	}
	return true, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
