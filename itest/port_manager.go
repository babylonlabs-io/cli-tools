//go:build e2e
// +build e2e

package e2etest

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// PortManager manages port allocation for tests to avoid conflicts
type PortManager struct {
	MinPort        int
	MaxPort        int
	UsedPorts      map[int]bool
	RecentlyUsed   map[int]time.Time // Track recently released ports to avoid reuse
	Mutex          sync.Mutex
}

// NewPortManager creates a new port manager with a safe range for testing
func NewPortManager() *PortManager {
	return &PortManager{
		MinPort:      9800,  // Start from a high port range
		MaxPort:      10800, // Allow 1000 ports
		UsedPorts:    make(map[int]bool),
		RecentlyUsed: make(map[int]time.Time),
	}
}

// isPortAvailable checks if a port is actually available on the system
func (pm *PortManager) isPortAvailable(port int) bool {
	// Try to listen on the port
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	
	// Additional check: try to bind again after a small delay to catch TIME_WAIT issues
	time.Sleep(10 * time.Millisecond)
	ln2, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln2.Close()
	
	return true
}

// isRecentlyUsed checks if a port was released recently (within 5 seconds)
func (pm *PortManager) isRecentlyUsed(port int) bool {
	if releaseTime, exists := pm.RecentlyUsed[port]; exists {
		return time.Since(releaseTime) < 5*time.Second
	}
	return false
}

// AllocatePort allocates a single available port
func (pm *PortManager) AllocatePort() (int, error) {
	pm.Mutex.Lock()
	defer pm.Mutex.Unlock()

	// Clean up old recently used entries
	cutoff := time.Now().Add(-5 * time.Second)
	for port, releaseTime := range pm.RecentlyUsed {
		if releaseTime.Before(cutoff) {
			delete(pm.RecentlyUsed, port)
		}
	}

	for port := pm.MinPort; port <= pm.MaxPort; port++ {
		if !pm.UsedPorts[port] && !pm.isRecentlyUsed(port) && pm.isPortAvailable(port) {
			pm.UsedPorts[port] = true
			return port, nil
		}
	}

	return 0, fmt.Errorf("no available ports in range %d-%d", pm.MinPort, pm.MaxPort)
}

// ReleasePort releases a port back to the pool
func (pm *PortManager) ReleasePort(port int) {
	pm.Mutex.Lock()
	defer pm.Mutex.Unlock()
	delete(pm.UsedPorts, port)
	// Mark as recently used to avoid immediate reuse
	pm.RecentlyUsed[port] = time.Now()
}
