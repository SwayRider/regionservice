// validate.go implements shared input validation for geospatial request
// fields: coordinates, radii, and bounding boxes. gRPC can carry NaN/Inf and
// out-of-range values that JSON cannot, so these checks must be explicit.

package server

import (
	"math"

	"github.com/swayrider/protos/common_types/geo"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Bounds for valid WGS84 coordinates.
const (
	minLat = -90.0
	maxLat = 90.0
	minLon = -180.0
	maxLon = 180.0
)

// validateCoordinate checks that a coordinate is present, finite, and within
// valid latitude/longitude ranges.
func validateCoordinate(c *geo.Coordinate, field string) error {
	if c == nil {
		return status.Errorf(codes.InvalidArgument, "%s must be set", field)
	}
	if math.IsNaN(c.Lat) || math.IsInf(c.Lat, 0) {
		return status.Errorf(
			codes.InvalidArgument, "%s latitude must be a finite number", field)
	}
	if math.IsNaN(c.Lon) || math.IsInf(c.Lon, 0) {
		return status.Errorf(
			codes.InvalidArgument, "%s longitude must be a finite number", field)
	}
	if c.Lat < minLat || c.Lat > maxLat {
		return status.Errorf(
			codes.InvalidArgument,
			"%s latitude must be within [%v, %v], got %v",
			field, minLat, maxLat, c.Lat)
	}
	if c.Lon < minLon || c.Lon > maxLon {
		return status.Errorf(
			codes.InvalidArgument,
			"%s longitude must be within [%v, %v], got %v",
			field, minLon, maxLon, c.Lon)
	}
	return nil
}

// validateRadiusKm checks that a radius is finite and positive.
func validateRadiusKm(km float64) error {
	if math.IsNaN(km) || math.IsInf(km, 0) {
		return status.Error(codes.InvalidArgument, "radius_km must be a finite number")
	}
	if km <= 0 {
		return status.Error(codes.InvalidArgument, "radius_km must be positive")
	}
	return nil
}

// validateBox checks that a bounding box is present, has valid corners, and is
// not inverted in latitude. Longitude inversion is intentionally allowed: it
// represents a box crossing the antimeridian and is handled by SearchByBox.
func validateBox(box *geo.BoundingBox) error {
	if box == nil {
		return status.Error(codes.InvalidArgument, "box must be set")
	}
	if err := validateCoordinate(box.BottomLeft, "box.bottom_left"); err != nil {
		return err
	}
	if err := validateCoordinate(box.TopRight, "box.top_right"); err != nil {
		return err
	}
	if box.BottomLeft.Lat > box.TopRight.Lat {
		return status.Error(
			codes.InvalidArgument,
			"box.bottom_left latitude must not exceed box.top_right latitude")
	}
	return nil
}
