package agent

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/store"
	"github.com/debashish-mukherjee/go-snmpsim/internal/v3"
	"github.com/gosnmp/gosnmp"
)

// VirtualAgent represents a single simulated SNMP agent
type VirtualAgent struct {
	deviceID      int
	port          int
	sysName       string
	v3Config      v3.Config
	v3EngineBoots uint32
	oidDB         *store.OIDDatabase
	indexManager  *store.OIDIndexManager  // Index manager for Zabbix LLD (table-aware)
	deviceMapping *store.DeviceOIDMapping // Device-specific OID overrides
	deviceOverlay map[string]interface{}  // Device-specific value overrides
	uptime        uint32
	startTime     time.Time
	lastPoll      time.Time
	pollCount     atomic.Int64

	mu sync.RWMutex
}

// NewVirtualAgent creates a new virtual SNMP agent
func NewVirtualAgent(deviceID int, port int, sysName string, oidDB *store.OIDDatabase, v3Config v3.Config, v3EngineBoots uint32) *VirtualAgent {
	if v3Config.Enabled && v3Config.Username == "" {
		v3Config.Username = "simuser"
	}
	if v3Config.Enabled && v3Config.EngineID == "" {
		v3Config.EngineID = v3.GenerateEngineID(fmt.Sprintf("device-%d", deviceID))
	}

	return &VirtualAgent{
		deviceID:      deviceID,
		port:          port,
		sysName:       sysName,
		v3Config:      v3Config,
		v3EngineBoots: v3EngineBoots,
		oidDB:         oidDB,
		indexManager:  nil,
		deviceMapping: nil,
		deviceOverlay: make(map[string]interface{}),
		startTime:     time.Now(),
		lastPoll:      time.Now(),
	}
}

// SetIndexManager assigns the index manager for Zabbix LLD support
func (va *VirtualAgent) SetIndexManager(im *store.OIDIndexManager) {
	va.mu.Lock()
	defer va.mu.Unlock()
	va.indexManager = im
}

// SetDeviceMapping assigns device-specific OID mappings to this agent
func (va *VirtualAgent) SetDeviceMapping(mapping *store.DeviceOIDMapping) {
	va.mu.Lock()
	defer va.mu.Unlock()
	va.deviceMapping = mapping
}

// HandlePacket processes an incoming SNMP packet and returns a response
func (va *VirtualAgent) HandlePacket(packet []byte) []byte {
	count := va.pollCount.Add(1)
	va.lastPoll = time.Now()

	// Log packet reception (sample every 1000th for high-volume scenarios)
	if count%1000 == 0 {
		log.Printf("Device %d (Port %d): Received packet #%d",
			va.deviceID, va.port, count)
	}

	req, reportOID, err := va.decodePacket(packet)
	if err != nil {
		log.Printf("Device %d: Failed to parse SNMP packet: %v", va.deviceID, err)
		return nil
	}

	if reportOID != "" {
		return va.handleV3USMReport(req, reportOID)
	}

	if va.shouldSendV3DiscoveryReport(req) {
		return va.handleV3DiscoveryReport(req)
	}

	switch req.PDUType {
	case gosnmp.GetNextRequest:
		return va.handleGetNextRequest(req)
	case gosnmp.SetRequest:
		return va.handleSetRequest(req)
	case gosnmp.GetBulkRequest:
		return va.handleGetBulkRequest(req)
	default:
		return va.handleGetRequest(req)
	}
}

func (va *VirtualAgent) decodePacket(packet []byte) (*gosnmp.SnmpPacket, string, error) {
	if va.v3Config.Enabled {
		// Use the full auth+priv decoder for ALL v3 traffic.
		// gosnmp reads msgFlags FROM the packet bytes — if the packet is noAuthNoPriv
		// (e.g. discovery), no HMAC verification is attempted even when auth params are
		// present in the decoder. This lets us handle both discovery and authenticated
		// packets in a single pass.
		usmParams := va.v3Config.BuildUSM(va.v3EngineBoots, uint32(time.Since(va.startTime).Seconds()))
		// Pre-initialize keys; without this, gosnmp calcPacketDigest gets a nil SecretKey.
		if initErr := usmParams.InitSecurityKeys(); initErr != nil {
			log.Printf("Device %d: Failed to initialize USM security keys: %v", va.deviceID, initErr)
		}
		secureDecoder := gosnmp.GoSNMP{
			Version:            gosnmp.Version3,
			SecurityModel:      gosnmp.UserSecurityModel,
			MsgFlags:           va.v3Config.SecurityLevel(),
			SecurityParameters: usmParams,
		}

		// Save a copy of raw bytes before SnmpDecodePacket modifies them.
		// SnmpDecodePacket zeroes the auth params and decrypts the privacy section
		// in-place. For HMAC verification, we need the original encrypted bytes
		// with only the auth params zeroed (not decrypted).
		rawCopy := make([]byte, len(packet))
		copy(rawCopy, packet)

		req, err := secureDecoder.SnmpDecodePacket(packet)
		if err == nil && req.Version == gosnmp.Version3 {
			// SnmpDecodePacket does NOT verify the incoming HMAC (it only decrypts).
			// We must manually verify authentication when the packet requests auth.
			if req.MsgFlags&gosnmp.AuthNoPriv != 0 {
				if authErr := va.verifyIncomingHMAC(rawCopy, req, usmParams); authErr != nil {
					// Auth verification failed: return WrongDigest report
					return req, v3.USMStatsWrongDigestOID, nil
				}
			}
			if reportOID := va.validateUSMWindow(req); reportOID != "" {
				return req, reportOID, nil
			}
			return req, "", nil
		}

		// Auth/digest failure: decode with noAuthNoPriv to extract packet structure
		// for the Report PDU, then signal WrongDigest.
		if err != nil && isAuthError(err) {
			noAuthDecoder := gosnmp.GoSNMP{
				Version:            gosnmp.Version3,
				SecurityModel:      gosnmp.UserSecurityModel,
				MsgFlags:           gosnmp.NoAuthNoPriv,
				SecurityParameters: &gosnmp.UsmSecurityParameters{UserName: va.v3Config.Username},
			}
			baseReq, baseErr := noAuthDecoder.SnmpDecodePacket(packet)
			if baseErr == nil {
				return baseReq, v3.USMStatsWrongDigestOID, nil
			}
			return nil, "", err
		}

		// If secure decode failed for non-auth reasons, the packet is not v3.
		if err != nil {
			// fall through to v2c / v1
		}
	}

	decoderV2 := gosnmp.GoSNMP{Version: gosnmp.Version2c, Community: "public"}
	req, err := decoderV2.SnmpDecodePacket(packet)
	if err == nil {
		return req, "", nil
	}

	decoderV1 := gosnmp.GoSNMP{Version: gosnmp.Version1, Community: "public"}
	req, err = decoderV1.SnmpDecodePacket(packet)
	if err == nil {
		return req, "", nil
	}

	return nil, "", err
}

// marshalPacket ensures USM SecretKey is initialized from the passphrase and
// the per-packet AES/DES salt is allocated before calling MarshalMsg.
// gosnmp's MarshalMsg uses SecretKey directly for HMAC signing and relies on
// InitPacket (which sets PrivacyParameters/salt) for the encryption IV.
// Callers that build fresh UsmSecurityParameters must call both before sending.
func marshalPacket(packet *gosnmp.SnmpPacket) ([]byte, error) {
	if packet.Version == gosnmp.Version3 && packet.SecurityParameters != nil {
		if usm, ok := packet.SecurityParameters.(*gosnmp.UsmSecurityParameters); ok && usm != nil {
			if err := usm.InitSecurityKeys(); err != nil {
				return nil, fmt.Errorf("init v3 security keys: %w", err)
			}
			if err := usm.InitPacket(packet); err != nil {
				return nil, fmt.Errorf("init v3 packet salt: %w", err)
			}
		}
	}
	return packet.MarshalMsg()
}

// isAuthError returns true when the error indicates an HMAC authentication failure.
func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, gosnmp.ErrWrongDigest) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "digest") || strings.Contains(msg, "authentication")
}

// verifyIncomingHMAC manually verifies the HMAC of an incoming authenticated SNMPv3 packet.
// rawCopy is a copy of the original packet bytes (before SnmpDecodePacket modifies them).
// The function zeros the auth digest bytes in rawCopy and computes HMAC, then compares
// with the received digest from the decoded packet's SecurityParameters.
func (va *VirtualAgent) verifyIncomingHMAC(rawCopy []byte, req *gosnmp.SnmpPacket, localUSM *gosnmp.UsmSecurityParameters) error {
	usmParams, ok := req.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if !ok || len(usmParams.AuthenticationParameters) == 0 {
		return nil // no auth params to verify
	}

	// Translate gosnmp auth protocol to our v3 package's AuthProtocol
	var authProto v3.AuthProtocol
	switch localUSM.AuthenticationProtocol {
	case gosnmp.MD5:
		authProto = v3.AuthMD5
	case gosnmp.SHA:
		authProto = v3.AuthSHA1
	case gosnmp.SHA224:
		authProto = v3.AuthSHA224
	case gosnmp.SHA256:
		authProto = v3.AuthSHA256
	case gosnmp.SHA384:
		authProto = v3.AuthSHA384
	case gosnmp.SHA512:
		authProto = v3.AuthSHA512
	default:
		return nil // no auth protocol configured, nothing to verify
	}

	if len(localUSM.SecretKey) == 0 {
		return nil // no localized key available, skip verification
	}

	received := []byte(usmParams.AuthenticationParameters)
	if len(received) == 0 {
		return nil
	}

	// Per RFC 3414: to verify HMAC, zero the auth params in the raw packet bytes
	// (over the encrypted packet, not decrypted), then compute HMAC and compare.
	// rawCopy contains the original bytes before SnmpDecodePacket decrypted them.
	// We find the auth digest bytes in rawCopy using the BER OCTET STRING prefix.
	// Auth params are encoded as: 04 NN [NN bytes] in the USM security parameters.
	// They appear in the first 200 bytes of the packet.
	searchLimit := len(rawCopy)
	if searchLimit > 200 {
		searchLimit = 200
	}
	authLen := byte(len(received))
	idx := -1
	for i := 0; i < searchLimit-int(authLen)-1; i++ {
		if rawCopy[i] == 0x04 && rawCopy[i+1] == authLen {
			// Check if the bytes at [i+2:i+2+authLen] match the received digest
			if bytes.Equal(rawCopy[i+2:i+2+int(authLen)], received) {
				idx = i + 2
				break
			}
		}
	}
	if idx < 0 {
		// Auth params not found in raw packet — skip verification
		return nil
	}
	// Zero the auth params in the copy
	for i := idx; i < idx+int(authLen); i++ {
		rawCopy[i] = 0
	}

	// Compute HMAC over the modified copy (encrypted payload + zeroed auth params)
	computed, err := v3.HMACDigest(authProto, localUSM.SecretKey, rawCopy)
	if err != nil {
		return fmt.Errorf("HMAC computation failed: %w", err)
	}

	// Truncate computed digest to the length of the received digest (e.g., 12 bytes for SHA1/MD5)
	truncated := computed
	if len(truncated) > len(received) {
		truncated = computed[:len(received)]
	}

	if !bytes.Equal(truncated, received) {
		return fmt.Errorf("HMAC mismatch: wrong authentication key")
	}
	return nil
}

func buildResponseFromRequest(req *gosnmp.SnmpPacket, vars []gosnmp.SnmpPDU, errCode gosnmp.SNMPError, errIndex uint8) *gosnmp.SnmpPacket {
	response := *req
	response.PDUType = gosnmp.GetResponse
	response.Variables = vars
	response.Error = errCode
	response.ErrorIndex = errIndex
	return &response
}

func (va *VirtualAgent) buildResponseFromRequest(req *gosnmp.SnmpPacket, vars []gosnmp.SnmpPDU, errCode gosnmp.SNMPError, errIndex uint8) *gosnmp.SnmpPacket {
	response := buildResponseFromRequest(req, vars, errCode, errIndex)

	if response.Version == gosnmp.Version3 {
		response.MsgFlags = req.MsgFlags & gosnmp.AuthPriv
		response.SecurityModel = gosnmp.UserSecurityModel
		response.ContextEngineID = va.v3Config.EngineID

		username := va.v3Config.Username
		if req.SecurityParameters != nil {
			if usm, ok := req.SecurityParameters.(*gosnmp.UsmSecurityParameters); ok && usm.UserName != "" {
				username = usm.UserName
			}
		}

		cfg := va.v3ConfigForFlags(response.MsgFlags)
		cfg.Username = username
		response.SecurityParameters = cfg.BuildUSM(
			va.v3EngineBoots,
			uint32(time.Since(va.startTime).Seconds()),
		)
	}

	return response
}

func (va *VirtualAgent) shouldSendV3DiscoveryReport(req *gosnmp.SnmpPacket) bool {
	if req == nil || req.Version != gosnmp.Version3 {
		return false
	}
	if !va.v3Config.Enabled {
		return false
	}

	usm, ok := req.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if !ok || usm == nil {
		return true
	}

	return usm.AuthoritativeEngineID == ""
}

func (va *VirtualAgent) validateUSMWindow(req *gosnmp.SnmpPacket) string {
	if req == nil || req.Version != gosnmp.Version3 {
		return ""
	}

	usm, ok := req.SecurityParameters.(*gosnmp.UsmSecurityParameters)
	if !ok || usm == nil {
		return v3.USMStatsUnknownEngineIDOID
	}

	if usm.AuthoritativeEngineID != "" && usm.AuthoritativeEngineID != va.v3Config.EngineID {
		return v3.USMStatsUnknownEngineIDOID
	}

	if usm.AuthoritativeEngineID != "" {
		now := uint32(time.Since(va.startTime).Seconds())
		if usm.AuthoritativeEngineBoots != va.v3EngineBoots {
			return v3.USMStatsNotInTimeWindowOID
		}

		var diff uint32
		if now > usm.AuthoritativeEngineTime {
			diff = now - usm.AuthoritativeEngineTime
		} else {
			diff = usm.AuthoritativeEngineTime - now
		}
		if diff > 150 {
			return v3.USMStatsNotInTimeWindowOID
		}
	}

	return ""
}

func (va *VirtualAgent) handleV3USMReport(req *gosnmp.SnmpPacket, oid string) []byte {
	response := va.buildResponseFromRequest(req, []gosnmp.SnmpPDU{v3.BuildUSMReportVar(oid)}, gosnmp.NoError, 0)
	response.PDUType = gosnmp.Report

	data, err := marshalPacket(response)
	if err != nil {
		log.Printf("Device %d: Failed to marshal v3 USM report: %v", va.deviceID, err)
		return nil
	}
	return data
}

func (va *VirtualAgent) v3ConfigForFlags(flags gosnmp.SnmpV3MsgFlags) v3.Config {
	cfg := va.v3Config
	level := flags & gosnmp.AuthPriv
	if level == gosnmp.NoAuthNoPriv {
		cfg.Auth = v3.AuthNone
		cfg.AuthKey = ""
		cfg.Priv = v3.PrivNone
		cfg.PrivKey = ""
		return cfg
	}
	if level == gosnmp.AuthNoPriv {
		cfg.Priv = v3.PrivNone
		cfg.PrivKey = ""
	}
	return cfg
}

func (va *VirtualAgent) handleV3DiscoveryReport(req *gosnmp.SnmpPacket) []byte {
	requestUsername := ""
	if req != nil && req.SecurityParameters != nil {
		if usm, ok := req.SecurityParameters.(*gosnmp.UsmSecurityParameters); ok && usm != nil {
			requestUsername = usm.UserName
		}
	}

	vars := []gosnmp.SnmpPDU{
		{
			Name:  ".1.3.6.1.6.3.15.1.1.4.0",
			Type:  gosnmp.Counter32,
			Value: uint(1),
		},
	}

	response := va.buildResponseFromRequest(req, vars, gosnmp.NoError, 0)
	response.PDUType = gosnmp.Report
	if usm, ok := response.SecurityParameters.(*gosnmp.UsmSecurityParameters); ok && usm != nil {
		usm.UserName = requestUsername
	}

	data, err := marshalPacket(response)
	if err != nil {
		log.Printf("Device %d: Failed to marshal v3 discovery report: %v", va.deviceID, err)
		return nil
	}

	return data
}

// handleGetRequest processes GET requests
func (va *VirtualAgent) handleGetRequest(req *gosnmp.SnmpPacket) []byte {
	// Pre-allocate response variables
	vars := make([]gosnmp.SnmpPDU, 0, len(req.Variables))
	
	// Process each variable with minimal lock time
	for _, v := range req.Variables {
		va.mu.RLock()
		value := va.getOIDValue(v.Name)
		va.mu.RUnlock()
		
		vars = append(vars, gosnmp.SnmpPDU{
			Name:  v.Name,
			Type:  value.Type,
			Value: value.Value,
		})
	}

	// Marshal response without holding lock
	outPacket := va.buildResponseFromRequest(req, vars, gosnmp.NoError, 0)

	// Marshal response
	data, err := marshalPacket(outPacket)
	if err != nil {
		log.Printf("Device %d: Failed to marshal response: %v", va.deviceID, err)
		return nil
	}

	return data
}

// handleGetNextRequest processes GETNEXT requests (walk operation)
func (va *VirtualAgent) handleGetNextRequest(req *gosnmp.SnmpPacket) []byte {
	// Pre-allocate response variables
	vars := make([]gosnmp.SnmpPDU, 0, len(req.Variables))
	
	// Process each variable with minimal lock time
	for _, v := range req.Variables {
		va.mu.RLock()
		nextOID, val := va.getNextOID(v.Name)
		va.mu.RUnlock()
		
		if val != nil {
			vars = append(vars, gosnmp.SnmpPDU{
				Name:  nextOID,
				Type:  val.Type,
				Value: val.Value,
			})
		}
	}

	// Marshal response without holding lock
	outPacket := va.buildResponseFromRequest(req, vars, gosnmp.NoError, 0)

	data, err := marshalPacket(outPacket)
	if err != nil {
		log.Printf("Device %d: Failed to marshal response: %v", va.deviceID, err)
		return nil
	}

	return data
}

// handleGetBulkRequest processes GETBULK requests (efficient walk)
// Zabbix default: NonRepeaters=0, MaxRepeaters=10
func (va *VirtualAgent) handleGetBulkRequest(req *gosnmp.SnmpPacket) []byte {
	// Pre-allocate response variables
	nonRepeaters := int(req.NonRepeaters)
	if nonRepeaters < 0 {
		nonRepeaters = 0
	}
	maxRepeaters := int(req.MaxRepetitions)
	if maxRepeaters <= 0 {
		maxRepeaters = 10
	}

	vars := make([]gosnmp.SnmpPDU, 0, len(req.Variables)*maxRepeaters)

	// Process each variable with minimal lock time
	for i, v := range req.Variables {
		if i < nonRepeaters {
			va.mu.RLock()
			nextOID, val := va.getNextOID(v.Name)
			va.mu.RUnlock()
			
			if val != nil {
				vars = append(vars, gosnmp.SnmpPDU{
					Name:  nextOID,
					Type:  val.Type,
					Value: val.Value,
				})
			}
			continue
		}

		// For repeaters, get multiple consecutive OIDs
		currentOID := v.Name
		for r := 0; r < maxRepeaters; r++ {
			va.mu.RLock()
			nextOID, val := va.getNextOID(currentOID)
			va.mu.RUnlock()
			
			if val == nil || val.Type == gosnmp.EndOfMibView {
				break
			}
			vars = append(vars, gosnmp.SnmpPDU{
				Name:  nextOID,
				Type:  val.Type,
				Value: val.Value,
			})
			currentOID = nextOID
		}
	}

	outPacket := va.buildResponseFromRequest(req, vars, gosnmp.NoError, 0)

	data, err := marshalPacket(outPacket)
	if err != nil {
		log.Printf("Device %d: Failed to marshal GETBULK response: %v", va.deviceID, err)
		return nil
	}

	return data
}

// handleSetRequest returns read-only error response
func (va *VirtualAgent) handleSetRequest(req *gosnmp.SnmpPacket) []byte {
	outPacket := va.buildResponseFromRequest(req, []gosnmp.SnmpPDU{}, 4, 1)

	data, err := marshalPacket(outPacket)
	if err != nil {
		log.Printf("Device %d: Failed to marshal SET response: %v", va.deviceID, err)
		return nil
	}

	return data
}

// getOIDValue retrieves the value for a specific OID
// Priority: device mapping (port/device-specific) > device overlay > system OIDs > OID database
func (va *VirtualAgent) getOIDValue(oid string) *store.OIDValue {
	oid = normalizeOID(oid)
	// Check device mapping first (highest priority)
	if va.deviceMapping != nil {
		if val := va.deviceMapping.GetOID(oid, va.port, va.sysName); val != nil {
			return val
		}
	}

	// Check device overlay second
	if val, ok := va.deviceOverlay[oid]; ok {
		return &store.OIDValue{
			Type:  gosnmp.OctetString,
			Value: val,
		}
	}

	// Check for special system OIDs
	if val := va.getSystemOID(oid); val != nil {
		return val
	}

	// Query OID database
	val := va.oidDB.Get(oid)
	if val != nil {
		return val
	}

	// Return noSuchObject
	return &store.OIDValue{
		Type:  gosnmp.NoSuchObject,
		Value: nil,
	}
}

// getNextOID retrieves the next OID after the given one
// Uses index manager if available for table-aware traversal (Zabbix LLD)
func (va *VirtualAgent) getNextOID(oid string) (string, *store.OIDValue) {
	oid = normalizeOID(oid)
	// Try index manager first (optimized for table traversal)
	if va.indexManager != nil {
		nextOID, val := va.indexManager.GetNext(oid, va.oidDB)
		// If index manager returned a value with unknown type (0/EndOfContents),
		// resolve the proper type from the OID database or system OIDs.
		if val != nil && val.Type == gosnmp.EndOfContents {
			if resolved := va.getOIDValue(nextOID); resolved != nil && resolved.Type != gosnmp.NoSuchObject {
				val = resolved
			}
		}
		return nextOID, val
	}

	// Fallback: basic database traversal
	nextOID := va.oidDB.GetNext(oid)
	if nextOID == "" {
		return oid, &store.OIDValue{
			Type:  gosnmp.EndOfMibView,
			Value: nil,
		}
	}

	value := va.getOIDValue(nextOID)
	return nextOID, value
}

func normalizeOID(oid string) string {
	if len(oid) > 0 && oid[0] == '.' {
		return oid[1:]
	}
	return oid
}

// getSystemOID returns system-specific OID values
func (va *VirtualAgent) getSystemOID(oid string) *store.OIDValue {
	switch oid {
	case "1.3.6.1.2.1.1.3.0": // sysUpTime
		uptime := uint32(time.Since(va.startTime).Seconds() * 100)
		return &store.OIDValue{
			Type:  gosnmp.TimeTicks,
			Value: uptime,
		}

	case "1.3.6.1.2.1.1.5.0": // sysName
		return &store.OIDValue{
			Type:  gosnmp.OctetString,
			Value: va.sysName,
		}

	case "1.3.6.1.2.1.1.6.0": // sysLocation
		return &store.OIDValue{
			Type:  gosnmp.OctetString,
			Value: fmt.Sprintf("Simulated-Device-%d", va.deviceID),
		}

	case "1.3.6.1.2.1.25.3.2.1.5.1": // hrDeviceIndex (random CPU load simulation)
		cpuLoad := rand.Intn(100)
		return &store.OIDValue{
			Type:  gosnmp.Integer,
			Value: cpuLoad,
		}
	}

	return nil
}

// SetOIDValue sets a device-specific OID value (overlay)
func (va *VirtualAgent) SetOIDValue(oid string, value interface{}) {
	va.mu.Lock()
	defer va.mu.Unlock()
	va.deviceOverlay[oid] = value
}

// GetStatistics returns agent statistics
func (va *VirtualAgent) GetStatistics() map[string]interface{} {
	va.mu.RLock()
	defer va.mu.RUnlock()

	uptime := uint32(time.Since(va.startTime).Seconds())
	return map[string]interface{}{
		"device_id":  va.deviceID,
		"port":       va.port,
		"sysName":    va.sysName,
		"uptime":     uptime,
		"poll_count": va.pollCount.Load(),
		"last_poll":  va.lastPoll.Format(time.RFC3339),
	}
}
