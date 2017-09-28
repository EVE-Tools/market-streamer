package citadels

import (
	"sync"
	"time"

	"github.com/EVE-Tools/market-streamer/lib/locations/locationCache"
	"github.com/antihax/goesi"
	"github.com/sirupsen/logrus"
)

var esiClient *goesi.APIClient

// Maps a regionID to citadelIDs in that region
var citadelsInRegion = struct {
	sync.RWMutex
	store map[int64][]int64
}{store: make(map[int64][]int64)}

// List of citadels which returned 403s, wiped every twelve hours
var citadelBlacklist = struct {
	sync.RWMutex
	store map[int64]bool
}{store: make(map[int64]bool)}

// Initialize initializes the citadel updates
func Initialize(client *goesi.APIClient) {
	esiClient = client

	updateCitadels()
	go scheduleCitadelUpdate()
	go scheduleBlacklistWipe()
}

// GetCitadelsInRegions returns all public citadel's IDs in a given list of regionIDs (excluding blacklisted ones)
func GetCitadelsInRegions(regionIDs []int64) []int64 {
	var ids []int64

	for _, regionID := range regionIDs {
		ids = append(ids, GetCitadelsInRegion(regionID)...)
	}

	return ids
}

// GetCitadelsInRegion returns all public citadel's IDs for a given regionID (excluding blacklisted ones)
func GetCitadelsInRegion(regionID int64) []int64 {
	var ids []int64
	citadelsInRegion.RLock()
	citadelBlacklist.RLock()

	for _, citadelID := range citadelsInRegion.store[regionID] {
		if !citadelBlacklist.store[citadelID] {
			ids = append(ids, citadelID)
		}
	}

	citadelBlacklist.RUnlock()
	citadelsInRegion.RUnlock()
	return ids
}

// BlacklistCitadel blacklists a citadel (e.g. if we don't have access)
func BlacklistCitadel(id int64) {
	citadelBlacklist.Lock()
	citadelBlacklist.store[id] = true
	citadelBlacklist.Unlock()
}

// Schdeule and perform citadel update
func scheduleCitadelUpdate() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()
	for {
		<-ticker.C
		go updateCitadels()
	}
}

// Schedule and perform blacklist wipe
func scheduleBlacklistWipe() {
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()
	for {
		<-ticker.C
		go wipeBlacklist()
	}
}

// Updates list of citadelIDs
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

// Wipes the blacklist
func wipeBlacklist() {
	logrus.Debug("Wiping citadel blacklist.")

	citadelBlacklist.Lock()
	citadelBlacklist.store = make(map[int64]bool)
	citadelBlacklist.Unlock()

	logrus.Debug("Done wiping citadel blacklist.")
}

// Get all citadels from ESI
func getCitadelIDs() ([]int64, error) {
	citadelIDs, _, err := esiClient.ESI.UniverseApi.GetUniverseStructures(nil)
	if err != nil {
		return nil, err
	}

	return citadelIDs, nil
}
