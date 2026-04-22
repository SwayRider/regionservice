// Package geodata provides filesystem-based access to geodata files.
package geodata

import (
	log "github.com/swayrider/swlib/logger"
)

// GeoDataReader reads geodata files from a local directory.
type GeoDataReader struct {
	geodataDir string
	lg         *log.Logger
}

// NewGeoDataReader creates a new GeoDataReader rooted at geodataDir.
func NewGeoDataReader(geodataDir string, l *log.Logger) *GeoDataReader {
	return &GeoDataReader{
		geodataDir: geodataDir,
		lg:         l.Derive(log.WithComponent("geodata")),
	}
}
