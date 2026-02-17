package engine

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/agent"
	"github.com/debashish-mukherjee/go-snmpsim/internal/store"
	"golang.org/x/sys/unix"
)

// Simulator manages multiple UDP listeners for virtual SNMP agents
type Simulator struct {
	listenAddr  string
	portStart   int
	portEnd     int
	numDevices  int
	snmprecFile string

	// Listeners and dispatcher
	listeners    map[int]*net.UDPConn        // port -> listener
	agents       map[int]*agent.VirtualAgent // port -> agent
	dispatcher   *PacketDispatcher
	indexManager *store.OIDIndexManager // Index manager for Zabbix LLD

	// Synchronization
	mu      sync.RWMutex
	wg      sync.WaitGroup
	running atomic.Bool

	// Performance
	packetPool *sync.Pool
}

// NewSimulator creates a new SNMP simulator instance
func NewSimulator(listenAddr string, portStart, portEnd, numDevices int, snmprecFile string) (*Simulator, error) {
	if portStart >= portEnd {
		return nil, fmt.Errorf("portStart must be less than portEnd")
	}

	if numDevices <= 0 {
		return nil, fmt.Errorf("numDevices must be positive")
	}

	sim := &Simulator{
		listenAddr:  listenAddr,
		portStart:   portStart,
		portEnd:     portEnd,
		numDevices:  numDevices,
		snmprecFile: snmprecFile,
		listeners:   make(map[int]*net.UDPConn),
		agents:      make(map[int]*agent.VirtualAgent),
		packetPool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 4096)
			},
		},
	}

	// Initialize dispatcher
	sim.dispatcher = NewPacketDispatcher(sim.packetPool)

	// Load OID database
	oidDB, err := store.LoadOIDDatabase(snmprecFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load OID database: %w", err)
	}

	// Create index manager for Zabbix LLD support
	indexManager := store.NewOIDIndexManager()
	if err := indexManager.BuildIndex(oidDB); err != nil {
		return nil, fmt.Errorf("failed to build OID index: %w", err)
	}
	sim.indexManager = indexManager

	// Create virtual agents
	if err := sim.createVirtualAgents(oidDB); err != nil {
		return nil, fmt.Errorf("failed to create virtual agents: %w", err)
	}

	return sim, nil
}

// createVirtualAgents creates virtual agents mapped to ports
func (s *Simulator) createVirtualAgents(oidDB *store.OIDDatabase) error {
	numPorts := s.portEnd - s.portStart

	// Distribute devices across available ports
	devicesPerPort := s.numDevices / numPorts
	if devicesPerPort == 0 {
		devicesPerPort = 1
	}

	deviceID := 0
	for port := s.portStart; port < s.portEnd && deviceID < s.numDevices; port++ {
		agent := agent.NewVirtualAgent(
			deviceID,
			port,
			fmt.Sprintf("Device-%d", deviceID),
			oidDB,
		)

		// Assign index manager for Zabbix LLD support
		if s.indexManager != nil {
			agent.SetIndexManager(s.indexManager)
		}

		s.agents[port] = agent
		deviceID++

		if deviceID >= s.numDevices {
			break
		}
	}

	log.Printf("Created %d virtual agents across ports %d-%d",
		len(s.agents), s.portStart, s.portStart+len(s.agents)-1)

	return nil
}

// Start initializes all UDP listeners and starts packet handling
func (s *Simulator) Start(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return fmt.Errorf("simulator already running")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Create UDP listeners with SO_REUSEADDR/SO_REUSEPORT
	for port := range s.agents {
		addr := net.UDPAddr{
			Port: port,
			IP:   net.ParseIP(s.listenAddr),
		}

		// Create listener
		conn, err := net.ListenUDP("udp", &addr)
		if err != nil {
			s.cleanup()
			return fmt.Errorf("failed to listen on port %d: %w", port, err)
		}

		// Set socket options for better performance
		if err := setSocketOptions(conn); err != nil {
			conn.Close()
			s.cleanup()
			return fmt.Errorf("failed to set socket options on port %d: %w", port, err)
		}

		s.listeners[port] = conn

		// Start listener goroutine
		s.wg.Add(1)
		go s.handleListener(ctx, conn, port)
	}

	log.Printf("Started %d UDP listeners", len(s.listeners))
	return nil
}

// handleListener handles incoming packets on a specific port
func (s *Simulator) handleListener(ctx context.Context, conn *net.UDPConn, port int) {
	defer s.wg.Done()

	agent := s.agents[port]
	buffer := s.packetPool.Get().([]byte)
	defer s.packetPool.Put(buffer)

	for {
		select {
		case <-ctx.Done():
			log.Printf("Closing listener on port %d", port)
			return
		default:
		}

		// Set read deadline to allow graceful shutdown
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if s.running.Load() {
				log.Printf("Error reading from port %d: %v", port, err)
			}
			continue
		}

		// Dispatch packet to agent
		response := agent.HandlePacket(buffer[:n])
		if response != nil {
			_, err := conn.WriteToUDP(response, remoteAddr)
			if err != nil {
				log.Printf("Error writing to port %d: %v", port, err)
			}
		}
	}
}

// Stop gracefully shuts down all listeners
func (s *Simulator) Stop() {
	if !s.running.CompareAndSwap(true, false) {
		return
	}

	s.cleanup()
	s.wg.Wait()

	log.Printf("All listeners stopped")
}

func (s *Simulator) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for port, conn := range s.listeners {
		if err := conn.Close(); err != nil {
			log.Printf("Error closing listener on port %d: %v", port, err)
		}
	}
	s.listeners = make(map[int]*net.UDPConn)
}

// Statistics returns current simulator statistics
func (s *Simulator) Statistics() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return map[string]interface{}{
		"running":          s.running.Load(),
		"active_listeners": len(s.listeners),
		"virtual_agents":   len(s.agents),
		"port_start":       s.portStart,
		"port_end":         s.portEnd,
	}
}

// setSocketOptions configures UDP socket for optimal performance
func setSocketOptions(conn *net.UDPConn) error {
	// Get the raw socket file descriptor
	file, err := conn.File()
	if err != nil {
		return err
	}
	defer file.Close()

	fd := int(file.Fd())

	// Set SO_RCVBUF to prevent packet loss during burst traffic
	// 256KB buffer should be sufficient for most scenarios
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 256*1024); err != nil {
		return fmt.Errorf("failed to set SO_RCVBUF: %w", err)
	}

	// Set SO_SNDBUF for transmission
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 256*1024); err != nil {
		return fmt.Errorf("failed to set SO_SNDBUF: %w", err)
	}

	// Try to enable SO_REUSEPORT if available (Linux 3.9+)
	// This allows multiple processes to bind to the same port
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, int(unix.SO_REUSEPORT), 1); err != nil {
		// This is not critical, just warn
		log.Printf("Warning: SO_REUSEPORT not available (may reduce performance): %v", err)
	}

	return nil
}
