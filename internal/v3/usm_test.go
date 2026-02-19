package v3

import "testing"

func TestUSMEncodeDecode(t *testing.T) {
	in := SecurityParams{
		AuthoritativeEngineID:    []byte{0x80, 0x00, 0x1f, 0x88, 0x01},
		AuthoritativeEngineBoots: 7,
		AuthoritativeEngineTime:  42,
		UserName:                 "simuser",
		AuthenticationParameters: []byte{1, 2, 3, 4},
		PrivacyParameters:        []byte{9, 8, 7, 6},
	}

	encoded, err := EncodeUSMSecurityParameters(in)
	if err != nil {
		t.Fatalf("EncodeUSMSecurityParameters: %v", err)
	}

	out, err := DecodeUSMSecurityParameters(encoded)
	if err != nil {
		t.Fatalf("DecodeUSMSecurityParameters: %v", err)
	}

	if out.UserName != in.UserName || out.AuthoritativeEngineBoots != in.AuthoritativeEngineBoots || out.AuthoritativeEngineTime != in.AuthoritativeEngineTime {
		t.Fatalf("decoded value mismatch: %+v", out)
	}
}
