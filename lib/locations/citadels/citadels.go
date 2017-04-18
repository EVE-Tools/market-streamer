package citadels

import (
	"sync"
	"time"

	"github.com/EVE-Tools/market-streamer/lib/locations/locationCache"
	"github.com/Sirupsen/logrus"
	"github.com/antihax/goesi"
)

var esiClient goesi.APIClient

var citadelsInRegion = struct {
	sync.RWMutex
	store map[int64][]int64
}{store: make(map[int64][]int64)}

// Initialize initializes the citadel updates
func Initialize() {
	esiClient = *goesi.NewAPIClient(nil, "Element43/market-streamer (element-43.com")
	updateCitadels()
	go scheduleCitadelUpdate()
}

// GetCitadelsInRegions returns all public citadel's IDs in a given list of regionIDs
func GetCitadelsInRegions(regionIDs []int64) []int64 {
	var ids []int64

	for _, regionID := range regionIDs {
		ids = append(ids, GetCitadelsInRegion(regionID)...)
	}

	return ids
}

// GetCitadelsInRegion returns all public citadel's IDs for a given regionID
func GetCitadelsInRegion(regionID int64) []int64 {
	citadelsInRegion.RLock()
	ids := citadelsInRegion.store[regionID]
	citadelsInRegion.RUnlock()
	return ids
}

// Keep ticking in own goroutine and spawn worker tasks.
func scheduleCitadelUpdate() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		<-ticker.C
		go updateCitadels()
	}
}

// Updates list of regionIDs
func updateCitadels() {
	logrus.Debug("Updating citadels.")

	citadelIDs, err := getCitadelIDs()
	if err != nil {
		logrus.WithError(err).Error("Could not get public citadels from ESI!")
		return
	}

	citadels, err := locationCache.GetLocations(citadelIDs)
	if err != nil {
		logrus.WithError(err).Error("Could not get citadels from location API!")
		return
	}

	citadelsInRegion.Lock()
	citadelsInRegion.store = make(map[int64][]int64)
	for _, citadel := range citadels {
		citadelsInRegion.store[citadel.Region.ID] = append(citadelsInRegion.store[citadel.Region.ID], citadel.Station.ID)
	}
	citadelsInRegion.Unlock()

	logrus.Debug("Citadel update done.")
}

// Get all citadels from ESI
func getCitadelIDs() ([]int64, error) {
	citadelIDs, _, err := esiClient.V1.UniverseApi.GetUniverseStructures(nil)
	if err != nil {
		return nil, err
	}

	return citadelIDs, nil
}
