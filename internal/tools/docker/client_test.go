package docker

import (
	"testing"

	"github.com/docker/go-connections/nat"
)

// TestPortParsing tests the port parsing logic to ensure it handles various formats correctly.
func TestPortParsing(t *testing.T) {
	testCases := []struct {
		name             string
		containerPort    string
		expectedPort     string
		expectedProtocol string
		shouldError      bool
	}{
		{
			name:             "Port with TCP protocol",
			containerPort:    "53/tcp",
			expectedPort:     "53",
			expectedProtocol: "tcp",
			shouldError:      false,
		},
		{
			name:             "Port with UDP protocol",
			containerPort:    "53/udp",
			expectedPort:     "53",
			expectedProtocol: "udp",
			shouldError:      false,
		},
		{
			name:             "Port without protocol (defaults to TCP)",
			containerPort:    "80",
			expectedPort:     "80",
			expectedProtocol: "tcp",
			shouldError:      false,
		},
		{
			name:             "HTTPS port with protocol",
			containerPort:    "443/tcp",
			expectedPort:     "443",
			expectedProtocol: "tcp",
			shouldError:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Parse container port - it may be "53" or "53/tcp"
			portNum := tc.containerPort
			protocol := "tcp"

			// Check if protocol is specified (e.g., "53/tcp" or "53/udp")
			for i, c := range tc.containerPort {
				if c == '/' {
					portNum = tc.containerPort[:i]
					protocol = tc.containerPort[i+1:]
					break
				}
			}

			// Verify the parsing
			if portNum != tc.expectedPort {
				t.Errorf("Expected port %s, got %s", tc.expectedPort, portNum)
			}
			if protocol != tc.expectedProtocol {
				t.Errorf("Expected protocol %s, got %s", tc.expectedProtocol, protocol)
			}

			// Test with nat.NewPort to ensure it works
			port, err := nat.NewPort(protocol, portNum)
			if tc.shouldError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tc.shouldError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if !tc.shouldError && err == nil {
				expectedPortString := portNum + "/" + protocol
				if string(port) != expectedPortString {
					t.Errorf("Expected port string %s, got %s", expectedPortString, string(port))
				}
			}
		})
	}
}

// TestContainerConfigPorts tests the full port configuration with various formats.
func TestContainerConfigPorts(t *testing.T) {
	testCases := []struct {
		name        string
		ports       map[string]string
		expectError bool
	}{
		{
			name: "Single TCP port with protocol",
			ports: map[string]string{
				"8080/tcp": "80/tcp",
			},
			expectError: false,
		},
		{
			name: "Single UDP port with protocol",
			ports: map[string]string{
				"5353/udp": "53/udp",
			},
			expectError: false,
		},
		{
			name: "Mixed TCP and UDP ports",
			ports: map[string]string{
				"30053/tcp": "53/tcp",
				"30053/udp": "53/udp",
			},
			expectError: false,
		},
		{
			name: "Ports without protocol (defaults to TCP)",
			ports: map[string]string{
				"8080": "80",
			},
			expectError: false,
		},
		{
			name: "Multiple HTTP/HTTPS ports",
			ports: map[string]string{
				"80/tcp":  "80/tcp",
				"443/tcp": "443/tcp",
			},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the port parsing logic from CreateAndStart
			portBindings := nat.PortMap{}
			exposedPorts := nat.PortSet{}

			for hostPort, containerPort := range tc.ports {
				// Parse container port - it may be "53" or "53/tcp"
				portNum := containerPort
				protocol := "tcp"

				// Check if protocol is specified (e.g., "53/tcp" or "53/udp")
				for i, c := range containerPort {
					if c == '/' {
						portNum = containerPort[:i]
						protocol = containerPort[i+1:]
						break
					}
				}

				port, err := nat.NewPort(protocol, portNum)
				if tc.expectError && err == nil {
					t.Error("Expected error but got none")
					return
				}
				if !tc.expectError && err != nil {
					t.Errorf("Unexpected error: %v", err)
					return
				}

				if err == nil {
					exposedPorts[port] = struct{}{}

					// Parse host port to extract just the port number
					hostPortNum := hostPort
					for i, c := range hostPort {
						if c == '/' {
							hostPortNum = hostPort[:i]
							break
						}
					}

					portBindings[port] = []nat.PortBinding{
						{
							HostIP:   "0.0.0.0",
							HostPort: hostPortNum,
						},
					}
				}
			}

			// Verify we got the expected number of ports
			if !tc.expectError {
				if len(exposedPorts) != len(tc.ports) {
					t.Errorf("Expected %d exposed ports, got %d", len(tc.ports), len(exposedPorts))
				}
				if len(portBindings) != len(tc.ports) {
					t.Errorf("Expected %d port bindings, got %d", len(tc.ports), len(portBindings))
				}
			}
		})
	}
}
