package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/debashish-mukherjee/go-snmpsim/internal/engine"
)

func main() {
	// Configuration flags
	portStart := flag.Int("port-start", 20000, "Starting port for UDP listeners")
	portEnd := flag.Int("port-end", 30000, "Ending port for UDP listeners")
	devices := flag.Int("devices", 100, "Number of virtual devices to simulate")
	snmprecFile := flag.String("snmprec", "", "Path to .snmprec file for OID templates")
	listenAddr := flag.String("listen", "0.0.0.0", "Listen address")
	flag.Parse()

	// Check file descriptors
	checkFileDescriptors(*portEnd - *portStart)

	log.Printf("Starting SNMP Simulator")
	log.Printf("Port range: %d-%d", *portStart, *portEnd)
	log.Printf("Number of devices: %d", *devices)

	// Create simulator
	simulator, err := engine.NewSimulator(
		*listenAddr,
		*portStart,
		*portEnd,
		*devices,
		*snmprecFile,
	)
	if err != nil {
		log.Fatalf("Failed to create simulator: %v", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		cancel()
	}()

	// Start simulator
	if err := simulator.Start(ctx); err != nil {
		log.Fatalf("Failed to start simulator: %v", err)
	}
	log.Printf("Simulator started successfully")

	// Wait for shutdown signal
	<-ctx.Done()

	log.Printf("Shutting down...")
	simulator.Stop()
	log.Printf("Simulator stopped")
}

func checkFileDescriptors(requiredFDs int) {
	var rlimit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit)
	if err != nil {
		log.Printf("Warning: Could not check file descriptor limit: %v", err)
		return
	}

	// Each port needs 1 socket, plus some overhead for system operations
	requiredTotal := uint64(requiredFDs) + 100

	if rlimit.Cur < uint64(requiredTotal) {
		log.Printf("Warning: Current file descriptor limit (%d) may be insufficient for %d devices (%d required)",
			rlimit.Cur, requiredFDs, requiredTotal)
		log.Printf("Increase with: ulimit -n %d", requiredTotal*2)
	} else {
		log.Printf("File descriptor limit OK: %d (need ~%d)", rlimit.Cur, requiredTotal)
	}
}
