// manifest.go provides types and methods for reading geodata manifests.

package geodata

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	log "github.com/swayrider/swlib/logger"
)

// Manifest describes the available geodata.
// It lists all regions with their contours and shared border crossing data.
type Manifest struct {
	Regions map[string]Region `yaml:"regions"` // Map of region name to region data
	Shared  Shared            `yaml:"shared"`  // Shared data across regions
}

// Region describes a single region's geodata files.
type Region struct {
	Contour *Contour `yaml:"contour"` // Core and extended contour files
}

// Shared contains data shared across all regions.
type Shared struct {
	BorderCrossings map[string]*BorderCrossingDesc `yaml:"border-crossings"` // Border crossing files
}

// Contour references the core and extended contour files for a region.
type Contour struct {
	Core     *ContourDesc `yaml:"core"`     // Official region boundary
	Extended *ContourDesc `yaml:"extended"` // Extended boundary with buffer
}

// ContourDesc describes a contour file on the filesystem.
type ContourDesc struct {
	Hash       string `yaml:"hash"`        // File content hash for verification
	HashType   string `yaml:"hash-type"`   // Hash algorithm (e.g., "sha256")
	RemoteFile string `yaml:"remote-file"` // Relative path under geodataDir
}

// BorderCrossingDesc describes a border crossing file on the filesystem.
type BorderCrossingDesc struct {
	Hash       string `yaml:"hash"`        // File content hash for verification
	HashType   string `yaml:"hash-type"`   // Hash algorithm (e.g., "sha256")
	RemoteFile string `yaml:"remote-file"` // Relative path under geodataDir
}

// GetManifest reads the manifest YAML file from the geodata directory.
func (r *GeoDataReader) GetManifest(
	_ context.Context,
) (manifest *Manifest, err error) {
	lg := r.lg.Derive(log.WithFunction("GetManifest"))

	path := filepath.Join(r.geodataDir, "manifest.yml")
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

	manifest = &Manifest{}
	err = yaml.Unmarshal(bytes, manifest)
	return
}
