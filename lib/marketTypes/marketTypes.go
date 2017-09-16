package marketTypes

import (
	"net/http"
	"time"

	"github.com/antihax/goesi"
	"github.com/sirupsen/logrus"
)

var esiClient goesi.APIClient
var esiSemaphore = make(chan struct{}, 200)
var typeIDs []int64

// Initialize initializes the market type updates
func Initialize() {
	httpClient := &http.Client{
		Timeout: time.Duration(time.Second * 10),
	}
	esiClient = *goesi.NewAPIClient(httpClient, "Element43/market-streamer (element-43.com)")

	updateTypes()
	go scheduleTypeUpdate()
}

// GetMarketTypes returns all typeIDs with a market
func GetMarketTypes() []int64 {
	return typeIDs
}

// Keep ticking in own goroutine and spawn worker tasks.
func scheduleTypeUpdate() {
	ticker := time.NewTicker(2 * time.Hour)
	defer ticker.Stop()
	for {
		<-ticker.C
		go updateTypes()
	}
}

// Update type list
func updateTypes() {
	logrus.Debug("Updating market types.")

	types, err := getMarketTypes()
	if err != nil {
		logrus.WithError(err).Error("Failed to get market types!")
	} else {
		typeIDs = types
	}

	logrus.Debug("Market type update done.")
}

// Get all types on market
func getMarketTypes() ([]int64, error) {
	typeIDs, err := getTypeIDs()
	if err != nil {
		return nil, err
	}

	marketTypes := make(chan int64)
	nonMarketTypes := make(chan int64)
	failure := make(chan error)

	typesLeft := len(typeIDs)

	for _, id := range typeIDs {
		go checkIfMarketTypeAsyncRetry(id, marketTypes, nonMarketTypes, failure)
	}

	var marketTypeIDs []int64

	for typesLeft > 0 {
		select {
		case typeID := <-marketTypes:
			marketTypeIDs = append(marketTypeIDs, typeID)
		case <-nonMarketTypes:
		case err := <-failure:
			logrus.Warnf("Error fetching type from ESI: %s", err.Error())
		}

		typesLeft--
	}

	return marketTypeIDs, nil
}

// Get all typeIDs from ESI
// TODO: move to static-data RPC
func getTypeIDs() ([]int32, error) {
	var typeIDs []int32
	params := make(map[string]interface{})
	params["page"] = int32(1)

	typeResult, _, err := esiClient.V1.UniverseApi.GetUniverseTypes(params)
	if err != nil {
		return nil, err
	}

	typeIDs = append(typeIDs, typeResult...)

	for len(typeResult) > 0 {
		params["page"] = params["page"].(int32) + 1
		typeResult, _, err = esiClient.V1.UniverseApi.GetUniverseTypes(params)
		if err != nil {
			return nil, err
		}

		typeIDs = append(typeIDs, typeResult...)
	}

	return typeIDs, nil
}

// Async check if market type, retry 3 times
func checkIfMarketTypeAsyncRetry(typeID int32, marketTypes chan int64, nonMarketTypes chan int64, failure chan error) {
	var isMarketType bool
	var err error
	retries := 3

	for retries > 0 {
		isMarketType, err = checkIfMarketType(typeID)
		if err != nil {
			retries--
		} else {
			err = nil
			retries = 0
		}
	}

	if err != nil {
		failure <- err
		return
	}

	if isMarketType {
		marketTypes <- int64(typeID)
		return
	}

	nonMarketTypes <- int64(typeID)
}

// Check if type is market type
func checkIfMarketType(typeID int32) (bool, error) {
	esiSemaphore <- struct{}{}
	typeInfo, _, err := esiClient.V3.UniverseApi.GetUniverseTypesTypeId(typeID, nil)
	<-esiSemaphore
	if err != nil {
		return false, err
	}

	// If it is published and has a market group it is a market type!
	if typeInfo.Published && (typeInfo.MarketGroupId != 0) {
		return true, nil
	}

	return false, nil
}
