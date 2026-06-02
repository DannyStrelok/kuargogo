package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

type MDNSService struct {
	Instance string
	HostName string
	IPs      []string
	Port     int
	TXT      []string
}

// DiscoverMDNS performs a mDNS query for services of the given type (e.g. "_rk._tcp").
// It returns a slice of MDNSService with discovered instances.
func DiscoverMDNS(serviceType string, timeout time.Duration) ([]MDNSService, error) {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create zeroconf resolver: %w", err)
	}
	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	
	var results []MDNSService

	err = resolver.Browse(ctx, serviceType, "local.", entries)
	if err != nil {
		return nil, fmt.Errorf("zeroconf browse error: %w", err)
	}

	for entry := range entries {
		ips := make([]string, len(entry.AddrIPv4))
		for i, ip := range entry.AddrIPv4 {
			ips[i] = ip.String()
		}
		results = append(results, MDNSService{
			Instance: entry.Instance,
			HostName: entry.HostName,
			IPs:      ips,
			Port:     entry.Port,
			TXT:      entry.Text,
		})
	}

	return results, nil
}

// ScanCommonServices looks for standard SSH and workstations in parallel
func ScanCommonServices(timeout time.Duration) ([]MDNSService, error) {
	serviceTypes := []string{"_ssh._tcp", "_workstation._tcp", "_rk._tcp"}
	var results []MDNSService
	seen := make(map[string]bool)

	resultsChan := make(chan []MDNSService, len(serviceTypes))
	var wg sync.WaitGroup

	for _, svc := range serviceTypes {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			// Use a slightly shorter timeout for probes to ensure they return
			// before the main collector times out.
			res, _ := DiscoverMDNS(s, timeout-200*time.Millisecond)
			resultsChan <- res
		}(svc)
	}

	// Close channel when all probes are done
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	// We wait up to the full timeout
	timer := time.NewTimer(timeout)
	defer timer.Stop()

Loop:
	for i := 0; i < len(serviceTypes); i++ {
		select {
		case res, ok := <-resultsChan:
			if !ok {
				break Loop
			}
			for _, r := range res {
				key := r.HostName
				if len(r.IPs) > 0 {
					key = r.IPs[0]
				}

				if !seen[key] {
					results = append(results, r)
					seen[key] = true
				}
			}
		case <-timer.C:
			break Loop
		}
	}

	return results, nil
}
