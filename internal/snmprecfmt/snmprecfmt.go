package snmprecfmt

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/gosnmp/gosnmp"
)

type Entry struct {
	OID   string
	Type  string
	Value string
}

func EntryFromPDU(oid string, ber gosnmp.Asn1BER, value interface{}) (Entry, error) {
	typeName := TypeName(ber)
	valueText, err := ValueString(ber, value)
	if err != nil {
		return Entry{}, err
	}
	return Entry{OID: strings.TrimPrefix(oid, "."), Type: typeName, Value: valueText}, nil
}

func TypeName(ber gosnmp.Asn1BER) string {
	switch ber {
	case gosnmp.Integer:
		return "integer"
	case gosnmp.OctetString:
		return "octetstring"
	case gosnmp.ObjectIdentifier:
		return "objectidentifier"
	case gosnmp.IPAddress:
		return "ipaddress"
	case gosnmp.Counter32:
		return "counter32"
	case gosnmp.Gauge32:
		return "gauge32"
	case gosnmp.TimeTicks:
		return "timeticks"
	case gosnmp.Opaque:
		return "opaque"
	case gosnmp.NsapAddress:
		return "nsapaddress"
	case gosnmp.Counter64:
		return "counter64"
	case gosnmp.Uinteger32:
		return "gauge32"
	case gosnmp.BitString:
		return "bits"
	case gosnmp.Null:
		return "null"
	default:
		return fmt.Sprintf("type-%d", int(ber))
	}
}

func ValueString(ber gosnmp.Asn1BER, value interface{}) (string, error) {
	switch ber {
	case gosnmp.Integer:
		n, err := toInt64(value)
		if err != nil {
			return "", err
		}
		return strconv.FormatInt(n, 10), nil
	case gosnmp.Counter32, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Uinteger32:
		n, err := toUint64(value)
		if err != nil {
			return "", err
		}
		return strconv.FormatUint(n, 10), nil
	case gosnmp.Counter64:
		n, err := toUint64(value)
		if err != nil {
			return "", err
		}
		return strconv.FormatUint(n, 10), nil
	case gosnmp.Null:
		return "", nil
	default:
		return stringify(value), nil
	}
}

func SortEntries(entries []Entry) {
	sort.Slice(entries, func(i, j int) bool {
		return CompareOID(entries[i].OID, entries[j].OID) < 0
	})
}

func CompareOID(a, b string) int {
	if a == b {
		return 0
	}
	a = strings.TrimPrefix(a, ".")
	b = strings.TrimPrefix(b, ".")
	aa := strings.Split(a, ".")
	bb := strings.Split(b, ".")
	max := len(aa)
	if len(bb) > max {
		max = len(bb)
	}
	for i := 0; i < max; i++ {
		if i >= len(aa) {
			return -1
		}
		if i >= len(bb) {
			return 1
		}
		ai, aErr := strconv.Atoi(aa[i])
		bi, bErr := strconv.Atoi(bb[i])
		switch {
		case aErr == nil && bErr == nil:
			if ai < bi {
				return -1
			}
			if ai > bi {
				return 1
			}
		default:
			if aa[i] < bb[i] {
				return -1
			}
			if aa[i] > bb[i] {
				return 1
			}
		}
	}
	return 0
}

func WriteFile(path string, entries []Entry) error {
	copyEntries := append([]Entry(nil), entries...)
	SortEntries(copyEntries)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	for _, entry := range copyEntries {
		if _, err := fmt.Fprintf(w, "%s|%s|%s\n", entry.OID, entry.Type, entry.Value); err != nil {
			return err
		}
	}
	return w.Flush()
}

func ReadFile(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	entries := make([]Entry, 0, len(lines))
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid snmprec line %d: %q", i+1, line)
		}
		entries = append(entries, Entry{
			OID:   strings.TrimPrefix(strings.TrimSpace(parts[0]), "."),
			Type:  strings.ToLower(strings.TrimSpace(parts[1])),
			Value: strings.TrimSpace(parts[2]),
		})
	}
	SortEntries(entries)
	return entries, nil
}

func stringify(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return fmt.Sprint(v)
	}
}

func toInt64(v interface{}) (int64, error) {
	switch n := v.(type) {
	case int:
		return int64(n), nil
	case int8:
		return int64(n), nil
	case int16:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case uint:
		return int64(n), nil
	case uint8:
		return int64(n), nil
	case uint16:
		return int64(n), nil
	case uint32:
		return int64(n), nil
	case uint64:
		return int64(n), nil
	default:
		return 0, fmt.Errorf("unsupported integer value type %T", v)
	}
}

func toUint64(v interface{}) (uint64, error) {
	switch n := v.(type) {
	case int:
		if n < 0 {
			return 0, fmt.Errorf("negative integer value %d", n)
		}
		return uint64(n), nil
	case int8:
		if n < 0 {
			return 0, fmt.Errorf("negative integer value %d", n)
		}
		return uint64(n), nil
	case int16:
		if n < 0 {
			return 0, fmt.Errorf("negative integer value %d", n)
		}
		return uint64(n), nil
	case int32:
		if n < 0 {
			return 0, fmt.Errorf("negative integer value %d", n)
		}
		return uint64(n), nil
	case int64:
		if n < 0 {
			return 0, fmt.Errorf("negative integer value %d", n)
		}
		return uint64(n), nil
	case uint:
		return uint64(n), nil
	case uint8:
		return uint64(n), nil
	case uint16:
		return uint64(n), nil
	case uint32:
		return uint64(n), nil
	case uint64:
		return n, nil
	default:
		return 0, fmt.Errorf("unsupported unsigned value type %T", v)
	}
}
