package containers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/require"
)

var errRegex = regexp.MustCompile(`(E|e)rror`)

// PortManagerInterface defines the interface for port management
type PortManagerInterface interface {
	AllocatePort() (int, error)
	ReleasePort(port int)
}

// generateTestHash creates a short hash from the test name for unique container naming
func generateTestHash(testName string) string {
	hash := sha256.Sum256([]byte(testName))
	// Use first 8 characters of hex for readability and add timestamp for extra uniqueness in CI
	hashStr := hex.EncodeToString(hash[:])[:8]
	// Add timestamp suffix for CI environments where tests might run in rapid succession
	timestamp := fmt.Sprintf("%d", time.Now().UnixNano()%100000)
	return fmt.Sprintf("%s-%s", hashStr, timestamp)
}

// Manager is a wrapper around all Docker instances, and the Docker API.
// It provides utilities to run and interact with all Docker containers used within e2e testing.
type Manager struct {
	cfg                   ImageConfig
	pool                  *dockertest.Pool
	resources             map[string]*dockertest.Resource
	bitcoindContainerName string
	mongoContainerName    string
	bitcoindHost          string // Store the dynamically assigned RPC port
	allocatedPort         int    // Store allocated port for cleanup
	portManager           PortManagerInterface // Port manager for dynamic allocation
}

// NewManager creates a new Manager instance and initializes
// all Docker specific utilities. Returns an error if initialization fails.
func NewManager(t *testing.T, pm PortManagerInterface) (docker *Manager, err error) {
	// Generate unique container names based on test name to avoid conflicts between tests
	testHash := generateTestHash(t.Name())
	docker = &Manager{
		cfg:                   NewImageConfig(),
		resources:             make(map[string]*dockertest.Resource),
		bitcoindContainerName: fmt.Sprintf("bitcoind-test-%s", testHash),
		mongoContainerName:    fmt.Sprintf("mongo-test-%s", testHash),
		portManager:           pm,
	}
	docker.pool, err = dockertest.NewPool("")
	if err != nil {
		return nil, err
	}
	return docker, nil
}

func (m *Manager) ExecBitcoindCliCmd(t *testing.T, command []string) (bytes.Buffer, bytes.Buffer, error) {
	// this is currently hardcoded, as it will be the same for all tests
	cmd := []string{"bitcoin-cli", "-chain=regtest", "-rpcuser=user", "-rpcpassword=pass"}
	cmd = append(cmd, command...)
	return m.ExecCmd(t, m.bitcoindContainerName, cmd)
}

// ExecCmd executes command by running it on the given container.
// It word for word `error` in output to discern between error and regular output.
// It retures stdout and stderr as bytes.Buffer and an error if the command fails.
func (m *Manager) ExecCmd(t *testing.T, containerName string, command []string) (bytes.Buffer, bytes.Buffer, error) {
	if _, ok := m.resources[containerName]; !ok {
		return bytes.Buffer{}, bytes.Buffer{}, fmt.Errorf("no resource %s found", containerName)
	}
	containerId := m.resources[containerName].Container.ID

	var (
		outBuf bytes.Buffer
		errBuf bytes.Buffer
	)

	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	t.Logf("\n\nRunning: \"%s\"", command)

	// We use the `require.Eventually` function because it is only allowed to do one transaction per block without
	// sequence numbers. For simplicity, we avoid keeping track of the sequence number and just use the `require.Eventually`.
	require.Eventually(
		t,
		func() bool {
			exec, err := m.pool.Client.CreateExec(docker.CreateExecOptions{
				Context:      ctx,
				AttachStdout: true,
				AttachStderr: true,
				Container:    containerId,
				User:         "root",
				Cmd:          command,
			})

			if err != nil {
				t.Logf("failed to create exec: %v", err)
				return false
			}

			err = m.pool.Client.StartExec(exec.ID, docker.StartExecOptions{
				Context:      ctx,
				Detach:       false,
				OutputStream: &outBuf,
				ErrorStream:  &errBuf,
			})
			if err != nil {
				t.Logf("failed to start exec: %v", err)
				return false
			}

			errBufString := errBuf.String()
			// Note that this does not match all errors.
			// This only works if CLI outputs "Error" or "error"
			// to stderr.
			if errRegex.MatchString(errBufString) {
				t.Log("\nstderr:")
				t.Log(errBufString)

				t.Log("\nstdout:")
				t.Log(outBuf.String())
				return false
			}

			return true
		},
		timeout,
		500*time.Millisecond,
		"command failed",
	)

	return outBuf, errBuf, nil
}

func (m *Manager) RunBitcoindResource(
	bitcoindCfgPath string,
) (*dockertest.Resource, error) {
	// Pre-allocate a port using our port manager to avoid CI conflicts
	rpcPort, err := m.portManager.AllocatePort()
	if err != nil {
		return nil, fmt.Errorf("failed to allocate port for bitcoind: %w", err)
	}

	// Store the allocated port immediately
	m.bitcoindHost = fmt.Sprintf("127.0.0.1:%d", rpcPort)
	m.allocatedPort = rpcPort

	bitcoindResource, err := m.pool.RunWithOptions(
		&dockertest.RunOptions{
			Name:       m.bitcoindContainerName,
			Repository: m.cfg.BitcoindRepository,
			Tag:        m.cfg.BitcoindVersion,
			User:       "root:root",
			Mounts: []string{
				fmt.Sprintf("%s/:/data/.bitcoin", bitcoindCfgPath),
			},
			ExposedPorts: []string{
				"18443",
			},
			PortBindings: map[docker.Port][]docker.PortBinding{
				"18443/tcp": {{HostIP: "", HostPort: fmt.Sprintf("%d", rpcPort)}},
			},
			Cmd: []string{
				"-regtest",
				"-txindex",
				"-rpcuser=user",
				"-rpcpassword=pass",
				"-rpcallowip=0.0.0.0/0",
				"-rpcbind=0.0.0.0",
			},
		},
		dockerConf,
	)
	if err != nil {
		// Release the port if container creation failed
		m.portManager.ReleasePort(rpcPort)
		return nil, err
	}
	m.resources[m.bitcoindContainerName] = bitcoindResource
	return bitcoindResource, nil
}

// ClearResources removes all outstanding Docker resources created by the Manager.
func (m *Manager) ClearResources() error {
	for _, resource := range m.resources {
		if err := m.pool.Purge(resource); err != nil {
			// In CI environments, containers might already be gone, so ignore "not found" errors
			if !isNotFoundError(err) {
				return err
			}
		}
	}

	// Release the allocated port
	if m.allocatedPort != 0 {
		m.portManager.ReleasePort(m.allocatedPort)
		m.allocatedPort = 0
	}

	return nil
}

// isNotFoundError checks if the error is due to container not being found
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "No such container") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "404")
}

func dockerConf(config *docker.HostConfig) {
	// in this case we don't want the nodes to restart on failure
	config.RestartPolicy = docker.RestartPolicy{
		Name: "no",
	}
	config.AutoRemove = true
}

func (m *Manager) RunMongoDbResource() (*dockertest.Resource, error) {
	resource, err := m.pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "mongo",
		Tag:        "7.0",
		ExposedPorts: []string{
			"27017",
		},
		Env: []string{
			"MONGO_INITDB_ROOT_USERNAME=root",
			"MONGO_INITDB_ROOT_PASSWORD=example",
		},
	},
		dockerConf,
	)
	if err != nil {
		return nil, err
	}

	m.resources[m.mongoContainerName] = resource
	return resource, nil
}

func (m *Manager) MongoHost() string {
	return m.resources[m.mongoContainerName].GetHostPort("27017/tcp")
}

func (m *Manager) BitcoindHost() string {
	return m.bitcoindHost
}
