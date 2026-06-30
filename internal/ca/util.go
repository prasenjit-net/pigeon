package ca

import (
	"encoding/base64"
	"encoding/binary"
)

func base64URLEncode(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func base64URLEncodeUint(v uint64) string {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, v)
	// strip leading zero bytes
	i := 0
	for i < len(buf)-1 && buf[i] == 0 {
		i++
	}
	return base64URLEncode(buf[i:])
}

func base64URLDecode(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(s)
}
