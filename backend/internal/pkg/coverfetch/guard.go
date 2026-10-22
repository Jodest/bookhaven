package coverfetch

import (
	"errors"
	"net/netip"
	"strings"
	"syscall"
)

// ErrBlockedAddress is returned by the dial guard for non-public addresses.
var ErrBlockedAddress = errors.New("coverfetch: address is not public")

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:db8::/32"),
}

// IsPublicAddr reports whether a is a publicly routable unicast address.
// IPv4-mapped IPv6 forms are unmapped first.
func IsPublicAddr(a netip.Addr) bool {
	a = a.Unmap().WithZone("")
	if !a.IsValid() || a.IsLoopback() || a.IsPrivate() || a.IsLinkLocalUnicast() ||
		a.IsLinkLocalMulticast() || a.IsInterfaceLocalMulticast() || a.IsMulticast() || a.IsUnspecified() {
		return false
	}
	for _, p := range blockedPrefixes {
		if p.Contains(a) {
			return false
		}
	}
	return true
}

// checkDialAddress validates a resolved "ip:port" dial target.
func checkDialAddress(address string) error {
	ap, err := netip.ParseAddrPort(address)
	if err != nil {
		return ErrBlockedAddress
	}
	if !IsPublicAddr(ap.Addr()) {
		return ErrBlockedAddress
	}
	if p := ap.Port(); p != 80 && p != 443 {
		return ErrBlockedAddress
	}
	return nil
}

// guardControl is a net.Dialer.Control hook: it runs for every address the
// dialer is about to connect to, after DNS resolution, so it also covers
// redirect hops and DNS answers that point inside the network.
func guardControl(_, address string, _ syscall.RawConn) error {
	return checkDialAddress(address)
}

// MatchHost reports whether host is allowed by the allowlist. An entry with a
// leading "*." allows subdomains (but not the bare domain); other entries are
// exact matches. Comparison is case-insensitive.
func MatchHost(allowed []string, host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	if host == "" {
		return false
	}
	for _, e := range allowed {
		e = strings.ToLower(strings.TrimSpace(e))
		if e == "" {
			continue
		}
		if strings.HasPrefix(e, "*.") {
			suffix := e[1:] // ".archive.org"
			if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
				return true
			}
			continue
		}
		if host == e {
			return true
		}
	}
	return false
}

// ParseHosts splits a comma-separated allowlist.
func ParseHosts(csv string) []string {
	var out []string
	for _, p := range strings.Split(csv, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
