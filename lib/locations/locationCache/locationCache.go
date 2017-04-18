package locationCache

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	staticData "github.com/EVE-Tools/static-data/lib/locations"
)

var httpClient *http.Client
var locationCache = struct {
	sync.RWMutex
	store map[int64]*staticData.Location
}{store: make(map[int64]*staticData.Location)}

// Initialize initializes infrastructure for locations
func Initialize() {
	httpClient = &http.Client{
		Timeout: time.Duration(time.Second * 10),
	}
}

// GetLocations returns a (cached) version of location info from the location endpoint
func GetLocations(locationIDs []int64) (map[int64]*staticData.Location, error) {
	// Check which locations are in cache, request missing
	var missingLocations []int64

	locationCache.RLock()
	for _, id := range locationIDs {
		if _, ok := locationCache.store[id]; !ok {
			missingLocations = append(missingLocations, id)
		}
	}
	locationCache.RUnlock()

	if len(missingLocations) > 0 {
		requestBody := staticData.RequestLocationsBody{
			Locations: locationIDs,
		}

		serializedRequest, err := requestBody.MarshalJSON()
		if err != nil {
			return nil, err
		}

		response, err := httpClient.Post("https://element-43.com/api/static-data/v1/location/", "application/json", bytes.NewBuffer(serializedRequest))
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		responseJSON, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		var parsedResponse staticData.Response

		err = parsedResponse.UnmarshalJSON(responseJSON)
		if err != nil {
			return nil, err
		}

		for _, location := range parsedResponse {
			var loc = location

			locationCache.Lock()
			locationCache.store[location.Station.ID] = &loc
			locationCache.Unlock()
		}
	}

	locations := make(map[int64]*staticData.Location)

	locationCache.RLock()
	for _, id := range locationIDs {
		if _, ok := locationCache.store[id]; ok {
			locations[id] = locationCache.store[id]
		}
	}
	locationCache.RUnlock()

	return locations, nil
}

// GetLocation returns a (cached) version of location info from the location endpoint
func GetLocation(locationID int64) (*staticData.Location, error) {
	if _, ok := locationCache.store[locationID]; !ok {
		requestBody := staticData.RequestLocationsBody{
			Locations: []int64{locationID},
		}

		serializedRequest, err := requestBody.MarshalJSON()
		if err != nil {
			return nil, err
		}

		response, err := http.Post("https://element-43.com/api/static-data/v1/location/", "application/json", bytes.NewBuffer(serializedRequest))
		if err != nil {
			return nil, err
		}
		defer response.Body.Close()
		responseJSON, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		var parsedResponse staticData.Response

		err = parsedResponse.UnmarshalJSON(responseJSON)
		if err != nil {
			return nil, err
		}

		for _, location := range parsedResponse {
			locationCache.Lock()
			locationCache.store[location.Station.ID] = &location
			locationCache.Unlock()
		}
	}

	return locationCache.store[locationID], nil
}
