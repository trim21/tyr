package util

import (
	"fmt"
	"net"

	"github.com/samber/lo"
	"github.com/trim21/errgo"
)

func GetLocalIpaddress(enabledIf []string) (map[string][]net.IP, error) {
	ifces, err := net.Interfaces()
	if err != nil {
		return nil, errgo.Wrap(err, "failed to get network interfaces")
	}

	result := make(map[string][]net.IP, len(ifces))

	// handle err
	for _, i := range ifces {
		if i.Flags&net.FlagLoopback == net.FlagLoopback {
			continue
		}

		if len(enabledIf) != 0 {
			if !lo.Contains(enabledIf, i.Name) {
				continue
			}
		}

		addrs, err := i.Addrs()
		if err != nil {
			return nil, errgo.Wrap(err, fmt.Sprintf("failed to get address of net interface %s", i.Name))
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				continue
			}

			if IsPrivateIP(ip) {
				continue
			}

			result[i.Name] = append(result[i.Name], ip)
		}
	}

	return result, nil
}

var privateIPBlocks []*net.IPNet

func init() {
	for _, cidr := range []string{
		"127.0.0.0/8",    // IPv4 loopback
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"169.254.0.0/16", // RFC3927 link-local
		"::1/128",        // IPv6 loopback
		"fe80::/10",      // IPv6 link-local
		"fc00::/7",       // IPv6 unique local addr
	} {
		_, block, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(fmt.Errorf("parse error on %q: %v", cidr, err))
		}
		privateIPBlocks = append(privateIPBlocks, block)
	}
}

func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}
