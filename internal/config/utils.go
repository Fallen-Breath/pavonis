package config

import (
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"os"
)

const envVarConfigContent = "PAVONIS_CONFIG"

func LoadConfigOrDie(configPath string) *Config {
	var configBuf []byte
	if configData, ok := os.LookupEnv(envVarConfigContent); ok {
		log.Infof("Loading config from envvar %s", envVarConfigContent)
		configBuf = []byte(configData)
	} else {
		buf, err := os.ReadFile(configPath)
		if err != nil {
			log.Fatalf("Failed to read config file %s: %v", configPath, err)
		}
		configBuf = buf
	}

	cfg := Config{}
	if err := yaml.Unmarshal(configBuf, &cfg); err != nil {
		log.Fatalf("Failed to parse yaml from config file %s: %v", configPath, err)
	}
	if err := cfg.Init(); err != nil {
		log.Fatalf("Config intialization failed: %v", err)
	}

	cfg.Dump()
	return &cfg
}
