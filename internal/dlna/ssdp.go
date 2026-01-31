package dlna

import (
	"context"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"ismartcoding/plainnas/internal/pkg/log"
)

func ssdpSearch(ctx context.Context, searchTargets []string) ([]upnpDiscovered, error) {
	if len(searchTargets) == 0 {
		return nil, nil
	}

	extraDests := ssdpExtraDestinations()
	if dlnaDebugEnabled() && len(extraDests) > 0 {
		parts := make([]string, 0, len(extraDests))
		for _, d := range extraDests {
			parts = append(parts, d.String())
		}
		log.Infof("[DLNA] ssdp: extraDests=%s (from PLAINNAS_DLNA_SSDP_EXTRA_DESTS)", strings.Join(parts, ","))
	}

	// Collect IPv4 addresses to send from.
	// IMPORTANT: always try an unbound socket first (kernel chooses route/interface),
	// which matches PlainAPP's MulticastSocket behavior and helps on hosts with
	// multiple interfaces / policy routing.
	addrs, err := localIPv4AddrInfo()
	if err != nil {
		return nil, err
	}
	// Prepend "auto" bind (0.0.0.0) and de-dup.
	uniq := make([]localIPv4Addr, 0, len(addrs)+1)
	uniq = append(uniq, localIPv4Addr{})
	seenIP := make(map[string]struct{}, len(addrs))
	for _, a := range addrs {
		if a.IP == nil {
			continue
		}
		k := a.IP.String()
		if _, ok := seenIP[k]; ok {
			continue
		}
		seenIP[k] = struct{}{}
		uniq = append(uniq, a)
	}
	addrs = uniq

	deadline := time.Now().Add(5 * time.Second)
	if dl, ok := ctx.Deadline(); ok {
		// Keep SSDP phase bounded even if caller has longer context.
		if dl.Before(deadline) {
			deadline = dl
		}
	}
	if dlnaDebugEnabled() {
		binds := make([]string, 0, len(addrs))
		for _, a := range addrs {
			if a.IP == nil {
				binds = append(binds, "<auto>")
			} else {
				binds = append(binds, a.IP.String())
			}
		}
		log.Infof("[DLNA] ssdp: bindAddrs=%s deadline=%s", strings.Join(binds, ","), deadline.Format(time.RFC3339Nano))
	}

	var out []upnpDiscovered
	seenLoc := make(map[string]struct{})

	for _, a := range addrs {
		bindIP := a.IP
		c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: bindIP, Port: 0})
		if err != nil {
			if dlnaDebugEnabled() {
				log.Infof("[DLNA] ssdp: bind failed ip=%v err=%v", bindIP, err)
			}
			continue
		}
		// Enable broadcast sends for subnet broadcast fallback.
		if rc, err := c.SyscallConn(); err == nil {
			_ = rc.Control(func(fd uintptr) {
				_ = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
			})
		}
		_ = c.SetReadBuffer(1 << 20)
		_ = c.SetWriteBuffer(1 << 20)
		_ = c.SetDeadline(deadline)

		mcast := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 1900}
		var bcast *net.UDPAddr
		if a.Broadcast != nil {
			bcast = &net.UDPAddr{IP: a.Broadcast, Port: 1900}
			if dlnaDebugEnabled() {
				log.Infof("[DLNA] ssdp: subnet broadcast=%s", bcast.String())
			}
		}

		for _, st := range searchTargets {
			q := buildMSearch(st)
			if dlnaDebugEnabled() && dlnaDebugPayloadEnabled() {
				log.Infof("[DLNA] ssdp: tx st=%s\n%s", st, truncateForLog(q, 1024))
			}
			// Many devices respond more reliably if M-SEARCH is repeated.
			for i := 0; i < 3; i++ {
				if dlnaDebugEnabled() {
					log.Infof("[DLNA] ssdp: send st=%s from=%s to=%s bytes=%d", st, udpAddrString(bindIP), mcast.String(), len(q))
				}
				if _, err := c.WriteToUDP([]byte(q), mcast); err != nil {
					if dlnaDebugEnabled() {
						log.Infof("[DLNA] ssdp: send failed st=%s from=%s to=%s err=%v", st, udpAddrString(bindIP), mcast.String(), err)
					}
				}
				if bcast != nil {
					if dlnaDebugEnabled() {
						log.Infof("[DLNA] ssdp: send st=%s from=%s to=%s bytes=%d", st, udpAddrString(bindIP), bcast.String(), len(q))
					}
					if _, err := c.WriteToUDP([]byte(q), bcast); err != nil {
						if dlnaDebugEnabled() {
							log.Infof("[DLNA] ssdp: send failed st=%s from=%s to=%s err=%v", st, udpAddrString(bindIP), bcast.String(), err)
						}
					}
				}
				for _, ed := range extraDests {
					if dlnaDebugEnabled() {
						log.Infof("[DLNA] ssdp: send st=%s from=%s to=%s bytes=%d", st, udpAddrString(bindIP), ed.String(), len(q))
					}
					if _, err := c.WriteToUDP([]byte(q), ed); err != nil {
						if dlnaDebugEnabled() {
							log.Infof("[DLNA] ssdp: send failed st=%s from=%s to=%s err=%v", st, udpAddrString(bindIP), ed.String(), err)
						}
					}
				}
				if i < 2 {
					select {
					case <-time.After(60 * time.Millisecond):
					case <-ctx.Done():
						break
					}
				}
			}
		}

		buf := make([]byte, 4096)
		// Stop reading shortly after the last response so we don't burn the
		// caller's context budget (needed for fetching device descriptions).
		const idleAfterLastResponse = 250 * time.Millisecond
		haveRx := false
		lastRx := time.Now()
		for {
			if ctx.Err() != nil {
				break
			}
			// Use short read deadlines so we can break once responses go idle,
			// but still respect the overall socket deadline.
			now := time.Now()
			chunkDL := now.Add(300 * time.Millisecond)
			if chunkDL.After(deadline) {
				chunkDL = deadline
			}
			_ = c.SetReadDeadline(chunkDL)

			n, ra, err := c.ReadFromUDP(buf)
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Timeout() {
					if haveRx && time.Since(lastRx) >= idleAfterLastResponse {
						break
					}
					continue
				}
				break
			}
			if n <= 0 {
				continue
			}
			haveRx = true
			lastRx = time.Now()
			resp := string(buf[:n])
			// Accept both discovery responses and NOTIFY advertisements.
			upperPrefix := strings.ToUpper(strings.TrimSpace(firstN(resp, 20)))
			if !(strings.HasPrefix(upperPrefix, "HTTP/1.1 200") || strings.HasPrefix(upperPrefix, "NOTIFY * HTTP")) {
				continue
			}

			h := parseSSDPHeaders(resp)
			loc := strings.TrimSpace(h["location"])
			if loc == "" {
				continue
			}
			if _, ok := seenLoc[loc]; ok {
				continue
			}
			seenLoc[loc] = struct{}{}

			if dlnaDebugEnabled() {
				log.Infof("[DLNA] ssdp: rx from=%s location=%s usn=%s st=%s server=%s", ra.IP.String(), loc, strings.TrimSpace(h["usn"]), strings.TrimSpace(h["st"]), strings.TrimSpace(h["server"]))
				if dlnaDebugPayloadEnabled() {
					log.Infof("[DLNA] ssdp: rxRaw from=%s\n%s", ra.IP.String(), truncateForLog(resp, 2048))
				}
			}

			out = append(out, upnpDiscovered{
				RemoteAddr: ra.IP.String(),
				Location:   loc,
				USN:        strings.TrimSpace(h["usn"]),
				ST:         strings.TrimSpace(h["st"]),
				Server:     strings.TrimSpace(h["server"]),
			})
		}

		_ = c.Close()
	}

	return out, nil
}

func buildMSearch(st string) string {
	st = strings.TrimSpace(st)
	if st == "" {
		st = "ssdp:all"
	}
	// IMPORTANT: two empty lines at end (PlainAPP comment: otherwise some TV OS can't recognize).
	// Header order matches PlainAPP (some devices are picky).
	// NOTE: PlainAPP uses LF-only newlines; we mirror that for maximum compatibility.
	return "M-SEARCH * HTTP/1.1\n" +
		"ST: " + st + "\n" +
		"HOST: 239.255.255.250:1900\n" +
		"MX: 3\n" +
		"MAN: \"ssdp:discover\"\n" +
		"\n\n"
}

func parseSSDPHeaders(resp string) map[string]string {
	lines := strings.Split(resp, "\n")
	out := make(map[string]string, len(lines))
	for _, l := range lines {
		l = strings.TrimSpace(strings.TrimRight(l, "\r"))
		if l == "" {
			continue
		}
		idx := strings.IndexByte(l, ':')
		if idx <= 0 {
			continue
		}
		k := strings.ToLower(strings.TrimSpace(l[:idx]))
		v := strings.TrimSpace(l[idx+1:])
		out[k] = v
	}
	return out
}

func parseUDNFromUSN(usn string) string {
	usn = strings.TrimSpace(usn)
	if usn == "" {
		return ""
	}
	// Typical: uuid:xxxxxxxx-xxxx-....::urn:schemas-upnp-org:device:MediaRenderer:1
	if i := strings.Index(usn, "::"); i >= 0 {
		usn = usn[:i]
	}
	usn = strings.TrimSpace(usn)
	if strings.HasPrefix(strings.ToLower(usn), "uuid:") {
		return "uuid:" + strings.TrimSpace(usn[5:])
	}
	return usn
}

type localIPv4Addr struct {
	IP        net.IP
	Broadcast net.IP
}

func localIPv4AddrInfo() ([]localIPv4Addr, error) {
	ifs, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []localIPv4Addr
	for _, it := range ifs {
		if (it.Flags & net.FlagUp) == 0 {
			continue
		}
		if (it.Flags & net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := it.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			var ip net.IP
			var mask net.IPMask
			switch v := a.(type) {
			case *net.IPNet:
				ip = v.IP
				mask = v.Mask
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil {
				continue
			}
			ip4 := ip.To4()
			if ip4 == nil {
				continue
			}
			var bcast net.IP
			if len(mask) == 4 {
				b := make(net.IP, 4)
				for i := 0; i < 4; i++ {
					b[i] = ip4[i] | ^mask[i]
				}
				bcast = b
			}
			out = append(out, localIPv4Addr{IP: ip4, Broadcast: bcast})
		}
	}
	return out, nil
}

func firstN(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func udpAddrString(ip net.IP) string {
	if ip == nil {
		return "0.0.0.0"
	}
	return ip.String()
}

func ssdpExtraDestinations() []*net.UDPAddr {
	// Comma-separated list of IP[:port] destinations.
	// Useful when multicast is filtered; you can try unicast to a known TV IP
	// or broadcast (e.g. 255.255.255.255).
	v := strings.TrimSpace(os.Getenv("PLAINNAS_DLNA_SSDP_EXTRA_DESTS"))
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	seen := make(map[string]struct{})
	out := make([]*net.UDPAddr, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		addrStr := p
		if !strings.Contains(addrStr, ":") {
			addrStr = addrStr + ":1900"
		}
		ua, err := net.ResolveUDPAddr("udp4", addrStr)
		if err != nil || ua == nil || ua.IP == nil {
			continue
		}
		key := ua.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, ua)
	}
	return out
}
