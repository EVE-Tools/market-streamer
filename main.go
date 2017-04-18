package main

import (
	"runtime"

	"github.com/EVE-Tools/market-streamer/lib/emdr"
	"github.com/EVE-Tools/market-streamer/lib/locations/citadels"
	"github.com/EVE-Tools/market-streamer/lib/locations/locationCache"
	"github.com/EVE-Tools/market-streamer/lib/locations/regions"
	"github.com/EVE-Tools/market-streamer/lib/marketTypes"
	"github.com/EVE-Tools/market-streamer/lib/scheduler"
	"github.com/EVE-Tools/market-streamer/lib/scraper"
	"github.com/Sirupsen/logrus"
	"github.com/kelseyhightower/envconfig"
)

// Config holds the application's configuration info from the environment.
type Config struct {
	LogLevel           string `default:"info" envconfig:"log_level"`
	ClientID           string `required:"true" envconfig:"client_id"`
	SecretKey          string `required:"true" envconfig:"secret_key"`
	RefreshToken       string `required:"true" envconfig:"refresh_token"`
	ZMQBindEndpoint    string `default:"tcp://127.0.0.1:8050" envconfig:"zmq_bind_endpoint"`
	LocationServiceURL string `default:"https://element-43.com/api/static-data/v1/location/" envconfig:"location_service_url"`
}

// Stores main configuration
var config Config

func main() {
	// Load config and connect to queues
	loadConfig()
	emdr := emdr.Initialize()
	locationCache.Initialize()
	regions.Initialize()
	citadels.Initialize()
	marketTypes.Initialize()
	scheduler.Initialize(emdr)
	scraper.Initialize(config.ClientID, config.SecretKey, config.RefreshToken)
	logrus.Debug("Done.")

	// Terminate this goroutine, crash if all other goroutines exited
	runtime.Goexit()
}

// Load configuration from environment and compile regexps
func loadConfig() {
	envconfig.MustProcess("MARKET_STREAMER", &config)

	logLevel, err := logrus.ParseLevel(config.LogLevel)
	if err != nil {
		panic(err)
	}
	logrus.SetLevel(logLevel)
	logrus.Debugf("Config: %q", config)
}
