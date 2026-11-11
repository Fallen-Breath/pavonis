package hfproxy

import (
	"github.com/Fallen-Breath/pavonis/internal/utils"
	"regexp"
)

var hfUrl = utils.MustParseUrl("https://huggingface.co")

// Requests under this prefix are forwarded to Hugging Face file storage hosts, as "/.hf-storage/{host}/{path}"
const storagePathPrefix = "/.hf-storage"

var storageHostPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^[a-z0-9-]+\.xethub\.hf\.co$`),
	regexp.MustCompile(`^([a-z0-9-]+\.)+cdn\.hf\.co$`),
}

func isStorageHost(host string) bool {
	for _, pattern := range storageHostPatterns {
		if pattern.MatchString(host) {
			return true
		}
	}
	return false
}
