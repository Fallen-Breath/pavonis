package utils

import (
	"github.com/sirupsen/logrus"
	"log"
)

func CreateLogrusStdLogger(level logrus.Level) (*log.Logger, func()) {
	writer := logrus.StandardLogger().WriterLevel(level)
	logger := log.New(writer, "", 0)
	closer := func() {
		_ = writer.Close()
	}
	return logger, closer
}
