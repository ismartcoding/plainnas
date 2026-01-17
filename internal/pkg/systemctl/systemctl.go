package systemctl

import (
	"fmt"
	"ismartcoding/plainnas/internal/pkg/cmd"
	"strings"
	"time"
)

type ServiceStatus struct {
	Active  bool
	Enabled bool
}

func Enable(serviceName string, enable bool) ([]byte, error) {
	action := "enable"
	if !enable {
		action = "disable"
	}

	return cmd.Run(fmt.Sprintf("sudo systemctl %s %s", action, serviceName))
}

func GetLatestLogs(serviceName string) string {
	t := time.Now()
	t.Add(-time.Second * time.Duration(5))
	// data format: "2021-12-26 08:00:00"
	r, _ := cmd.Run(fmt.Sprintf(`sudo journalctl -u %s -n 50 --no-pager --since "%s"`, serviceName, t.Format("2006-01-02 15:04:05")))
	return strings.TrimSpace(string(r))
}

func Start(serviceName string, enable bool) ([]byte, error) {
	action := "restart"
	if !enable {
		action = "stop"
	}

	return cmd.Run(fmt.Sprintf("sudo systemctl %s %s", action, serviceName))
}

func Active(serviceName string) bool {
	r, _ := cmd.Run(fmt.Sprintf("sudo systemctl is-active %s", serviceName))
	return strings.TrimSpace(string(r)) == "active"
}

func Enabled(serviceName string) bool {
	r, _ := cmd.Run(fmt.Sprintf("sudo systemctl is-enabled %s", serviceName))
	return strings.TrimSpace(string(r)) == "enabled"
}

func GetStatus(serviceName string) ServiceStatus {
	r, _ := cmd.Run(fmt.Sprintf("sudo systemctl show %s --property=UnitFileState,ActiveState", serviceName))
	s := string(r)
	return ServiceStatus{
		Enabled: strings.Contains(s, "=enabled"),
		Active:  strings.Contains(s, "=active"),
	}
}
