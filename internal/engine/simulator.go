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
	"github.com/debashish-mukherjee/go-snmpsim/internal/routing"
	"github.com/debashish-mukherjee/go-snmpsim/internal/store"
	"github.com/debashish-mukherjee/go-snmpsim/internal/traps"
	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
	"github.com/debashish-mukherjee/go-snmpsim/internal/variation"
	"golang.org/x/sys/unix"
)

// Simulator manages multiple UDP listeners for virtual SNMP agents
type Simulator struct {
	listenAddr    string
	listenAddr6   string
	portStart     int
	portEnd       int
	numDevices    int
	snmprecFile   string
	routeFile     string
	variationFile string
	v3Config      v3.Config
	v3State       *v3.EngineStateStore
	router        *routing.Router
	datasetStore  *store.DatasetStore
	variations    *variation.Binder
	trapManager   *traps.Manager

	// Listeners and dispatcher
	listeners    map[string]*net.UDPConn     // key -> listener
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
func NewSimulator(listenAddr string, portStart, portEnd, numDevices int, snmprecFile string, routeFile string, variationFile string, v3Config v3.Config) (*Simulator, error) {
	if portStart >= portEnd {
		return nil, fmt.Errorf("portStart must be less than portEnd")
	}

	if numDevices <= 0 {
		return nil, fmt.Errorf("numDevices must be positive")
	}

	if v3Config.Enabled {
		if err := v3Config.Validate(); err != nil {
			return nil, fmt.Errorf("invalid snmpv3 configuration: %w", err)
		}
	}

	v3State, err := v3.NewEngineStateStore("")
	if err != nil {
		return nil, fmt.Errorf("failed to initialize v3 state: %w", err)
	}

	sim := &Simulator{
		listenAddr:    listenAddr,
		portStart:     portStart,
		portEnd:       portEnd,
		numDevices:    numDevices,
		snmprecFile:   snmprecFile,
		routeFile:     routeFile,
		variationFile: variationFile,
		v3Config:      v3Config,
		v3State:       v3State,
		listeners:     make(map[string]*net.UDPConn),
		agents:        make(map[int]*agent.VirtualAgent),
		packetPool: &sync.Pool{
			New: func() interface{} {
				return make([]byte, 4096)
			},
		},
	}

	// Initialize dispatcher
	sim.dispatcher = NewPacketDispatcher(sim.packetPool)

	var routeEngine *routing.Router
	if routeFile != "" {
		routeEngine, err = routing.LoadFromFile(routeFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load route file: %w", err)
		}
	}

	if variationFile != "" {
		sim.variations, err = variation.LoadBinder(variationFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load variation file: %w", err)
		}
	}

	extraDatasetPaths := []string{}
	if routeEngine != nil {
		extraDatasetPaths = routeEngine.DatasetPaths()
	}

	datasetStore, err := store.NewDatasetStore(snmprecFile, extraDatasetPaths)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize dataset store: %w", err)
	}
	sim.router = routeEngine
	sim.datasetStore = datasetStore

	oidDB, _ := datasetStore.Resolve("")
	if oidDB == nil {
		return nil, fmt.Errorf("default dataset could not be resolved")
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

// SetListenAddr6 configures optional IPv6 UDP listener address (e.g. :: or ::1).
func (s *Simulator) SetListenAddr6(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listenAddr6 = addr
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
		cfg := s.v3Config
		boots := uint32(1)
		if cfg.Enabled {
			if cfg.EngineID == "" {
				cfg.EngineID = v3.GenerateEngineID(fmt.Sprintf("device-%d-port-%d", deviceID, port))
			}
			persistedBoots, err := s.v3State.EnsureBoots(cfg.EngineID)
			if err != nil {
				return fmt.Errorf("failed to persist v3 engine boots: %w", err)
			}
			boots = persistedBoots
		}

		virtualAgent := agent.NewVirtualAgent(
			deviceID,
			port,
			fmt.Sprintf("Device-%d", deviceID),
			oidDB,
			cfg,
			boots,
		)

		// Assign index manager for Zabbix LLD support
		if s.indexManager != nil {
			virtualAgent.SetIndexManager(s.indexManager)
		}
		virtualAgent.SetRouting(s.router, s.datasetStore)
		virtualAgent.SetVariationBinder(s.variations)
		if s.trapManager != nil {
			virtualAgent.SetVariationEventHook(func(ev agent.VariationEvent) {
				s.trapManager.EnqueueVariationEvent(ev.DeviceID, ev.Port, ev.OID, ev.Detail)
			})
			virtualAgent.SetSetEventHook(func(ev agent.SetEvent) {
				s.trapManager.EnqueueSetEvent(ev.DeviceID, ev.Port, ev.OID, ev.Type, ev.Value)
			})
		}

		s.agents[port] = virtualAgent
		deviceID++

		if deviceID >= s.numDevices {
			break
		}
	}

	log.Printf("Created %d virtual agents across ports %d-%d",
		len(s.agents), s.portStart, s.portStart+len(s.agents)-1)

	return nil
}

func (s *Simulator) SetTrapConfig(cfg traps.Config) error {
	manager, err := traps.NewManager(cfg)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.trapManager = manager
	for _, vAgent := range s.agents {
		if manager == nil {
			vAgent.SetVariationEventHook(nil)
			vAgent.SetSetEventHook(nil)
			continue
		}
		vAgent.SetVariationEventHook(func(ev agent.VariationEvent) {
			manager.EnqueueVariationEvent(ev.DeviceID, ev.Port, ev.OID, ev.Detail)
		})
		vAgent.SetSetEventHook(func(ev agent.SetEvent) {
			manager.EnqueueSetEvent(ev.DeviceID, ev.Port, ev.OID, ev.Type, ev.Value)
		})
	}
	return nil
}

// Start initializes all UDP listeners and starts packet handling
func (s *Simulator) Start(ctx context.Context) error {
	if !s.running.CompareAndSwap(false, true) {
		return fmt.Errorf("simulator already running")
	}

	s.mu.Lock()
	if s.trapManager != nil {
		s.trapManager.Start()
	}

	// Create UDP listeners with SO_REUSEADDR/SO_REUSEPORT
	for port := range s.agents {
		if err := s.startListener(ctx, "udp", s.listenAddr, port, "ipv4"); err != nil {
			s.mu.Unlock()
			s.cleanup()
			return err
		}
		if s.listenAddr6 != "" {
			if err := s.startListener(ctx, "udp6", s.listenAddr6, port, "ipv6"); err != nil {
				s.mu.Unlock()
				s.cleanup()
				return err
			}
		}
	}

	s.mu.Unlock()

	log.Printf("Started %d UDP listeners", len(s.listeners))
	return nil
}

func (s *Simulator) startListener(ctx context.Context, network, listenAddr string, port int, family string) error {
	addr := net.UDPAddr{Port: port, IP: net.ParseIP(listenAddr)}
	conn, err := net.ListenUDP(network, &addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s port %d: %w", family, port, err)
	}
	if err := setSocketOptions(conn); err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to set socket options on %s port %d: %w", family, port, err)
	}
	key := fmt.Sprintf("%s:%d", family, port)
	s.listeners[key] = conn
	s.wg.Add(1)
	go s.handleListener(ctx, conn, port)
	return nil
}

// handleListener handles incoming packets on a specific port
func (s *Simulator) handleListener(ctx context.Context, conn *net.UDPConn, port int) {
	defer s.wg.Done()

	agent := s.agents[port]

	for {
		select {
		case <-ctx.Done():
			log.Printf("Closing listener on port %d", port)
			return
		default:
		}

		// Get buffer from pool for this packet
		buffer := s.packetPool.Get().([]byte)

		// Set read deadline to allow graceful shutdown
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))

		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			s.packetPool.Put(buffer) // Return buffer on error
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			if s.running.Load() {
				log.Printf("Error reading from port %d: %v", port, err)
			}
			continue
		}

		// Dispatch packet to agent
		response := agent.HandlePacketFrom(buffer[:n], remoteAddr, port)
		s.packetPool.Put(buffer) // Return buffer after processing

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
	if s.trapManager != nil {
		s.trapManager.Stop()
	}

	log.Printf("All listeners stopped")
}

func (s *Simulator) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for key, conn := range s.listeners {
		// Set a past deadline to unblock any pending ReadFromUDP calls
		// before closing the connection.
		conn.SetDeadline(time.Now())
		if err := conn.Close(); err != nil {
			log.Printf("Error closing listener %s: %v", key, err)
		}
	}
	s.listeners = make(map[string]*net.UDPConn)
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
	// Use SyscallConn to access the raw socket FD without affecting the
	// non-blocking state of the connection (conn.File() would set blocking mode
	// which breaks deadline-based shutdown).
	rawConn, err := conn.SyscallConn()
	if err != nil {
		return fmt.Errorf("failed to get raw conn: %w", err)
	}

	var setsockoptErr error
	err = rawConn.Control(func(fd uintptr) {
		ifd := int(fd)

		// Set SO_RCVBUF to prevent packet loss during burst traffic
		// 256KB buffer should be sufficient for most scenarios
		if err := syscall.SetsockoptInt(ifd, syscall.SOL_SOCKET, syscall.SO_RCVBUF, 256*1024); err != nil {
			setsockoptErr = fmt.Errorf("failed to set SO_RCVBUF: %w", err)
			return
		}

		// Set SO_SNDBUF for transmission
		if err := syscall.SetsockoptInt(ifd, syscall.SOL_SOCKET, syscall.SO_SNDBUF, 256*1024); err != nil {
			setsockoptErr = fmt.Errorf("failed to set SO_SNDBUF: %w", err)
			return
		}

		// Try to enable SO_REUSEPORT if available (Linux 3.9+)
		if err := syscall.SetsockoptInt(ifd, syscall.SOL_SOCKET, int(unix.SO_REUSEPORT), 1); err != nil {
			log.Printf("Warning: SO_REUSEPORT not available (may reduce performance): %v", err)
		}
	})
	if err != nil {
		return fmt.Errorf("rawConn.Control failed: %w", err)
	}
	return setsockoptErr
}
