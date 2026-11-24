package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"go.yaml.in/yaml/v3"
)

const envVarConfigContent = "PAVONIS_CONFIG"

func LoadConfigOrDie(configPath string) *Config {
	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}
	cfg.Dump()
	return cfg
}

func loadConfig(configPath string) (*Config, error) {
	var configBuf []byte
	configSource := "config file " + configPath
	referenceBaseDir := filepath.Dir(configPath)
	if configData, ok := os.LookupEnv(envVarConfigContent); ok {
		log.Infof("Loading config from envvar %s", envVarConfigContent)
		configBuf = []byte(configData)
		configSource = "envvar " + envVarConfigContent
		referenceBaseDir = "" // working directory
	} else {
		buf, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("Failed to read config file %s: %v", configPath, err)
		}
		configBuf = buf
	}

	cfg := Config{}
	if err := unmarshalStrict(configBuf, &cfg, configSource); err != nil {
		return nil, fmt.Errorf("Failed to parse yaml from %s: %v", configSource, err)
	}
	if err := cfg.resolveReferences(referenceBaseDir); err != nil {
		return nil, fmt.Errorf("Failed to resolve references: %v", err)
	}
	if err := cfg.Init(); err != nil {
		return nil, fmt.Errorf("Config intialization failed: %v", err)
	}
	return &cfg, nil
}

// resolveReferences replaces references to other files with the content of those files
func (cfg *Config) resolveReferences(baseDir string) error {
	for siteIdx, siteCfg := range cfg.Sites {
		if siteCfg == nil || siteCfg.file == "" {
			continue
		}
		path := siteCfg.file
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		log.Infof("Loading site file %s", path)
		buf, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("[site%d] failed to read site file %s: %v", siteIdx, path, err)
		}
		var loaded *SiteConfig
		if err := unmarshalStrict(buf, &loaded, "site file "+path); err != nil {
			return fmt.Errorf("[site%d] failed to parse site file %s: %v", siteIdx, path, err)
		}
		if loaded == nil {
			return fmt.Errorf("[site%d] site file %s is empty", siteIdx, path)
		}
		if loaded.file != "" {
			return fmt.Errorf("[site%d] site file %s cannot reference another site file", siteIdx, path)
		}
		cfg.Sites[siteIdx] = loaded
	}
	return nil
}

func LoadUsersFile(path string) ([]*User, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read users file: %v", err)
	}
	usersFile := UsersFile{}
	if err := unmarshalStrict(buf, &usersFile, "users file "+path); err != nil {
		return nil, fmt.Errorf("failed to parse users file: %v", err)
	}
	for userIdx, user := range usersFile.Users {
		if err := ValidateUser(user); err != nil {
			return nil, fmt.Errorf("failed to validate user[%d]: %v", userIdx, err)
		}
	}
	return usersFile.Users, nil
}

func unmarshalStrict(data []byte, out any, source string) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(out); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		log.Warnf("Ignoring YAML content after the first document in %s; only the first document is used", source)
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
