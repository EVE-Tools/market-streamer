package scheduler

import (
	"math/rand"
	"sync"
	"time"

	"github.com/EVE-Tools/market-streamer/lib/locations/regions"
	"github.com/EVE-Tools/market-streamer/lib/scraper"
	"github.com/Sirupsen/logrus"
)

var upstream chan<- []byte

// regionID -> Update time
var regionUpdateSchedule = struct {
	sync.RWMutex
	store map[int64]time.Time
}{store: make(map[int64]time.Time)}

// Initialize initializes the market and region update scheduling
func Initialize(emdr chan<- []byte) {
	upstream = emdr
	regionIDs := regions.GetMarketRegions()

	regionUpdateSchedule.Lock()
	for _, regionID := range regionIDs {
		regionUpdateSchedule.store[regionID] = time.Now().Add(time.Second * time.Duration(rand.Intn(300)))
	}
	regionUpdateSchedule.Unlock()

	updateRegions()
	go scheduleRegionUpdate()
	go scheduleMarketUpdate()
}

// ScheduleRegion schedules the regionID for update at a specific time
func ScheduleRegion(regionID int64, timestamp time.Time) {
	regionUpdateSchedule.Lock()
	regionUpdateSchedule.store[regionID] = timestamp
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
	regionUpdateSchedule.store = make(map[int64]time.Time)

	for _, regionID := range regionIDs {
		// Transfer regions from old map or add new entries
		if oldTimestamp, ok := oldMap[regionID]; ok {
			regionUpdateSchedule.store[regionID] = oldTimestamp
		} else {
			regionUpdateSchedule.store[regionID] = time.Now().Add(time.Second * time.Duration(rand.Int63n(300)))
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
	for regionID, timestamp := range regionUpdateSchedule.store {
		if timestamp.Before(time.Now()) {
			// Update again in 10 minutes if not re-scheduled by itself
			regionUpdateSchedule.store[regionID] = time.Now().Add(time.Second * 600)
			go func(regionID int64) {
				payload, runAgain, err := scraper.ScrapeMarket(regionID)
				if err != nil {
					logrus.WithError(err).Error("Failed to scrape market.")
					return
				}
				upstream <- payload
				ScheduleRegion(regionID, *runAgain)
			}(regionID)
		}
	}
	regionUpdateSchedule.Unlock()
}
