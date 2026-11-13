package crproxy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
)

const packedTokenIssuer = "pavonis"

// The public, fixed key only obfuscates tokens; it is not a security boundary.
// Reuse the block cipher, but create an independent CTR stream for every token.
var packedTokenCipher = func() cipher.Block {
	block, err := aes.NewCipher([]byte("pavonis-cr-token"))
	if err != nil {
		panic(err)
	}
	return block
}()

// The token issued by Pavonis when auth is enabled. Its contents are obfuscated,
// not authenticated; the Pavonis credentials inside are checked on every request.
type packedToken struct {
	Issuer   string `json:"issuer"`
	User     string `json:"user"`
	Password string `json:"password"`
	// The Authorization header value sent to the upstream, e.g. "Bearer xxx" or "Basic xxx". Empty for none
	UpstreamAuthorization string `json:"upstream_authorization"`
}

func packToken(user, password, upstreamAuthorization string) string {
	data, _ := json.Marshal(packedToken{
		Issuer:                packedTokenIssuer,
		User:                  user,
		Password:              password,
		UpstreamAuthorization: upstreamAuthorization,
	})
	buf := make([]byte, aes.BlockSize+len(data))
	iv := buf[:aes.BlockSize]
	_, _ = rand.Read(iv) // crypto/rand.Read always succeeds or terminates the process.
	cipher.NewCTR(packedTokenCipher, iv).XORKeyStream(buf[aes.BlockSize:], data)
	return encodeTokenBase62(buf)
}

func unpackToken(s string) (*packedToken, bool) {
	buf, ok := decodeTokenBase62(s)
	if !ok || len(buf) <= aes.BlockSize {
		return nil, false
	}
	iv, data := buf[:aes.BlockSize], buf[aes.BlockSize:]
	cipher.NewCTR(packedTokenCipher, iv).XORKeyStream(data, data)
	var token packedToken
	if err := json.Unmarshal(data, &token); err != nil || token.Issuer != packedTokenIssuer {
		return nil, false
	}
	return &token, true
}

const tokenBase62Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Encode each 8-byte block as 11 base62 digits, with a shorter final block.
// Fixed widths preserve leading zero bytes and keep conversion linear in size.
var tokenBase62EncodedSizes = [...]int{0, 2, 3, 5, 6, 7, 9, 10, 11}

func encodeTokenBase62(data []byte) string {
	out := make([]byte, len(data)/8*11+tokenBase62EncodedSizes[len(data)%8])
	offset := 0
	for len(data) > 0 {
		n := min(len(data), 8)
		var value uint64
		for _, c := range data[:n] {
			value = value<<8 | uint64(c)
		}
		end := offset + tokenBase62EncodedSizes[n]
		for i := end - 1; i >= offset; i-- {
			out[i] = tokenBase62Alphabet[value%62]
			value /= 62
		}
		offset = end
		data = data[n:]
	}
	return string(out)
}

func decodeTokenBase62(s string) ([]byte, bool) {
	// A final block must have one of the lengths produced by the encoder.
	decodedSizes := [...]int{0, -1, 1, 2, -1, 3, 4, 5, -1, 6, 7, 8}
	tailSize := decodedSizes[len(s)%11]
	if tailSize < 0 {
		return nil, false
	}
	out := make([]byte, len(s)/11*8+tailSize)
	offset := 0
	for len(s) > 0 {
		n := min(len(s), 11)
		var value uint64
		for i := 0; i < n; i++ {
			var digit uint64
			switch c := s[i]; {
			case c >= '0' && c <= '9':
				digit = uint64(c - '0')
			case c >= 'A' && c <= 'Z':
				digit = uint64(c-'A') + 10
			case c >= 'a' && c <= 'z':
				digit = uint64(c-'a') + 36
			default:
				return nil, false
			}
			if value > (^uint64(0)-digit)/62 {
				return nil, false
			}
			value = value*62 + digit
		}
		byteCount := decodedSizes[n]
		if value>>(8*byteCount) != 0 {
			return nil, false
		}
		end := offset + byteCount
		for i := end - 1; i >= offset; i-- {
			out[i] = byte(value)
			value >>= 8
		}
		offset = end
		s = s[n:]
	}
	return out, true
}
