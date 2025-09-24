package config

import (
	"log"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Settings struct {
	RedisHost     string `envconfig:"SYNAPSE_REDIS_HOST" default:"localhost"`
	RedisPort     int    `envconfig:"SYNAPSE_REDIS_PORT" default:"6379"`
	RedisDB       int    `envconfig:"SYNAPSE_REDIS_DB" default:"0"`
	RedisPassword string `envconfig:"SYNAPSE_REDIS_PASSWORD" default:""`
	GRPCPort      int    `envconfig:"SYNAPSE_GRPC_PORT" default:"50051"`
}

// Load reads configuration from environment variables.
func Load() *Settings {
	// Attempt to load .env file.
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	var s Settings
	err := envconfig.Process("synapse", &s)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	return &s
}
