// border_crossing.go provides methods for retrieving border crossing CSV files.

package geodata

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/gocarina/gocsv"
	"github.com/swayrider/regionservice/internal/types"
	log "github.com/swayrider/swlib/logger"
)

// GetBorderCrossing reads a border crossing CSV file from the local filesystem.
// RemoteFile in the descriptor is relative to geodataDir.
// Returns a collection of border crossings parsed from the CSV.
func (r *GeoDataReader) GetBorderCrossing(
	_ context.Context,
	bcDesc *BorderCrossingDesc,
) (
	crossings types.BorderCrossingCollection,
	err error,
) {
	lg := r.lg.Derive(log.WithFunction("GetBorderCrossing"))

	path := filepath.Join(r.geodataDir, bcDesc.RemoteFile)
	lg.Debugf("reading %s", path)

	f, err := os.Open(path)
	if err != nil {
		lg.Warnf("failed to open %s: %v", path, err)
		return
	}
	defer f.Close()

	bytes, err := io.ReadAll(f)
	if err != nil {
		lg.Warnf("failed to read %s: %v", path, err)
		return
	}

	err = gocsv.UnmarshalBytes(bytes, &crossings)
	if err != nil {
		lg.Warnf("failed to unmarshal %s: %v", path, err)
		return
	}

	return
}
