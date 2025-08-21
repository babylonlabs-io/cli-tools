//go:build e2e
// +build e2e

package e2etest

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

// PortManager manages port allocation for tests to avoid conflicts
type PortManager struct {
	MinPort      int
	MaxPort      int
	UsedPorts    map[int]bool
	RecentlyUsed map[int]time.Time // Track recently released ports to avoid reuse
	Mutex        sync.Mutex
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
	// Try to listen on the port multiple times with retries
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return false
		}
		ln.Close()

		// Wait a bit to ensure the port is properly released
		time.Sleep(20 * time.Millisecond)
	}

	return true
}

// isRecentlyUsed checks if a port was released recently (within 10 seconds)
func (pm *PortManager) isRecentlyUsed(port int) bool {
	if releaseTime, exists := pm.RecentlyUsed[port]; exists {
		return time.Since(releaseTime) < 10*time.Second
	}
	return false
}

// AllocatePort allocates a single available port using randomized search
func (pm *PortManager) AllocatePort() (int, error) {
	pm.Mutex.Lock()
	defer pm.Mutex.Unlock()

	// Clean up old recently used entries
	cutoff := time.Now().Add(-10 * time.Second)
	for port, releaseTime := range pm.RecentlyUsed {
		if releaseTime.Before(cutoff) {
			delete(pm.RecentlyUsed, port)
		}
	}

	// Try randomized port allocation to reduce collisions
	attempts := 0
	maxAttempts := (pm.MaxPort - pm.MinPort + 1) * 2 // Allow 2x range attempts

	for attempts < maxAttempts {
		attempts++

		// Generate random port in range
		port := pm.MinPort + rand.Intn(pm.MaxPort-pm.MinPort+1)

		if !pm.UsedPorts[port] && pm.isPortAvailable(port) {
			pm.UsedPorts[port] = true
			return port, nil
		}

		// Add small random delay between attempts to reduce race conditions
		time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
	}

	return 0, fmt.Errorf("no available ports in range %d-%d after %d attempts", pm.MinPort, pm.MaxPort, maxAttempts)
}

// ReleasePort releases a port back to the pool
func (pm *PortManager) ReleasePort(port int) {
	pm.Mutex.Lock()
	defer pm.Mutex.Unlock()
	delete(pm.UsedPorts, port)
	// Mark as recently used to avoid immediate reuse
	pm.RecentlyUsed[port] = time.Now()
}
