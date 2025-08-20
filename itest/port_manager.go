//go:build e2e
// +build e2e

package e2etest

import (
	"fmt"
	"net"
	"sync"
)

// PortManager manages port allocation for tests to avoid conflicts
type PortManager struct {
	MinPort   int
	MaxPort   int
	UsedPorts map[int]bool
	Mutex     sync.Mutex
}

// NewPortManager creates a new port manager with a safe range for testing
func NewPortManager() *PortManager {
	return &PortManager{
		MinPort:   9800,  // Start from a high port range
		MaxPort:   10800, // Allow 1000 ports
		UsedPorts: make(map[int]bool),
	}
}

// isPortAvailable checks if a port is actually available on the system
func (pm *PortManager) isPortAvailable(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// AllocatePort allocates a single available port
func (pm *PortManager) AllocatePort() (int, error) {
	pm.Mutex.Lock()
	defer pm.Mutex.Unlock()

	for port := pm.MinPort; port <= pm.MaxPort; port++ {
		if !pm.UsedPorts[port] && pm.isPortAvailable(port) {
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
}
