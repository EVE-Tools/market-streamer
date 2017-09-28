package locationCache

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"sync"

	staticData "github.com/EVE-Tools/static-data/lib/locations"
)

var locationServiceURL string
var httpClient *http.Client
var locationCache = struct {
	sync.RWMutex
	store map[int64]*staticData.Location
}{store: make(map[int64]*staticData.Location)}

// Initialize initializes infrastructure for locations
func Initialize(url string, client *http.Client) {
	locationServiceURL = url

	httpClient = client
}

// GetLocations returns a (cached) version of location info from the location endpoint
func GetLocations(locationIDs []int64) (map[int64]*staticData.Location, error) {
	// Deduplicate IDs
	locationIDs = deduplicateIDs(locationIDs)

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

		response, err := httpClient.Post(locationServiceURL, "application/json", bytes.NewBuffer(serializedRequest))
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
			loc := location
			locationCache.Lock()
			locationCache.store[loc.Station.ID] = &loc
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

// Deduplicate a slice of integers
func deduplicateIDs(ids []int64) []int64 {
	// This is a small trick for deduplicating IDs: Simply create a map
	// and use it as a set by mapping the keys to empty values, then re-add
	// keys to target slice. The map has constant lookup time, so adding the
	// keys is really fast and the size of the target slice is already determined
	// by the map. This is more efficient than a naïve algorithm.
	idSet := make(map[int64]struct{})
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	var i int
	uniqueIDs := make([]int64, len(idSet))
	for id := range idSet {
		uniqueIDs[i] = id
		i++
	}

	return uniqueIDs
}
