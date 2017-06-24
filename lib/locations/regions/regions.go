package regions

import (
	"net/http"
	"time"

	"github.com/EVE-Tools/goesi"
	"github.com/Sirupsen/logrus"
)

var esiClient goesi.APIClient
var regionIDs []int64

// Initialize initializes the region updates
func Initialize() {
	httpClient := &http.Client{
		Timeout: time.Duration(time.Second * 10),
	}
	esiClient = *goesi.NewAPIClient(httpClient, "Element43/market-streamer (element-43.com")
	updateRegions()
	go scheduleRegionUpdate()
}

// GetMarketRegions returns all regionIDs with a market
func GetMarketRegions() []int64 {
	return regionIDs
}

// Keep ticking in own goroutine and spawn worker tasks.
func scheduleRegionUpdate() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		<-ticker.C
		go updateRegions()
	}
}

// Updates list of regionIDs
func updateRegions() {
	logrus.Debug("Updating regions.")

	regions, err := getMarketRegions()
	if err != nil {
		logrus.WithError(err).Error("Could not get regionIDs from ESI!")
		return
	}

	regionIDs = regions
	logrus.Debug("Region update done.")
}

// Get all regionIDs from ESI
func getRegionIDs() ([]int32, error) {
	regionIDs, _, err := esiClient.V1.UniverseApi.GetUniverseRegions(nil)
	if err != nil {
		return nil, err
	}

	return regionIDs, nil
}

// Get all regions with a market (filter WH)
func getMarketRegions() ([]int64, error) {
	regionIDs, err := getRegionIDs()
	if err != nil {
		return nil, err
	}

	var marketRegionIDs []int64
	for _, regionID := range regionIDs {
		// Filter invalid regions
		if regionID < 11000000 && regionID != 10000004 && regionID != 10000019 {
			marketRegionIDs = append(marketRegionIDs, int64(regionID))
		}
	}

	return marketRegionIDs, nil
}
