package agent

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	// Tailscale CGNAT subnet (RFC 6598: 100.64.0.0/10)
	_, tailscaleIPv4Net, _ = net.ParseCIDR("100.64.0.0/10")
	// Tailscale IPv6 ULA subnet (fd7a:115c:a1e0::/48)
	_, tailscaleIPv6Net, _ = net.ParseCIDR("fd7a:115c:a1e0::/48")

	cachedPublicIP     string
	cachedPublicIPOnce sync.Once
)

// IsTailscaleOrLocal checks whether an incoming request remote address is
// from Tailscale (100.64.0.0/10, fd7a:115c:a1e0::/48) or localhost loopback.
func IsTailscaleOrLocal(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(strings.TrimSpace(host))
	if ip == nil {
		return false
	}

	// Always allow localhost loopback for local daemons/proxies
	if ip.IsLoopback() {
		return true
	}

	// Tailscale IPv4
	if tailscaleIPv4Net != nil && tailscaleIPv4Net.Contains(ip) {
		return true
	}

	// Tailscale IPv6
	if tailscaleIPv6Net != nil && tailscaleIPv6Net.Contains(ip) {
		return true
	}

	return false
}

// FindLocalTailscaleIP searches the server's network interfaces for an IP in 100.64.0.0/10
func FindLocalTailscaleIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		// Ignore down or loopback interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip != nil && ip.To4() != nil && tailscaleIPv4Net != nil && tailscaleIPv4Net.Contains(ip) {
				return ip.String()
			}
		}
	}
	return ""
}

// DetectPublicIP discovers the real external public IPv4 address of the VPS.
// It ensures the detected IP is NOT a Tailscale IP (100.64.0.0/10), NOT private, and NOT localhost.
func DetectPublicIP() string {
	if cachedPublicIP != "" {
		return cachedPublicIP
	}

	endpoints := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}

	client := &http.Client{Timeout: 3 * time.Second}
	for _, ep := range endpoints {
		ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, ep, nil)
		if err != nil {
			cancel()
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			cancel()
			continue
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()

		if err == nil {
			ipStr := strings.TrimSpace(string(body))
			ip := net.ParseIP(ipStr)
			if ip != nil && ip.To4() != nil {
				// Must not be private, not loopback, not tailscale
				if !ip.IsPrivate() && !ip.IsLoopback() && (tailscaleIPv4Net == nil || !tailscaleIPv4Net.Contains(ip)) {
					cachedPublicIP = ipStr
					return ipStr
				}
			}
		}
	}

	return ""
}

// SetCachedPublicIP explicitly sets the known public IP/host
func SetCachedPublicIP(ipOrHost string) {
	cachedPublicIP = strings.TrimSpace(ipOrHost)
}
