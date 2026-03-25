package config

import (
	"log"
	"os"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		HTTPPort int `mapstructure:"http_port"`
	} `mapstructure:"server"`
	Worker struct {
		ConcurrencyMultiplier int `mapstructure:"concurrency_multiplier"`
	} `mapstructure:"worker"`
	Models struct {
		Gemini     ProviderConfig `mapstructure:"gemini"`
		OpenRouter ProviderConfig `mapstructure:"openrouter"`
	} `mapstructure:"models"`
}

type ProviderConfig struct {
	Codes []string `mapstructure:"codes"`
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

	return getConfig()
}

func (c *Config) OnUpdate(fn func(*Config)) {
	viper.OnConfigChange(func(e fsnotify.Event) {
		if updated := getConfig(); updated != nil {
			fn(updated)
		}
	})
}

func getConfig() *Config {
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		log.Printf("Unable to decode into struct: %v", err)
		return nil
	}
	return &cfg
}
