package main

import (
	"net/http"
	"runtime"
	"time"

	"github.com/EVE-Tools/element43/go/lib/transport"
	"github.com/EVE-Tools/market-streamer/lib/emdr"
	"github.com/EVE-Tools/market-streamer/lib/locations/citadels"
	"github.com/EVE-Tools/market-streamer/lib/locations/locationCache"
	"github.com/EVE-Tools/market-streamer/lib/locations/regions"
	"github.com/EVE-Tools/market-streamer/lib/marketTypes"
	"github.com/EVE-Tools/market-streamer/lib/scheduler"
	"github.com/EVE-Tools/market-streamer/lib/scraper"
	"github.com/antihax/goesi"
	"github.com/kelseyhightower/envconfig"
	"github.com/sirupsen/logrus"
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
	const userAgent string = "Element43/market-streamer (element-43.com)"
	const timeout time.Duration = time.Duration(time.Second * 10)

	httpClient := &http.Client{
		Timeout:   timeout,
		Transport: transport.NewTransport(userAgent),
	}

	httpClientESI := &http.Client{
		Timeout:   timeout,
		Transport: transport.NewESITransport(userAgent, timeout),
	}

	esiClient := goesi.NewAPIClient(httpClientESI, userAgent)

	// Load config and connect to queues
	loadConfig()
	emdr := emdr.Initialize(config.ZMQBindEndpoint)
	locationCache.Initialize(config.LocationServiceURL, httpClient)
	regions.Initialize(esiClient)
	citadels.Initialize(esiClient)
	marketTypes.Initialize(esiClient)
	scheduler.Initialize(emdr)
	scraper.Initialize(config.ClientID, config.SecretKey, config.RefreshToken, httpClientESI, esiClient)
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
