package scheduler

import (
	"math/rand"
	"sync"
	"time"

	"github.com/EVE-Tools/market-streamer/lib/locations/regions"
	"github.com/EVE-Tools/market-streamer/lib/scraper"
	"github.com/sirupsen/logrus"
)

var upstream chan<- []byte

// regionID -> Update time, last modified time
var regionUpdateSchedule = struct {
	sync.RWMutex
	store map[int64]scheduleEntry
}{store: make(map[int64]scheduleEntry)}

type scheduleEntry struct {
	runAgain     time.Time
	lastModified time.Time
}

// Initialize initializes the market and region update scheduling
func Initialize(emdr chan<- []byte) {
	upstream = emdr
	regionIDs := regions.GetMarketRegions()

	regionUpdateSchedule.Lock()
	for _, regionID := range regionIDs {
		randomOffset := time.Now().Add(time.Second * time.Duration(rand.Intn(300)))
		regionUpdateSchedule.store[regionID] = scheduleEntry{
			runAgain:     randomOffset,
			lastModified: time.Time{},
		}
	}
	regionUpdateSchedule.Unlock()

	updateRegions()
	go scheduleRegionUpdate()
	go scheduleMarketUpdate()
}

// ScheduleRegion schedules the regionID for update at a specific time
func ScheduleRegion(regionID int64, runAgain time.Time, lastModified time.Time) {
	regionUpdateSchedule.Lock()
	cacheEntry := regionUpdateSchedule.store[regionID]
	cacheEntry.runAgain = runAgain
	cacheEntry.lastModified = lastModified
	regionUpdateSchedule.store[regionID] = cacheEntry
	regionUpdateSchedule.Unlock()
}

// Schedules region updates
func scheduleRegionUpdate() {
	ticker := time.NewTicker(time.Minute * 5)
	defer ticker.Stop()
	for {
		<-ticker.C
		go updateRegions()
	}
}

// Updates regions
func updateRegions() {
	regionIDs := regions.GetMarketRegions()

	regionUpdateSchedule.Lock()
	oldMap := regionUpdateSchedule.store
	regionUpdateSchedule.store = make(map[int64]scheduleEntry)

	for _, regionID := range regionIDs {
		// Transfer regions from old map or add new entries
		if oldEntry, ok := oldMap[regionID]; ok {
			regionUpdateSchedule.store[regionID] = oldEntry
		} else {
			randomOffset := time.Now().Add(time.Second * time.Duration(rand.Intn(300)))
			regionUpdateSchedule.store[regionID] = scheduleEntry{
				runAgain:     randomOffset,
				lastModified: time.Time{},
			}
		}
	}
	regionUpdateSchedule.Unlock()
}

// Schedules market updates
func scheduleMarketUpdate() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		<-ticker.C
		go updateMarkets()
	}
}

// Updates markets
func updateMarkets() {
	regionUpdateSchedule.Lock()
	for regionID, entry := range regionUpdateSchedule.store {
		if entry.runAgain.Before(time.Now()) {
			// Update again in 10 minutes if not re-scheduled by itself
			entry.runAgain = time.Now().Add(time.Second * 600)
			regionUpdateSchedule.store[regionID] = entry

			go func(regionID int64, lastModified time.Time) {
				payload, runAgain, newLastModified, err := scraper.ScrapeMarket(regionID, lastModified)
				if err != nil {
					logrus.WithError(err).Error("Failed to scrape market.")
					return
				}

				if payload != nil {
					// Payload could be nil when there was no modifiaction of the market
					upstream <- payload
				}
				ScheduleRegion(regionID, *runAgain, *newLastModified)
			}(regionID, entry.lastModified)
		}
	}
	regionUpdateSchedule.Unlock()
}
