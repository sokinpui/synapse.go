package config

import (
	"log"

	"github.com/kelseyhightower/envconfig"
)

// Settings holds the application configuration.
type Settings struct {
	RedisHost     string `envconfig:"SYNAPSE_REDIS_HOST" default:"localhost"`
	RedisPort     int    `envconfig:"SYNAPSE_REDIS_PORT" default:"6379"`
	RedisDB       int    `envconfig:"SYNAPSE_REDIS_DB" default:"0"`
	RedisPassword string `envconfig:"SYNAPSE_REDIS_PASSWORD" default:""`
	GRPCPort      int    `envconfig:"SYNAPSE_GRPC_PORT" default:"50051"`
}

// Load reads configuration from environment variables.
func Load() *Settings {
	var s Settings
	err := envconfig.Process("synapse", &s)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	return &s
}
