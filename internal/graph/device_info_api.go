package graph

import (
	"bufio"
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"ismartcoding/plainnas/internal/graph/model"
	"ismartcoding/plainnas/internal/version"
)

func buildDeviceInfo() (*model.DeviceInfo, error) {
	hostname, _ := os.Hostname()
	uts := syscall.Utsname{}
	_ = syscall.Uname(&uts)
	kernel := charsToString(uts.Release)
	cpuThreads := runtime.NumCPU()

	var l1, l5, l15 float64
	if data, err := os.ReadFile("/proc/loadavg"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 3 {
			pf := func(s string) float64 {
				f, _ := strconv.ParseFloat(s, 64)
				return f
			}
			l1, l5, l15 = pf(parts[0]), pf(parts[1]), pf(parts[2])
		}
	}

	var memTotal, memAvail, swapTotal, swapFree uint64
	if f, err := os.Open("/proc/meminfo"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "MemTotal:") {
				memTotal = parseMeminfoKB(line) * 1024
			} else if strings.HasPrefix(line, "MemAvailable:") {
				memAvail = parseMeminfoKB(line) * 1024
			} else if strings.HasPrefix(line, "SwapTotal:") {
				swapTotal = parseMeminfoKB(line) * 1024
			} else if strings.HasPrefix(line, "SwapFree:") {
				swapFree = parseMeminfoKB(line) * 1024
			}
		}
	}
	// physical memory usage not required in API response
	swapUsed := swapTotal - swapFree

	var uptimeSec float64
	if data, err := os.ReadFile("/proc/uptime"); err == nil {
		parts := strings.Fields(string(data))
		if len(parts) >= 1 {
			uptimeSec, _ = strconv.ParseFloat(parts[0], 64)
		}
	}
	bootTime := time.Now().Add(-time.Duration(uptimeSec) * time.Second).Unix()

	cpuModel := ""
	cpuCores := 0
	if f, err := os.Open("/proc/cpuinfo"); err == nil {
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "model name") {
				if i := strings.Index(line, ":"); i >= 0 {
					cpuModel = strings.TrimSpace(line[i+1:])
				}
			} else if strings.HasPrefix(line, "cpu cores") {
				if i := strings.Index(line, ":"); i >= 0 {
					if n, err := strconv.Atoi(strings.TrimSpace(line[i+1:])); err == nil {
						cpuCores = n
					}
				}
			}
		}
	}
	if cpuCores == 0 {
		cpuCores = cpuThreads
	}

	var ips []string
	var nics []*model.NicInfo
	if ifs, err := net.Interfaces(); err == nil {
		for _, it := range ifs {
			if (it.Flags & net.FlagUp) == 0 {
				continue
			}
			if it.Name == "lo" || (it.Flags&net.FlagLoopback) != 0 {
				continue
			}
			addrs, _ := it.Addrs()
			for _, a := range addrs {
				switch v := a.(type) {
				case *net.IPNet:
					if v.IP != nil && v.IP.IsLoopback() {
						continue
					}
				case *net.IPAddr:
					if v.IP != nil && v.IP.IsLoopback() {
						continue
					}
				}
				ips = append(ips, a.String())
			}
			if len(it.HardwareAddr) == 0 {
				continue
			}
			speedMbps := 0
			if b, err := os.ReadFile("/sys/class/net/" + it.Name + "/speed"); err == nil {
				if v, err := strconv.Atoi(strings.TrimSpace(string(b))); err == nil {
					speedMbps = v
				}
			}
			nics = append(nics, &model.NicInfo{
				Name:      it.Name,
				Mac:       it.HardwareAddr.String(),
				SpeedRate: int64(speedMbps) * 1000 * 1000 / 8,
			})
		}
	}

	modelName := ""
	if data, err := os.ReadFile("/proc/device-tree/model"); err == nil {
		modelName = strings.TrimSpace(string(data))
	}
	if modelName == "" {
		if data, err := os.ReadFile("/proc/device-tree/compatible"); err == nil {
			parts := strings.SplitN(strings.TrimSpace(string(data)), ",", 2)
			if len(parts) > 0 {
				modelName = parts[0]
			}
		}
	}

	d := &model.DeviceInfo{
		Hostname:         hostname,
		Os:               runtime.GOOS,
		KernelVersion:    kernel,
		AppVersion:       version.Version,
		AppFullVersion:   version.FullVersion(),
		Arch:             runtime.GOARCH,
		Uptime:           int64(uptimeSec * 1000),
		BootTime:         bootTime * 1000,
		CPUModel:         cpuModel,
		CPUCores:         cpuCores,
		CPUThreads:       cpuThreads,
		Load1:            l1,
		Load5:            l5,
		Load15:           l15,
		MemoryTotalBytes: int64(memTotal),
		MemoryFreeBytes:  int64(memAvail),
		SwapTotalBytes:   int64(swapTotal),
		SwapFreeBytes:    int64(swapFree),
		SwapUsedBytes:    int64(swapUsed),
		Ips:              ips,
		Nics:             nics,
		Model:            modelName,
	}
	return d, nil
}
