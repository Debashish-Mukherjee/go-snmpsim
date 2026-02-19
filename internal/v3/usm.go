package v3

import (
	"encoding/asn1"
	"fmt"

	"github.com/gosnmp/gosnmp"
)

const (
	USMStatsNotInTimeWindowOID = ".1.3.6.1.6.3.15.1.1.2.0"
	USMStatsUnknownEngineIDOID = ".1.3.6.1.6.3.15.1.1.4.0"
	USMStatsWrongDigestOID     = ".1.3.6.1.6.3.15.1.1.5.0"
)

type SecurityParams struct {
	AuthoritativeEngineID    []byte
	AuthoritativeEngineBoots int
	AuthoritativeEngineTime  int
	UserName                 string
	AuthenticationParameters []byte
	PrivacyParameters        []byte
}

func EncodeUSMSecurityParameters(params SecurityParams) ([]byte, error) {
	raw, err := asn1.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("encode usm params: %w", err)
	}
	return raw, nil
}

func DecodeUSMSecurityParameters(data []byte) (SecurityParams, error) {
	var params SecurityParams
	_, err := asn1.Unmarshal(data, &params)
	if err != nil {
		return SecurityParams{}, fmt.Errorf("decode usm params: %w", err)
	}
	return params, nil
}

func BuildUSMReportVar(oid string) gosnmp.SnmpPDU {
	return gosnmp.SnmpPDU{
		Name:  oid,
		Type:  gosnmp.Counter32,
		Value: uint(1),
	}
}
