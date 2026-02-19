package traps

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/robfig/cron/v3"
)

const (
	TrapOIDCron      = "1.3.6.1.4.1.55555.0.1"
	TrapOIDVariation = "1.3.6.1.4.1.55555.0.2"
	TrapOIDSet       = "1.3.6.1.4.1.55555.0.3"
)

type Config struct {
	Targets     []string
	Version     string
	Community   string
	V3User      string
	V3Auth      string
	V3AuthKey   string
	V3Priv      string
	V3PrivKey   string
	CronSpecs   []string
	OnVariation bool
	OnSetOIDs   []string
	Inform      bool

	Timeout time.Duration
	Retries int
}

func (c *Config) Normalize() error {
	c.Version = strings.ToLower(strings.TrimSpace(c.Version))
	if c.Version == "" {
		c.Version = "v2c"
	}
	if c.Version != "v2c" && c.Version != "v3" {
		return fmt.Errorf("invalid trap version %q (want v2c or v3)", c.Version)
	}
	if len(c.Targets) == 0 {
		return nil
	}

	for i, t := range c.Targets {
		host, port, err := net.SplitHostPort(t)
		if err != nil || host == "" || port == "" {
			return fmt.Errorf("invalid trap target %q (want host:port)", t)
		}
		if _, err := strconv.Atoi(port); err != nil {
			return fmt.Errorf("invalid trap target port in %q", t)
		}
		c.Targets[i] = net.JoinHostPort(host, port)
	}

	if c.Timeout <= 0 {
		c.Timeout = 2 * time.Second
	}
	if c.Retries < 0 {
		c.Retries = 0
	}

	if c.Version == "v2c" {
		if c.Community == "" {
			c.Community = "public"
		}
		return nil
	}

	if c.V3User == "" {
		return fmt.Errorf("trap v3 requires username")
	}
	auth := parseV3Auth(c.V3Auth)
	if auth == gosnmp.NoAuth {
		if strings.TrimSpace(c.V3AuthKey) != "" || strings.TrimSpace(c.V3Priv) != "" || strings.TrimSpace(c.V3PrivKey) != "" {
			return fmt.Errorf("trap v3 auth/priv parameters require valid --v3-auth")
		}
		return nil
	}
	if c.V3AuthKey == "" {
		return fmt.Errorf("trap v3 auth protocol requires auth key")
	}
	priv := parseV3Priv(c.V3Priv)
	if priv != gosnmp.NoPriv && c.V3PrivKey == "" {
		return fmt.Errorf("trap v3 priv protocol requires priv key")
	}
	if priv == gosnmp.NoPriv && c.V3PrivKey != "" {
		return fmt.Errorf("trap v3 priv key provided but priv protocol is empty")
	}
	return nil
}

type message struct {
	trapOID string
	vars    []gosnmp.SnmpPDU
}

type Manager struct {
	config    Config
	sender    *Sender
	onSetOIDs map[string]struct{}

	queue chan message
	stop  chan struct{}
	wg    sync.WaitGroup

	cron *cron.Cron
}

func NewManager(cfg Config) (*Manager, error) {
	if err := cfg.Normalize(); err != nil {
		return nil, err
	}
	if len(cfg.Targets) == 0 {
		return nil, nil
	}

	builder, err := NewBuilder(cfg)
	if err != nil {
		return nil, err
	}

	onSet := make(map[string]struct{}, len(cfg.OnSetOIDs))
	for _, oid := range cfg.OnSetOIDs {
		oid = strings.TrimPrefix(strings.TrimSpace(oid), ".")
		if oid != "" {
			onSet[oid] = struct{}{}
		}
	}

	m := &Manager{
		config:    cfg,
		sender:    NewSender(builder, cfg.Targets, cfg.Inform),
		onSetOIDs: onSet,
		queue:     make(chan message, 1024),
		stop:      make(chan struct{}),
	}

	if len(cfg.CronSpecs) > 0 {
		m.cron = cron.New(cron.WithParser(cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)))
		for _, spec := range cfg.CronSpecs {
			s := strings.TrimSpace(spec)
			if s == "" {
				continue
			}
			if _, err := m.cron.AddFunc(s, func() {
				m.EnqueueCronEvent("cron")
			}); err != nil {
				return nil, fmt.Errorf("invalid cron spec %q: %w", s, err)
			}
		}
	}

	return m, nil
}

func (m *Manager) Start() {
	if m == nil {
		return
	}
	m.wg.Add(1)
	go m.loop()
	if m.cron != nil {
		m.cron.Start()
	}
}

func (m *Manager) Stop() {
	if m == nil {
		return
	}
	if m.cron != nil {
		ctx := m.cron.Stop()
		<-ctx.Done()
	}
	close(m.stop)
	m.wg.Wait()
}

func (m *Manager) loop() {
	defer m.wg.Done()
	for {
		select {
		case <-m.stop:
			return
		case msg := <-m.queue:
			if err := m.sender.Send(msg.trapOID, msg.vars); err != nil {
				log.Printf("trap send failed: %v", err)
			}
		}
	}
}

func (m *Manager) enqueue(trapOID string, vars []gosnmp.SnmpPDU) {
	if m == nil {
		return
	}
	select {
	case m.queue <- message{trapOID: trapOID, vars: vars}:
	default:
		log.Printf("trap queue full; dropping event %s", trapOID)
	}
}

func (m *Manager) EnqueueCronEvent(spec string) {
	vars := []gosnmp.SnmpPDU{
		{Name: ".1.3.6.1.4.1.55555.1.1.0", Type: gosnmp.OctetString, Value: "cron"},
		{Name: ".1.3.6.1.4.1.55555.1.2.0", Type: gosnmp.OctetString, Value: spec},
	}
	m.enqueue(TrapOIDCron, vars)
}

func (m *Manager) EnqueueVariationEvent(deviceID int, port int, oid string, detail string) {
	if m == nil || !m.config.OnVariation {
		return
	}
	vars := []gosnmp.SnmpPDU{
		{Name: ".1.3.6.1.4.1.55555.2.1.0", Type: gosnmp.OctetString, Value: strings.TrimPrefix(oid, ".")},
		{Name: ".1.3.6.1.4.1.55555.2.2.0", Type: gosnmp.OctetString, Value: detail},
		{Name: ".1.3.6.1.4.1.55555.2.3.0", Type: gosnmp.Integer, Value: deviceID},
		{Name: ".1.3.6.1.4.1.55555.2.4.0", Type: gosnmp.Integer, Value: port},
	}
	m.enqueue(TrapOIDVariation, vars)
}

func (m *Manager) EnqueueSetEvent(deviceID int, port int, oid string, valueType string, valueText string) {
	if m == nil {
		return
	}
	oid = strings.TrimPrefix(strings.TrimSpace(oid), ".")
	if len(m.onSetOIDs) > 0 {
		if _, ok := m.onSetOIDs[oid]; !ok {
			return
		}
	}

	vars := []gosnmp.SnmpPDU{
		{Name: ".1.3.6.1.4.1.55555.3.1.0", Type: gosnmp.OctetString, Value: oid},
		{Name: ".1.3.6.1.4.1.55555.3.2.0", Type: gosnmp.OctetString, Value: valueType},
		{Name: ".1.3.6.1.4.1.55555.3.3.0", Type: gosnmp.OctetString, Value: valueText},
		{Name: ".1.3.6.1.4.1.55555.3.4.0", Type: gosnmp.Integer, Value: deviceID},
		{Name: ".1.3.6.1.4.1.55555.3.5.0", Type: gosnmp.Integer, Value: port},
	}
	m.enqueue(TrapOIDSet, vars)
}

type Builder interface {
	Build(target string) (*gosnmp.GoSNMP, error)
}

type v2Builder struct {
	community string
	timeout   time.Duration
	retries   int
}

func (b *v2Builder) Build(target string) (*gosnmp.GoSNMP, error) {
	host, port, err := parseTarget(target)
	if err != nil {
		return nil, err
	}
	return &gosnmp.GoSNMP{
		Target:    host,
		Port:      port,
		Version:   gosnmp.Version2c,
		Community: b.community,
		Timeout:   b.timeout,
		Retries:   b.retries,
	}, nil
}

type v3Builder struct {
	user    string
	auth    gosnmp.SnmpV3AuthProtocol
	authKey string
	priv    gosnmp.SnmpV3PrivProtocol
	privKey string
	timeout time.Duration
	retries int
}

func (b *v3Builder) Build(target string) (*gosnmp.GoSNMP, error) {
	host, port, err := parseTarget(target)
	if err != nil {
		return nil, err
	}
	flags := gosnmp.NoAuthNoPriv
	if b.auth != gosnmp.NoAuth {
		flags = gosnmp.AuthNoPriv
	}
	if b.priv != gosnmp.NoPriv {
		flags = gosnmp.AuthPriv
	}

	return &gosnmp.GoSNMP{
		Target:        host,
		Port:          port,
		Version:       gosnmp.Version3,
		Timeout:       b.timeout,
		Retries:       b.retries,
		SecurityModel: gosnmp.UserSecurityModel,
		MsgFlags:      flags,
		SecurityParameters: &gosnmp.UsmSecurityParameters{
			UserName:                 b.user,
			AuthenticationProtocol:   b.auth,
			AuthenticationPassphrase: b.authKey,
			PrivacyProtocol:          b.priv,
			PrivacyPassphrase:        b.privKey,
		},
	}, nil
}

func NewBuilder(cfg Config) (Builder, error) {
	if cfg.Version == "v3" {
		return &v3Builder{
			user:    cfg.V3User,
			auth:    parseV3Auth(cfg.V3Auth),
			authKey: cfg.V3AuthKey,
			priv:    parseV3Priv(cfg.V3Priv),
			privKey: cfg.V3PrivKey,
			timeout: cfg.Timeout,
			retries: cfg.Retries,
		}, nil
	}
	return &v2Builder{community: cfg.Community, timeout: cfg.Timeout, retries: cfg.Retries}, nil
}

type Sender struct {
	builder Builder
	targets []string
	inform  bool
}

func NewSender(builder Builder, targets []string, inform bool) *Sender {
	return &Sender{builder: builder, targets: append([]string(nil), targets...), inform: inform}
}

func (s *Sender) Send(trapOID string, vars []gosnmp.SnmpPDU) error {
	if len(s.targets) == 0 {
		return nil
	}
	fullVars := append([]gosnmp.SnmpPDU{
		{Name: ".1.3.6.1.2.1.1.3.0", Type: gosnmp.TimeTicks, Value: uint32(time.Now().Unix() % 4294967295)},
		{Name: ".1.3.6.1.6.3.1.1.4.1.0", Type: gosnmp.ObjectIdentifier, Value: trapOID},
	}, vars...)

	trap := gosnmp.SnmpTrap{Variables: fullVars, IsInform: s.inform}
	for _, target := range s.targets {
		client, err := s.builder.Build(target)
		if err != nil {
			return err
		}
		if err := client.Connect(); err != nil {
			return fmt.Errorf("connect trap target %s: %w", target, err)
		}
		_, err = client.SendTrap(trap)
		_ = client.Conn.Close()
		if err != nil {
			return fmt.Errorf("send trap to %s: %w", target, err)
		}
	}
	return nil
}

func parseTarget(target string) (string, uint16, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(target))
	if err != nil {
		return "", 0, fmt.Errorf("invalid trap target %q", target)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n <= 0 || n > 65535 {
		return "", 0, fmt.Errorf("invalid trap target port %q", target)
	}
	return host, uint16(n), nil
}

func parseV3Auth(s string) gosnmp.SnmpV3AuthProtocol {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "NONE":
		return gosnmp.NoAuth
	case "MD5":
		return gosnmp.MD5
	case "SHA", "SHA1":
		return gosnmp.SHA
	case "SHA224":
		return gosnmp.SHA224
	case "SHA256":
		return gosnmp.SHA256
	case "SHA384":
		return gosnmp.SHA384
	case "SHA512":
		return gosnmp.SHA512
	default:
		return gosnmp.NoAuth
	}
}

func parseV3Priv(s string) gosnmp.SnmpV3PrivProtocol {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "", "NONE":
		return gosnmp.NoPriv
	case "DES":
		return gosnmp.DES
	case "AES", "AES128":
		return gosnmp.AES
	case "AES192":
		return gosnmp.AES192
	case "AES256":
		return gosnmp.AES256
	default:
		return gosnmp.NoPriv
	}
}
