// validate_test.go tests the shared geospatial input validators.

package server

import (
	"math"
	"testing"

	"github.com/swayrider/protos/common_types/geo"
)

func TestValidateCoordinate(t *testing.T) {
	tests := []struct {
		name    string
		c       *geo.Coordinate
		wantErr bool
	}{
		{"nil", nil, true},
		{"origin", &geo.Coordinate{Lat: 0, Lon: 0}, false},
		{"max bounds", &geo.Coordinate{Lat: 90, Lon: 180}, false},
		{"min bounds", &geo.Coordinate{Lat: -90, Lon: -180}, false},
		{"lat too high", &geo.Coordinate{Lat: 90.1, Lon: 0}, true},
		{"lon too low", &geo.Coordinate{Lat: 0, Lon: -180.1}, true},
		{"NaN lat", &geo.Coordinate{Lat: math.NaN(), Lon: 0}, true},
		{"Inf lon", &geo.Coordinate{Lat: 0, Lon: math.Inf(1)}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCoordinate(tt.c, "location")
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateRadiusKm(t *testing.T) {
	tests := []struct {
		name    string
		km      float64
		wantErr bool
	}{
		{"positive", 10, false},
		{"zero", 0, true},
		{"negative", -1, true},
		{"NaN", math.NaN(), true},
		{"Inf", math.Inf(1), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRadiusKm(tt.km)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBox(t *testing.T) {
	tests := []struct {
		name    string
		box     *geo.BoundingBox
		wantErr bool
	}{
		{
			name: "valid",
			box: &geo.BoundingBox{
				BottomLeft: &geo.Coordinate{Lon: 0, Lat: 0},
				TopRight:   &geo.Coordinate{Lon: 10, Lat: 10},
			},
			wantErr: false,
		},
		{
			name: "antimeridian allowed",
			box: &geo.BoundingBox{
				BottomLeft: &geo.Coordinate{Lon: 179, Lat: -10},
				TopRight:   &geo.Coordinate{Lon: -179, Lat: 10},
			},
			wantErr: false,
		},
		{
			name: "inverted latitude",
			box: &geo.BoundingBox{
				BottomLeft: &geo.Coordinate{Lon: 0, Lat: 10},
				TopRight:   &geo.Coordinate{Lon: 10, Lat: 0},
			},
			wantErr: true,
		},
		{
			name:    "nil",
			box:     nil,
			wantErr: true,
		},
		{
			name: "missing corner",
			box: &geo.BoundingBox{
				BottomLeft: &geo.Coordinate{Lon: 0, Lat: 0},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBox(tt.box)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tt.wantErr)
			}
		})
	}
}
