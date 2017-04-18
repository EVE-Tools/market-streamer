package scraper

import "time"

// ESIOrder is an order returned by the ESI API.
type ESIOrder struct {
	OrderID      int64     `json:"order_id"`
	TypeID       int64     `json:"type_id"`
	LocationID   int64     `json:"location_id"`
	VolumeTotal  int64     `json:"volume_total"`
	VolumeRemain int64     `json:"volume_remain"`
	MinVolume    int64     `json:"min_volume"`
	Price        float64   `json:"price"`
	IsBuyOrder   bool      `json:"is_buy_order"`
	Duration     int64     `json:"duration"`
	Issued       time.Time `json:"issued"`
	Range        string    `json:"range"`
}
