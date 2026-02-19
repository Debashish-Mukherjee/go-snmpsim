package recorder

import (
	"fmt"
	"strings"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/snmprecfmt"
	"github.com/gosnmp/gosnmp"
)

var DefaultRoots = []string{
	"1.3.6.1.2.1.1",
	"1.3.6.1.2.1.2.2",
	"1.3.6.1.2.1.31.1.1",
	"1.3.6.1.2.1.25",
	"1.3.6.1.2.1.99",
	"1.3.6.1.4.1",
}

type Options struct {
	Target    string
	Port      uint16
	Timeout   time.Duration
	Retries   int
	MaxOIDs   int
	RateLimit int

	Roots   []string
	Exclude []string

	Community string

	V3User    string
	V3Auth    string
	V3AuthKey string
	V3Priv    string
	V3PrivKey string
}

func Record(opts Options) ([]snmprecfmt.Entry, error) {
	client, err := newClient(opts)
	if err != nil {
		return nil, err
	}

	if err := client.Connect(); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer client.Conn.Close()

	roots := opts.Roots
	if len(roots) == 0 {
		roots = append([]string(nil), DefaultRoots...)
	}

	entries := make(map[string]snmprecfmt.Entry)
	rootErrors := make([]error, 0)

	var throttle <-chan time.Time
	if opts.RateLimit > 0 {
		interval := time.Second / time.Duration(opts.RateLimit)
		if interval <= 0 {
			interval = time.Millisecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		throttle = ticker.C
	}

	for _, root := range roots {
		if opts.MaxOIDs > 0 && len(entries) >= opts.MaxOIDs {
			break
		}
		if err := walkRoot(client, strings.TrimPrefix(root, "."), opts.Exclude, opts.MaxOIDs, entries, throttle); err != nil {
			rootErrors = append(rootErrors, fmt.Errorf("root %s: %w", root, err))
		}
	}

	if len(entries) == 0 && len(rootErrors) > 0 {
		return nil, rootErrors[0]
	}

	out := make([]snmprecfmt.Entry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry)
	}
	snmprecfmt.SortEntries(out)
	return out, nil
}

func walkRoot(client *gosnmp.GoSNMP, root string, excludes []string, maxOIDs int, entries map[string]snmprecfmt.Entry, throttle <-chan time.Time) error {
	current := root
	for {
		if maxOIDs > 0 && len(entries) >= maxOIDs {
			return nil
		}

		if throttle != nil {
			<-throttle
		}

		pkt, err := client.GetNext([]string{current})
		if err != nil {
			return fmt.Errorf("getnext %s: %w", current, err)
		}
		if pkt == nil || len(pkt.Variables) == 0 {
			return nil
		}

		pdu := pkt.Variables[0]
		if pdu.Type == gosnmp.EndOfMibView || pdu.Type == gosnmp.NoSuchObject || pdu.Type == gosnmp.NoSuchInstance {
			return nil
		}

		oid := strings.TrimPrefix(pdu.Name, ".")
		if !isInSubtree(oid, root) {
			return nil
		}

		current = oid
		if shouldExclude(oid, excludes) {
			continue
		}
		if _, exists := entries[oid]; exists {
			continue
		}

		entry, err := snmprecfmt.EntryFromPDU(oid, pdu.Type, pdu.Value)
		if err != nil {
			return fmt.Errorf("convert %s: %w", oid, err)
		}
		entries[oid] = entry
	}
}

func newClient(opts Options) (*gosnmp.GoSNMP, error) {
	target := opts.Target
	if target == "" {
		target = "127.0.0.1"
	}
	port := opts.Port
	if port == 0 {
		port = 161
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	retries := opts.Retries
	if retries < 0 {
		retries = 0
	}

	if opts.Community != "" && opts.V3User != "" {
		return nil, fmt.Errorf("use either community (v1/v2c) or v3 flags, not both")
	}

	if opts.V3User != "" {
		usm := &gosnmp.UsmSecurityParameters{UserName: opts.V3User}
		flags := gosnmp.NoAuthNoPriv

		auth := parseV3Auth(opts.V3Auth)
		if auth != gosnmp.NoAuth {
			if opts.V3AuthKey == "" {
				return nil, fmt.Errorf("v3 auth protocol set but --v3-auth-key is empty")
			}
			usm.AuthenticationProtocol = auth
			usm.AuthenticationPassphrase = opts.V3AuthKey
			flags = gosnmp.AuthNoPriv
		}

		priv := parseV3Priv(opts.V3Priv)
		if priv != gosnmp.NoPriv {
			if auth == gosnmp.NoAuth {
				return nil, fmt.Errorf("v3 privacy requires auth protocol")
			}
			if opts.V3PrivKey == "" {
				return nil, fmt.Errorf("v3 privacy protocol set but --v3-priv-key is empty")
			}
			usm.PrivacyProtocol = priv
			usm.PrivacyPassphrase = opts.V3PrivKey
			flags = gosnmp.AuthPriv
		}

		return &gosnmp.GoSNMP{
			Target:             target,
			Port:               port,
			Version:            gosnmp.Version3,
			Timeout:            timeout,
			Retries:            retries,
			SecurityModel:      gosnmp.UserSecurityModel,
			MsgFlags:           flags,
			SecurityParameters: usm,
		}, nil
	}

	community := opts.Community
	if community == "" {
		return nil, fmt.Errorf("set either --community or --v3-user")
	}

	return &gosnmp.GoSNMP{
		Target:    target,
		Port:      port,
		Version:   gosnmp.Version2c,
		Community: community,
		Timeout:   timeout,
		Retries:   retries,
	}, nil
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

func isInSubtree(oid, root string) bool {
	return oid == root || strings.HasPrefix(oid, root+".")
}

func shouldExclude(oid string, excludes []string) bool {
	for _, ex := range excludes {
		ex = strings.TrimPrefix(strings.TrimSpace(ex), ".")
		if ex == "" {
			continue
		}
		if oid == ex || strings.HasPrefix(oid, ex+".") {
			return true
		}
	}
	return false
}
