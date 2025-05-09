package context

import (
	"math"
	"math/rand"
	"strings"
	"time"
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

type RequestContext struct {
	RequestId  string
	StartTime  time.Time
	Host       string
	ClientAddr string // Applied http proxy header
	LogPrefix  string
}

func NewRequestContext(host, clientAddr string) *RequestContext {
	return &RequestContext{
		RequestId:  generateRequestId(8),
		StartTime:  time.Now(),
		Host:       host,
		ClientAddr: clientAddr,
	}
}
