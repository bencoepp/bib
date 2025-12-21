package discovery

import (
	"context"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
)

const (
	// BibMDNSService is the mDNS service name for bibd nodes
	BibMDNSService = "_bib._tcp"

	// BibMDNSDomain is the mDNS domain
	BibMDNSDomain = "local."
)

// discoverMDNS discovers bibd nodes using mDNS
func (d *Discoverer) discoverMDNS(ctx context.Context) ([]DiscoveredNode, error) {
	timeout := d.opts.MDNSTimeout
	if timeout == 0 {
		timeout = 3 * time.Second
	}

	// Create a context with timeout for mDNS discovery
	mdnsCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var nodes []DiscoveredNode
	var mu sync.Mutex
	entriesCh := make(chan *mdns.ServiceEntry, 10)

	// Start listening for entries
	go func() {
		for entry := range entriesCh {
			node := d.mdnsEntryToNode(mdnsCtx, entry)
			if node != nil {
				mu.Lock()
				nodes = append(nodes, *node)
				mu.Unlock()
			}
		}
	}()

	// Configure mDNS query parameters
	params := &mdns.QueryParam{
		Service:             BibMDNSService,
		Domain:              BibMDNSDomain,
		Timeout:             timeout,
		Entries:             entriesCh,
		WantUnicastResponse: true,
	}

	// Run the query
	if err := mdns.Query(params); err != nil {
		close(entriesCh)
		return nodes, fmt.Errorf("mDNS query failed: %w", err)
	}

	close(entriesCh)
	return nodes, nil
}

// mdnsEntryToNode converts an mDNS service entry to a DiscoveredNode
func (d *Discoverer) mdnsEntryToNode(ctx context.Context, entry *mdns.ServiceEntry) *DiscoveredNode {
	if entry == nil {
		return nil
	}

	// Determine the address
	var host string
	if entry.AddrV4 != nil {
		host = entry.AddrV4.String()
	} else if entry.AddrV6 != nil {
		host = fmt.Sprintf("[%s]", entry.AddrV6.String())
	} else if entry.Host != "" {
		host = entry.Host
	} else {
		return nil
	}

	address := fmt.Sprintf("%s:%d", host, entry.Port)

	// Measure latency if enabled
	var latency time.Duration
	if d.opts.MeasureLatency {
		if l, err := measureLatency(ctx, address, d.opts.LatencyTimeout); err == nil {
			latency = l
		}
	}

	// Parse TXT records for node info
	nodeInfo := parseMDNSTxtRecords(entry.InfoFields)

	return &DiscoveredNode{
		Address:      address,
		Method:       MethodMDNS,
		Latency:      latency,
		NodeInfo:     nodeInfo,
		DiscoveredAt: time.Now(),
	}
}

// parseMDNSTxtRecords parses mDNS TXT records into NodeInfo
func parseMDNSTxtRecords(fields []string) *NodeInfo {
	if len(fields) == 0 {
		return nil
	}

	info := &NodeInfo{}
	for _, field := range fields {
		// Parse key=value pairs
		for i := 0; i < len(field); i++ {
			if field[i] == '=' {
				key := field[:i]
				value := field[i+1:]
				switch key {
				case "name":
					info.Name = value
				case "version":
					info.Version = value
				case "peer_id":
					info.PeerID = value
				case "mode":
					info.Mode = value
				}
				break
			}
		}
	}

	// Only return if we got some info
	if info.Name == "" && info.Version == "" && info.PeerID == "" && info.Mode == "" {
		return nil
	}

	return info
}

// BrowseMDNS performs a one-time mDNS browse for bib services
// This is a convenience function for simple use cases
func BrowseMDNS(timeout time.Duration) ([]DiscoveredNode, error) {
	ctx := context.Background()
	d := New(DiscoveryOptions{
		EnableMDNS:     true,
		MDNSTimeout:    timeout,
		MeasureLatency: true,
		LatencyTimeout: 2 * time.Second,
	})
	return d.discoverMDNS(ctx)
}

// MDNSServiceInfo contains information for registering an mDNS service
type MDNSServiceInfo struct {
	Name string
	Port int
	Host string
	TXT  []string
}

// RegisterMDNSService registers a bibd node as an mDNS service
// Returns a function to deregister the service
func RegisterMDNSService(info MDNSServiceInfo) (func(), error) {
	// Get the host IP addresses
	host := info.Host
	if host == "" || host == "0.0.0.0" {
		host, _ = os.Hostname()
	}

	// Get local IPs
	ips, err := getLocalIPs()
	if err != nil {
		return nil, fmt.Errorf("failed to get local IPs: %w", err)
	}

	if len(ips) == 0 {
		return nil, fmt.Errorf("no local IP addresses found")
	}

	// Create the mDNS service
	service, err := mdns.NewMDNSService(
		info.Name,
		BibMDNSService,
		BibMDNSDomain,
		host,
		info.Port,
		ips,
		info.TXT,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create mDNS service: %w", err)
	}

	// Create the server
	server, err := mdns.NewServer(&mdns.Config{Zone: service})
	if err != nil {
		return nil, fmt.Errorf("failed to create mDNS server: %w", err)
	}

	// Return a function to stop the server
	return func() {
		server.Shutdown()
	}, nil
}

// getLocalIPs returns all non-loopback local IP addresses
func getLocalIPs() ([]net.IP, error) {
	var ips []net.IP

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
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

			if ip == nil || ip.IsLoopback() {
				continue
			}

			ips = append(ips, ip)
		}
	}

	return ips, nil
}
