package config

import (
	"github.com/spf13/viper"
)

const (
	RestServerPort     = "REST_SERVER_PORT"
	GRPCServerPort     = "GRPC_SERVER_PORT"
	HealthCheckAddress = "HEALTH_CHECK_ADDRESS"
	DbRootDir          = "DB_ROOT_DIR"
	DbUpdateURL        = "DB_UPDATE_URL"
	SqliteVersion      = "SQLITE_VERSION"
)

type Config struct {
	RestServerPort     int
	GRPCServerPort     int
	HealthCheckAddress string
	DbRootDir          string
	DbUpdateURL        string
	SqliteVersion      string
}

func LoadConfig() *Config {
	config := &Config{}

	config.RestServerPort = viper.GetInt(RestServerPort)
	config.GRPCServerPort = viper.GetInt(GRPCServerPort)
	config.HealthCheckAddress = viper.GetString(HealthCheckAddress)
	config.DbRootDir = viper.GetString(DbRootDir)
	config.DbUpdateURL = viper.GetString(DbUpdateURL)
	config.SqliteVersion = viper.GetString(SqliteVersion)

	return config
}
