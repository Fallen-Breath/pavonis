package context

import (
	"math"
	"math/rand"
	"strings"
)

const griCharsets = "abcdefghijklmnopqrstuvwxyz0123456789"
const griCharsetsLen = int64(len(griCharsets))

func generateRequestId(genLen int) string {
	num := rand.Int63n(int64(math.Pow(float64(griCharsetsLen), float64(genLen))))

	var result strings.Builder
	for i := 0; i < genLen; i++ {
		result.WriteByte(griCharsets[num%griCharsetsLen])
		num = num / griCharsetsLen
	}

	return result.String()
}

type HttpContext struct {
	RequestId  string
	ClientAddr string // Applied http proxy header
}

func NewHttpContext(clientAddr string) *HttpContext {
	return &HttpContext{
		RequestId:  generateRequestId(10),
		ClientAddr: clientAddr,
	}
}
