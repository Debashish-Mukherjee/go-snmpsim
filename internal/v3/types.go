package v3

import (
	"fmt"
	"strings"

	"github.com/gosnmp/gosnmp"
)

type AuthProtocol string

const (
	AuthNone   AuthProtocol = ""
	AuthMD5    AuthProtocol = "MD5"
	AuthSHA1   AuthProtocol = "SHA1"
	AuthSHA224 AuthProtocol = "SHA224"
	AuthSHA256 AuthProtocol = "SHA256"
	AuthSHA384 AuthProtocol = "SHA384"
	AuthSHA512 AuthProtocol = "SHA512"
)

type PrivProtocol string

const (
	PrivNone   PrivProtocol = ""
	PrivDES    PrivProtocol = "DES"
	Priv3DES   PrivProtocol = "3DES"
	PrivAES128 PrivProtocol = "AES128"
	PrivAES192 PrivProtocol = "AES192"
	PrivAES256 PrivProtocol = "AES256"
)

type Config struct {
	Enabled bool
	EngineID string
	Username string

	Auth AuthProtocol
	AuthKey string

	Priv PrivProtocol
	PrivKey string
}

func (c Config) SecurityLevel() gosnmp.SnmpV3MsgFlags {
	if c.Auth == AuthNone {
		return gosnmp.NoAuthNoPriv
	}
	if c.Priv == PrivNone {
		return gosnmp.AuthNoPriv
	}
	return gosnmp.AuthPriv
}

func (c Config) ToGoSNMPAuth() gosnmp.SnmpV3AuthProtocol {
	switch strings.ToUpper(string(c.Auth)) {
	case string(AuthMD5):
		return gosnmp.MD5
	case string(AuthSHA1):
		return gosnmp.SHA
	case string(AuthSHA224):
		return gosnmp.SHA224
	case string(AuthSHA256):
		return gosnmp.SHA256
	case string(AuthSHA384):
		return gosnmp.SHA384
	case string(AuthSHA512):
		return gosnmp.SHA512
	default:
		return gosnmp.NoAuth
	}
}

func (c Config) ToGoSNMPPriv() gosnmp.SnmpV3PrivProtocol {
	switch strings.ToUpper(string(c.Priv)) {
	case string(PrivDES):
		return gosnmp.DES
	case string(PrivAES128):
		return gosnmp.AES
	case string(PrivAES192):
		return gosnmp.AES192
	case string(PrivAES256):
		return gosnmp.AES256
	default:
		return gosnmp.NoPriv
	}
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Username == "" {
		return fmt.Errorf("snmpv3 username is required when v3 is enabled")
	}
	if c.Auth != AuthNone && c.AuthKey == "" {
		return fmt.Errorf("snmpv3 auth key is required for auth protocols")
	}
	if c.Priv != PrivNone {
		if c.Auth == AuthNone {
			return fmt.Errorf("privacy protocol requires auth protocol")
		}
		if c.PrivKey == "" {
			return fmt.Errorf("snmpv3 priv key is required for priv protocols")
		}
	}
	if strings.EqualFold(string(c.Priv), string(Priv3DES)) {
		// crypto helpers support 3DES, but gosnmp wire path does not.
		return fmt.Errorf("snmpv3 3DES is not supported by gosnmp wire codec; use DES/AES128/AES192/AES256")
	}
	return nil
}

func (c Config) BuildUSM(boots, engineTime uint32) *gosnmp.UsmSecurityParameters {
	return &gosnmp.UsmSecurityParameters{
		AuthoritativeEngineID:    c.EngineID,
		AuthoritativeEngineBoots: boots,
		AuthoritativeEngineTime:  engineTime,
		UserName:                 c.Username,
		AuthenticationProtocol:   c.ToGoSNMPAuth(),
		PrivacyProtocol:          c.ToGoSNMPPriv(),
		AuthenticationPassphrase: c.AuthKey,
		PrivacyPassphrase:        c.PrivKey,
	}
}
