package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const envVarConfigContent = "PAVONIS_CONFIG"

func LoadConfigOrDie(configPath string) *Config {
	var configBuf []byte
	configSource := "config file " + configPath
	if configData, ok := os.LookupEnv(envVarConfigContent); ok {
		log.Infof("Loading config from envvar %s", envVarConfigContent)
		configBuf = []byte(configData)
		configSource = "envvar " + envVarConfigContent
	} else {
		buf, err := os.ReadFile(configPath)
		if err != nil {
			log.Fatalf("Failed to read config file %s: %v", configPath, err)
		}
		configBuf = buf
	}

	cfg := Config{}
	if err := unmarshalStrict(configBuf, &cfg); err != nil {
		log.Fatalf("Failed to parse yaml from %s: %v", configSource, err)
	}
	if err := cfg.Init(); err != nil {
		log.Fatalf("Config intialization failed: %v", err)
	}

	cfg.Dump()
	return &cfg
}

func LoadUsersFile(path string) ([]*User, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read users file: %v", err)
	}
	usersFile := UsersFile{}
	if err := unmarshalStrict(buf, &usersFile); err != nil {
		return nil, fmt.Errorf("failed to parse users file: %v", err)
	}
	for userIdx, user := range usersFile.Users {
		if err := ValidateUser(user); err != nil {
			return nil, fmt.Errorf("failed to validate user[%d]: %v", userIdx, err)
		}
	}
	return usersFile.Users, nil
}

// unmarshalStrict rejects unknown keys, and treats empty input as an empty document
func unmarshalStrict(data []byte, out any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func cleanNil[T any](slice []*T) []*T {
	if slice == nil {
		return []*T{}
	}

	cleaned := make([]*T, 0, len(slice))
	for _, item := range slice {
		if item != nil {
			cleaned = append(cleaned, item)
		}
	}

	return cleaned
}
