package config

import (
	"log"
	"os"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		HTTPPort int `yaml:"http_port"`
	} `yaml:"server"`
	Worker struct {
		ConcurrencyMultiplier int `yaml:"concurrency_multiplier"`
	} `yaml:"worker"`
	Models struct {
		Gemini     ProviderConfig `yaml:"gemini"`
		OpenRouter ProviderConfig `yaml:"openrouter"`
	} `yaml:"models"`
}

type ProviderConfig struct {
	Codes []string `yaml:"codes"`
}

func Load() *Config {
	path := os.Getenv("SYNAPSE_CONFIG_PATH")
	if path == "" {
		path = "config.yaml"
	}

	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("Config file changed: %s", e.Name)
	})

	return getConfig()
}

func getConfig() *Config {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Printf("Unable to decode into struct: %v", err)
		return nil
	}
	return &cfg
}
