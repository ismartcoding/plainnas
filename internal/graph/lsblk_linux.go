//go:build linux

package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type lsblkOutput struct {
	BlockDevices []lsblkDevice `json:"blockdevices"`
}

type lsblkDevice struct {
	Name       string        `json:"name"`
	Path       string        `json:"path"`
	Type       string        `json:"type"`
	Fstype     string        `json:"fstype"`
	UUID       string        `json:"uuid"`
	Label      string        `json:"label"`
	SizeBytes  json.Number   `json:"size"`
	Mountpoint string        `json:"mountpoint"`
	PKName     string        `json:"pkname"`
	PartN      json.Number   `json:"partn"`
	RM         any           `json:"rm"`
	Model      string        `json:"model"`
	Children   []lsblkDevice `json:"children"`
}

func runLsblkJSON(cols []string) (*lsblkOutput, error) {
	cols = append([]string{}, cols...)
	for i := range cols {
		cols[i] = strings.TrimSpace(cols[i])
	}

	cmd := exec.Command("lsblk", "-b", "-J", "-o", strings.Join(cols, ","))
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("lsblk failed: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}

	var parsed lsblkOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseJSONInt(n json.Number) (int, bool) {
	if n == "" {
		return 0, false
	}
	if v, err := n.Int64(); err == nil {
		return int(v), true
	}
	if v, err := strconv.Atoi(n.String()); err == nil {
		return v, true
	}
	return 0, false
}

func parseLsblkBoolish(v any) (bool, bool) {
	switch x := v.(type) {
	case nil:
		return false, false
	case bool:
		return x, true
	case float64:
		return x != 0, true
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		if s == "" {
			return false, false
		}
		if s == "1" || s == "true" || s == "yes" {
			return true, true
		}
		if s == "0" || s == "false" || s == "no" {
			return false, true
		}
		return false, false
	default:
		return false, false
	}
}
