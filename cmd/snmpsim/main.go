package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/debashish-mukherjee/go-snmpsim/internal/api"
	"github.com/debashish-mukherjee/go-snmpsim/internal/engine"
	"github.com/debashish-mukherjee/go-snmpsim/internal/traps"
	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
	"github.com/debashish-mukherjee/go-snmpsim/internal/webui"
)

type stringSliceFlag []string

func (f *stringSliceFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *stringSliceFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		item := strings.TrimSpace(part)
		if item != "" {
			*f = append(*f, item)
		}
	}
	return nil
}

func main() {
	// Configuration flags
	portStart := flag.Int("port-start", 20000, "Starting port for UDP listeners")
	portEnd := flag.Int("port-end", 30000, "Ending port for UDP listeners")
	devices := flag.Int("devices", 100, "Number of virtual devices to simulate")
	snmprecFile := flag.String("snmprec", "", "Path to .snmprec file for OID templates")
	routeFile := flag.String("route-file", "", "Path to routes.yaml for dataset routing")
	variationFile := flag.String("variation-file", "", "Path to variations.yaml for OID variation chains")
	listenAddr := flag.String("listen", "0.0.0.0", "Listen address")
	v3Enabled := flag.Bool("v3-enabled", true, "Enable SNMPv3 support")
	engineID := flag.String("engine-id", "", "SNMPv3 authoritative engine ID (hex or plain text)")
	v3User := flag.String("v3-user", "simuser", "SNMPv3 username")
	legacyV3User := flag.String("snmpv3-user", "", "Deprecated alias of --v3-user")
	v3Auth := flag.String("v3-auth", "", "SNMPv3 auth protocol: MD5,SHA1,SHA224,SHA256,SHA384,SHA512")
	v3AuthKey := flag.String("v3-auth-key", "", "SNMPv3 auth passphrase")
	v3Priv := flag.String("v3-priv", "", "SNMPv3 priv protocol: DES,3DES,AES128,AES192,AES256")
	v3PrivKey := flag.String("v3-priv-key", "", "SNMPv3 privacy passphrase")
	trapVersion := flag.String("trap-version", "v2c", "Trap/Inform version: v2c|v3")
	trapCommunity := flag.String("trap-community", "public", "Trap community for v2c notifications")
	trapOnVariation := flag.Bool("trap-on-variation", false, "Emit traps on variation events")
	trapInform := flag.Bool("trap-inform", false, "Emit informs instead of traps")
	webPort := flag.String("web-port", "8080", "Port for web UI API server")

	var trapTargets stringSliceFlag
	var trapCronSpecs stringSliceFlag
	var trapSetOIDs stringSliceFlag
	flag.Var(&trapTargets, "trap-target", "Trap target host:port (repeatable)")
	flag.Var(&trapCronSpecs, "trap-cron", "Cron spec for periodic trap emission (repeatable)")
	flag.Var(&trapSetOIDs, "trap-on-set-oid", "Emit trap on SET to OID (repeatable)")
	flag.Parse()

	// Check file descriptors
	checkFileDescriptors(*portEnd - *portStart)

	if *legacyV3User != "" {
		*v3User = *legacyV3User
	}

	parsedEngineID, err := v3.ParseEngineID(*engineID)
	if err != nil {
		log.Fatalf("Invalid engine ID: %v", err)
	}

	v3Config := v3.Config{
		Enabled:  *v3Enabled,
		EngineID: parsedEngineID,
		Username: *v3User,
		Auth:     v3.AuthProtocol(strings.ToUpper(*v3Auth)),
		AuthKey:  *v3AuthKey,
		Priv:     v3.PrivProtocol(strings.ToUpper(*v3Priv)),
		PrivKey:  *v3PrivKey,
	}

	if v3Config.Enabled {
		if err := v3Config.Validate(); err != nil {
			log.Fatalf("Invalid SNMPv3 config: %v", err)
		}
	}

	log.Printf("Starting SNMP Simulator")
	log.Printf("SNMP Port range: %d-%d", *portStart, *portEnd)
	log.Printf("Number of devices: %d", *devices)
	if v3Config.Enabled {
		log.Printf("SNMPv3 enabled: user=%s auth=%s priv=%s", v3Config.Username, v3Config.Auth, v3Config.Priv)
	} else {
		log.Printf("SNMPv3 enabled: false")
	}
	log.Printf("Web UI port: %s (http://localhost:%s)", *webPort, *webPort)

	// Create simulator
	simulator, err := engine.NewSimulator(
		*listenAddr,
		*portStart,
		*portEnd,
		*devices,
		*snmprecFile,
		*routeFile,
		*variationFile,
		v3Config,
	)
	if err != nil {
		log.Fatalf("Failed to create simulator: %v", err)
	}

	if len(trapTargets) > 0 {
		trapConfig := traps.Config{
			Targets:     trapTargets,
			Version:     *trapVersion,
			Community:   *trapCommunity,
			V3User:      *v3User,
			V3Auth:      *v3Auth,
			V3AuthKey:   *v3AuthKey,
			V3Priv:      *v3Priv,
			V3PrivKey:   *v3PrivKey,
			CronSpecs:   trapCronSpecs,
			OnVariation: *trapOnVariation,
			OnSetOIDs:   trapSetOIDs,
			Inform:      *trapInform,
		}
		if err := simulator.SetTrapConfig(trapConfig); err != nil {
			log.Fatalf("Invalid trap config: %v", err)
		}
		log.Printf("Trap emission enabled: targets=%d version=%s", len(trapTargets), *trapVersion)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize workload manager
	workloadManager := webui.NewWorkloadManager("config/workloads")

	// Create API server
	apiServer := api.NewServer(":" + *webPort)
	apiServer.SetSimulator(simulator)
	apiServer.SetSimulatorStatus(*portStart, *portEnd, *devices, *listenAddr, "just-started")
	apiServer.SetWorkloadManager(workloadManager)
	apiServer.SetSNMPTester(webui.NewSNMPTester())

	// Start API server in goroutine
	go func() {
		log.Printf("Starting web UI server on http://localhost:%s", *webPort)
		if err := apiServer.Start(); err != nil {
			log.Printf("Warning: Web UI server error: %v", err)
		}
	}()

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
	apiServer.Stop()
	simulator.Stop()
	log.Printf("Graceful shutdown complete")
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
