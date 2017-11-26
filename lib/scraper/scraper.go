package scraper

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"golang.org/x/oauth2"

	"github.com/EVE-Tools/emdr-to-nsq/lib/emds"
	"github.com/EVE-Tools/market-streamer/lib/locations/citadels"
	"github.com/EVE-Tools/market-streamer/lib/locations/locationCache"
	"github.com/EVE-Tools/market-streamer/lib/marketTypes"
	"github.com/antihax/goesi"
	"github.com/antihax/goesi/esi"
	"github.com/klauspost/compress/zlib"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type esiOrder esi.GetMarketsRegionIdOrders200Ok

var esiClient *goesi.APIClient
var esiPublicContext context.Context

// Initialize initializes the scraper
func Initialize(clientID string, secretKey string, refreshToken string, httpClient *http.Client, client *goesi.APIClient) {
	// Requests to citadel's markets are authenticated - we're just using a default key for retrieving public markets
	esiAuthenticator := goesi.NewSSOAuthenticator(
		httpClient,
		clientID,
		secretKey,
		"eveauth-e43://market-streamer",
		[]string{"esi-universe.read_structures.v1",
			"esi-search.search_structures.v1",
			"esi-markets.structure_markets.v1"})

	// Build token source for auto-refreshing tokens
	token := &oauth2.Token{
		AccessToken:  "",
		TokenType:    "Bearer",
		RefreshToken: refreshToken,
		Expiry:       time.Now().AddDate(0, 0, -1),
	}

	esiPublicToken, err := esiAuthenticator.TokenSource(token)
	if err != nil {
		log.Fatalf("Error starting bootstrap ESI client: %v", err)
	}

	esiPublicContext = context.WithValue(context.TODO(), goesi.ContextOAuth2, esiPublicToken)
	esiClient = client
}

// ScrapeMarket gets a market from ESI and pushes it to supported backends
func ScrapeMarket(regionID int64, lastModified time.Time) ([]byte, *time.Time, *time.Time, error) {
	// Prepare empty rowsets with all market types
	rowsets := generateRowsetsForRegion(regionID)

	//
	// Fetch public region Orders
	//
	// // First page -> re-schedule, check if modified since last execution
	params := make(map[string]interface{})
	params["page"] = int32(1)

	esiOrdersRegion, response, err := esiClient.ESI.MarketApi.GetMarketsRegionIdOrders(nil, "all", int32(regionID), params)
	if err != nil {
		return nil, nil, nil, err
	}

	expiry, err := time.Parse(time.RFC1123, response.Header.Get("expires"))
	if err != nil {
		// Will run in 10 minutes, anyway
		logrus.WithError(err).Warn("Could not parse ESI expires timestamp!")
		return nil, nil, nil, err
	}

	// If expired in the past check back in fifteen seconds (see next line) as the CDN might take some time to refresh
	if expiry.Before(time.Now()) {
		expiry = time.Now().Add(time.Second * 10)
	}

	// Re-schedule self with 5 second safety margin
	runAgain := expiry.Add(time.Second * 5)

	// Check if we really got a new market or the cached version from last time
	newLastModified, err := time.Parse(time.RFC1123, response.Header.Get("last-modified"))
	if err != nil {
		// Will run in 10 minutes, anyway
		logrus.WithError(err).Warn("Could not parse ESI last-modified timestamp!")
		return nil, nil, nil, err
	}

	if !newLastModified.After(lastModified) {
		// We got an old market, stop here
		logrus.WithFields(logrus.Fields{
			"regionID": regionID,
			"runAgain": runAgain,
		}).Info("Old market.")

		return nil, &runAgain, &newLastModified, nil
	}

	// Add orders to rowset
	err = appendResponseRegion(rowsets, esiOrdersRegion, response)
	if err != nil {
		return nil, nil, nil, err
	}

	// Fetch all other pages
	for len(esiOrdersRegion) > 0 {
		params["page"] = params["page"].(int32) + 1
		esiOrdersRegion, response, err = esiClient.ESI.MarketApi.GetMarketsRegionIdOrders(nil, "all", int32(regionID), params)
		if err != nil {
			return nil, nil, nil, err
		}

		// Add orders to rowset
		err = appendResponseRegion(rowsets, esiOrdersRegion, response)
		if err != nil {
			return nil, nil, nil, err
		}
	}

	//
	// Fetch orders in citadels
	//
	citadelIDs := citadels.GetCitadelsInRegion(regionID)

	for _, citadelID := range citadelIDs {
		params["page"] = int32(1)
		esiOrdersCitadel, response, err := esiClient.ESI.MarketApi.GetMarketsStructuresStructureId(esiPublicContext, citadelID, params)
		if err != nil {
			// Blacklist and skip these citadels
			if (response != nil) && (response.StatusCode == 403) {
				citadels.BlacklistCitadel(citadelID)
				continue
			}
			return nil, nil, nil, err
		}

		// Add orders to rowset
		err = appendResponseCitadel(rowsets, esiOrdersCitadel, response)
		if err != nil {
			return nil, nil, nil, err
		}

		// Fetch all other pages
		for len(esiOrdersCitadel) > 0 {
			params["page"] = params["page"].(int32) + 1
			esiOrdersCitadel, response, err = esiClient.ESI.MarketApi.GetMarketsStructuresStructureId(esiPublicContext, citadelID, params)
			if err != nil {
				return nil, nil, nil, err
			}

			// Add orders to rowset
			err = appendResponseCitadel(rowsets, esiOrdersCitadel, response)
			if err != nil {
				return nil, nil, nil, err
			}
		}
	}

	// Set generatedAt, sort slices within rowsets and deduplicate orders
	for _, rowset := range rowsets {
		// Sort
		sort.Sort(emds.ByOrderID(rowset.Rows))

		if len(rowset.Rows) > 0 {
			rowset.GeneratedAt = rowset.Rows[0].GeneratedAt
		}

		// Dedup by ID
		inititalLength := len(rowset.Rows)
		deduplicated := rowset.Rows[:0]
		numRemoved := 0
		for index, row := range rowset.Rows {
			if index > 0 && (rowset.Rows[index-1].OrderID == row.OrderID) {
				numRemoved++
				logrus.WithField("order", fmt.Sprintf("%+v", rowset.Rows[index-1])).Debug("A: ")
				logrus.WithField("order", fmt.Sprintf("%+v", rowset.Rows[index])).Debug("B: ")
			} else {
				deduplicated = append(deduplicated, row)
			}
		}

		rowset.Rows = deduplicated

		if numRemoved > 0 {
			logrus.WithFields(logrus.Fields{
				"numDuplicates": numRemoved,
				"regionID":      rowset.RegionID,
				"typeID":        rowset.TypeID,
				"lengthBefore":  inititalLength,
				"lengthAfter":   len(rowset.Rows),
			}).Debug("Removed duplicate orders.")
		}
	}

	// Serialize rowsets
	rowsetSlice := make([]emds.Rowset, len(rowsets))
	rowsetIndex := 0
	numOrders := 0
	for _, rowset := range rowsets {
		numOrders += len(rowset.Rows)
		rowsetSlice[rowsetIndex] = *rowset
		rowsetIndex++
	}

	rowsetJSON, err := emds.RowsetsToUUDIF(rowsetSlice, "Element43/market-streamer", "0.1")
	if err != nil {
		return nil, nil, nil, err
	}

	var compressedJSONBuffer bytes.Buffer
	compressionWriter := zlib.NewWriter(&compressedJSONBuffer)
	_, err = compressionWriter.Write(rowsetJSON)
	if err != nil {
		return nil, nil, nil, err
	}

	err = compressionWriter.Close()
	if err != nil {
		return nil, nil, nil, err
	}

	compressedJSON := compressedJSONBuffer.Bytes()

	logrus.WithFields(logrus.Fields{
		"regionID":          regionID,
		"numOrders":         numOrders,
		"bytesUncompressed": len(rowsetJSON),
		"bytesCompressed":   len(compressedJSON),
	}).Info("Uploading market.")

	return compressedJSON, &runAgain, &newLastModified, nil
}

// Type conversion for regions
func appendResponseRegion(rowsets map[int64]*emds.Rowset, regionOrders []esi.GetMarketsRegionIdOrders200Ok, response *http.Response) error {
	var orders []esiOrder

	for _, regionOrder := range regionOrders {
		orders = append(orders, esiOrder(regionOrder))
	}

	return appendResponse(rowsets, orders, response)
}

// Type conversion for citadels
func appendResponseCitadel(rowsets map[int64]*emds.Rowset, citadelOrders []esi.GetMarketsStructuresStructureId200Ok, response *http.Response) error {
	var orders []esiOrder

	for _, citadelOrder := range citadelOrders {
		orders = append(orders, esiOrder(citadelOrder))
	}

	return appendResponse(rowsets, orders, response)
}

func appendResponse(rowsets map[int64]*emds.Rowset, esiOrders []esiOrder, response *http.Response) error {
	lastModified, err := time.Parse(time.RFC1123, response.Header.Get("last-modified"))
	if err != nil {
		// Default to now
		logrus.WithError(err).Warn("Could not parse ESI last-modified timestamp!")
		lastModified = time.Now()
	}

	generatedAt := lastModified.Format(time.RFC3339)

	return appendOrders(rowsets, esiOrders, generatedAt)
}

func appendOrders(rowsets map[int64]*emds.Rowset, esiOrders []esiOrder, generatedAt string) error {
	// Collect locations
	var locationIDs []int64
	for _, order := range esiOrders {
		locationIDs = append(locationIDs, order.LocationId)
	}

	locations, err := locationCache.GetLocations(locationIDs)
	if err != nil {
		return err
	}

	// Add orders including location info
	for _, order := range esiOrders {
		if location, ok := locations[order.LocationId]; ok {
			typeID := int64(order.TypeId)

			orderRange, err := emds.ConvertRange(order.Range_)
			if err != nil {
				logrus.WithError(err).Error("Could not parse range! Skipping order.")
				continue
			}

			// Create rowset for types which should not be there
			if _, ok := rowsets[typeID]; !ok {
				logrus.WithField("typeID", typeID).WithField("order", fmt.Sprintf("%+v", order)).Debug("Type not in marketTypes but in orders!")

				rowsets[typeID] = &emds.Rowset{
					GeneratedAt: generatedAt,
					RegionID:    location.Region.ID,
					TypeID:      typeID,
				}
			}

			rowset := rowsets[typeID]
			rowset.Rows = append(rowset.Rows, emds.Order{
				OrderID:       order.OrderId,
				RegionID:      rowset.RegionID,
				TypeID:        int64(order.TypeId),
				GeneratedAt:   generatedAt,
				Price:         float64(order.Price),
				VolRemaining:  int64(order.VolumeRemain),
				OrderRange:    orderRange,
				VolEntered:    int64(order.VolumeTotal),
				MinVolume:     int64(order.MinVolume),
				Bid:           order.IsBuyOrder,
				IssueDate:     order.Issued.Format(time.RFC3339),
				Duration:      int64(order.Duration),
				StationID:     order.LocationId,
				SolarSystemID: location.SolarSystem.ID,
			})

		} else {
			logrus.WithField("locationID", order.LocationId).Warn("Unknown location.")
		}
	}

	return nil
}

// Generates empty rowsets for population by scraper
func generateRowsetsForRegion(regionID int64) map[int64]*emds.Rowset {
	rowsets := map[int64]*emds.Rowset{}
	now := time.Now().Format(time.RFC3339)
	types := marketTypes.GetMarketTypes()

	for _, typeID := range types {
		rowsets[typeID] = &emds.Rowset{
			GeneratedAt: now,
			RegionID:    regionID,
			TypeID:      typeID,
		}
	}

	return rowsets
}
