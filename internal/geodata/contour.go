// contour.go provides methods for retrieving region contour GeoJSON files.

package geodata

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-spatial/geom/encoding/geojson"
	log "github.com/swayrider/swlib/logger"
)

// GetContour reads a region contour GeoJSON file from the local filesystem.
// RemoteFile in the descriptor is relative to geodataDir.
// Returns a FeatureCollection containing the region polygon(s).
func (r *GeoDataReader) GetContour(
	_ context.Context,
	contourDesc *ContourDesc,
) (
	features *geojson.FeatureCollection,
	err error,
) {
	lg := r.lg.Derive(log.WithFunction("GetContour"))

	path := filepath.Join(r.geodataDir, contourDesc.RemoteFile)
	lg.Debugf("reading %s", path)

	f, err := os.Open(path)
	if err != nil {
		lg.Warnf("failed to open %s: %v", path, err)
		return
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	bytes, err := io.ReadAll(f)
	if err != nil {
		lg.Warnf("failed to read %s: %v", path, err)
		return
	}

	data, err := geojson.Unmarshal(bytes)
	if err != nil {
		lg.Warnf("failed to unmarshal %s: %v", path, err)
		return
	}

	features, err = toFeatureCollection(data)
	if err != nil {
		lg.Warnf("unsupported geojson in %s: %v", path, err)
		return
	}

	return
}

// toFeatureCollection converts an unmarshalled GeoJSON value into a
// FeatureCollection. Unmarshal yields either a Feature or a FeatureCollection;
// anything else is rejected with an explicit error so callers never receive a
// nil collection alongside a nil error.
func toFeatureCollection(data interface{}) (*geojson.FeatureCollection, error) {
	switch v := data.(type) {
	case geojson.FeatureCollection:
		return &v, nil
	case geojson.Feature:
		return &geojson.FeatureCollection{
			Features: []geojson.Feature{v},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported geojson type %T", data)
	}
}
