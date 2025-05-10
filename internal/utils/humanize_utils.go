package utils

import "github.com/dustin/go-humanize"

func PrettyByteSize(byteSize int64) string {
	if byteSize < 0 {
		return "-" + humanize.IBytes(uint64(-byteSize))
	} else {
		return humanize.IBytes(uint64(byteSize))
	}
}
