package cf

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cloudflare/cloudflare-go"

	"github.com/whywriteit/wwi-agent/logger"
)

// Loop is a main loop
func Loop(ctx context.Context, token, homeDomain string) error {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// receive cancel
			return nil
		case <-ticker.C:
			if err := run(ctx, token, homeDomain); err != nil {
				return fmt.Errorf("failed to run cloudflare service: %w", err)
			}
		}
	}
}

// run run cloudflare
func run(ctx context.Context, token, homeDomain string) error {
	api, err := cloudflare.NewWithAPIToken(token)
	if err != nil {
		return fmt.Errorf("failed to create cloudflare client: %w", err)
	}
	hostname, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}
	ip, err := externalIP()
	if err != nil {
		return fmt.Errorf("failed to get ip address: %w", err)
	}

	zoneID, err := getZoneID(api, homeDomain)
	if err != nil {
		return fmt.Errorf("failed to get zone id: %w", err)
	}
	domain := strings.Join([]string{hostname, homeDomain}, ".")
	ok, record, err := isExistARecord(ctx, api, zoneID, domain)
	if err != nil {
		return fmt.Errorf("failed to check record: %w", err)
	}
	req := cloudflare.DNSRecord{
		Name:    domain,
		Type:    "A",
		Content: ip,
		TTL:     60 * 10,
	}
	switch {
	case !ok:
		// create record
		logger.Logf("%s is not registered in Cloudflare, will be register", domain)
		if _, err := api.CreateDNSRecord(ctx, zoneID, req); err != nil {
			return fmt.Errorf("failed to create record: %w", err)
		}
	case record.Content != ip:
		// content is old, update ip address
		logger.Logf("content of %s is old. will be update", domain)
		if err := api.UpdateDNSRecord(ctx, zoneID, record.ID, req); err != nil {
			return fmt.Errorf("failed to update record: %w", err)
		}
	}

	return nil
}

func isExistARecord(ctx context.Context, api *cloudflare.API, zoneID, domain string) (bool, *cloudflare.DNSRecord, error) {
	req := cloudflare.DNSRecord{Name: domain}
	records, err := api.DNSRecords(ctx, zoneID, req)
	if err != nil {
		return false, nil, fmt.Errorf("failed to retrieve records: %w", err)
	}
	if len(records) == 0 {
		return false, nil, nil
	}

	for _, r := range records {
		if r.Type == "A" {
			// found A Record
			return true, &r, nil
		}
	}

	return false, nil, nil
}

func getZoneID(api *cloudflare.API, domain string) (string, error) {
	s := strings.Split(domain, ".")
	if len(s) < 2 {
		return "", fmt.Errorf("domain need two area")
	}

	nakedDomain := strings.Join(s[len(s)-2:], ".")
	zoneID, err := api.ZoneIDByName(nakedDomain)
	if err != nil {
		return "", fmt.Errorf("failed to get zone id: %w", err)
	}

	return zoneID, nil
}

func externalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}
