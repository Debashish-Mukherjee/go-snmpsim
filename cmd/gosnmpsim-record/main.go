package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/debashish-mukherjee/go-snmpsim/internal/recorder"
	"github.com/debashish-mukherjee/go-snmpsim/internal/snmprecfmt"
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
	target := flag.String("target", "127.0.0.1", "SNMP target host")
	port := flag.Uint("port", 161, "SNMP target port")
	out := flag.String("out", "", "Output .snmprec path")
	community := flag.String("community", "", "SNMP community (v1/v2c mode)")
	v3User := flag.String("v3-user", "", "SNMPv3 username")
	v3Auth := flag.String("v3-auth", "", "SNMPv3 auth protocol: MD5,SHA1,SHA224,SHA256,SHA384,SHA512")
	v3AuthKey := flag.String("v3-auth-key", "", "SNMPv3 auth passphrase")
	v3Priv := flag.String("v3-priv", "", "SNMPv3 privacy protocol: DES,AES128,AES192,AES256")
	v3PrivKey := flag.String("v3-priv-key", "", "SNMPv3 privacy passphrase")
	maxOIDs := flag.Int("max-oids", 0, "Maximum OIDs to record (0 = unlimited)")
	rateLimit := flag.Int("rate-limit", 0, "Maximum OIDs processed per second (0 = unlimited)")
	timeout := flag.Duration("timeout", 2*time.Second, "Request timeout")
	retries := flag.Int("retries", 0, "SNMP retries")

	var excludes stringSliceFlag
	flag.Var(&excludes, "exclude", "OID prefix to exclude (repeatable or comma-separated)")

	flag.Parse()

	if *out == "" {
		fmt.Fprintln(os.Stderr, "missing required flag: --out")
		os.Exit(2)
	}

	entries, err := recorder.Record(recorder.Options{
		Target:    *target,
		Port:      uint16(*port),
		Timeout:   *timeout,
		Retries:   *retries,
		MaxOIDs:   *maxOIDs,
		RateLimit: *rateLimit,
		Exclude:   excludes,
		Community: *community,
		V3User:    *v3User,
		V3Auth:    *v3Auth,
		V3AuthKey: *v3AuthKey,
		V3Priv:    *v3Priv,
		V3PrivKey: *v3PrivKey,
	})
	if err != nil {
		log.Fatalf("record failed: %v", err)
	}

	if err := snmprecfmt.WriteFile(*out, entries); err != nil {
		log.Fatalf("write output: %v", err)
	}

	log.Printf("Recorded %d OIDs to %s", len(entries), *out)
}
