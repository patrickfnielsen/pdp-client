package config

import (
	"os"
	"strconv"

	_ "github.com/joho/godotenv/autoload"

	"log/slog"
)

var VERSION = 0.1
var Enviroment = GetEnv("PDP_ENV", "PROD")
var LogLevel = slog.Level(GetEnv("PDP_LOG_LEVEL", 0))

var PolicyRepository = GetEnv("PDP_REPOSITORY", "")
var PolicyRepositoryBranch = GetEnv("PDP_REPOSITORY_BRANCH", "main")
var PolicyRepositoryKey = GetEnv("PDP_REPOSITORY_KEY", "")

var PolicyServerLogConsole = GetEnv("PDP_LOG_CONSOLE", true)
var PolicyServerLogHTTP = GetEnv("PDP_LOG_HTTP", false)

var PolicyLogServer = GetEnv("PDP_LOG_HTTP_SERVER", "")
var PolicyLogServerEndpoint = GetEnv("PDP_LOG_HTTP_SERVER_ENDPOINT", "/api/v1/decision/logs")
var PolicyLogServerToken = GetEnv("PDP_LOG_HTTP_SERVER_TOKEN", "")
var PolicyLogServerTLS = GetEnv("PDP_LOG_HTTP_SERVER_TLS", true)

type EnvType interface {
	string | int | bool
}

func GetEnv[T EnvType](envName string, defaultValue T) T {
	value := os.Getenv(envName)
	if value == "" {
		return defaultValue
	}

	var ret any = defaultValue
	switch any(defaultValue).(type) {
	case string:
		ret = value
	case bool:
		i, err := strconv.ParseBool(value)
		if err == nil {
			ret = i
		}
	case int:
		i, err := strconv.Atoi(value)
		if err == nil {
			ret = i
		}
	}

	return ret.(T)
}
