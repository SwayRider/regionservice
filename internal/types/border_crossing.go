// Package types defines shared data types for the regionservice.
//
// This package contains type definitions for road classifications and border
// crossing data structures used throughout the service.
package types

import "strings"

// RoadType represents an OpenStreetMap highway classification.
// Road types are used to categorize border crossings by road importance.
type RoadType string

// Road type constants matching OSM highway classifications.
const (
	MOTORWAY  RoadType = "motorway"  // Highest capacity roads (highways/freeways)
	TRUNK     RoadType = "trunk"     // Important roads that aren't motorways
	PRIMARY   RoadType = "primary"   // Major roads connecting large towns
	SECONDARY RoadType = "secondary" // Roads connecting smaller towns
)

// RoadTypeFromString converts a string to a RoadType.
// Returns empty string for unrecognized road types.
func RoadTypeFromString(s string) RoadType {
	switch strings.ToLower(s) {
	case "motorway":
		return MOTORWAY
	case "trunk":
		return TRUNK
	case "primary":
		return PRIMARY
	case "secondary":
		return SECONDARY
	default:
		return ""
	}
}

// String returns the string representation of the road type.
func (r RoadType) String() string {
	return string(r)
}

// BorderCrossingCollection is a slice of border crossings for CSV unmarshaling.
type BorderCrossingCollection []BorderCrossing

// BorderCrossing represents a road crossing point between two regions.
// Data is loaded from CSV files on the local filesystem.
type BorderCrossing struct {
	FromRegion string   `csv:"from_region"` // Source region name
	ToRegion   string   `csv:"to_region"`   // Destination region name
	OsmId      int      `csv:"osm_id"`      // OpenStreetMap way ID
	RoadType   RoadType `csv:"osm_type"`    // Road classification
	Lon        float64  `csv:"lon"`         // Crossing longitude
	Lat        float64  `csv:"lat"`         // Crossing latitude
}
