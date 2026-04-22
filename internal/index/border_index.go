// border_index.go provides indexing for border crossings between regions.

package index

import (
	"context"
	"sort"

	//"github.com/dhconnelly/rtreego"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/planar"
	"github.com/paulmach/orb/geo"
	"github.com/swayrider/regionservice/internal/types"
)

// BorderCrossingResult holds a border crossing with its squared distance to a query line.
type BorderCrossingResult struct {
	DistanceSquared float64         // Squared distance for efficient comparison
	BorderCrossing  *BorderCrossing // The border crossing
}

// ClosestBorderCrossing holds a border crossing with its actual distance to a query point.
type ClosestBorderCrossing struct {
	Distance       float64         // Distance in meters
	BorderCrossing *BorderCrossing // The border crossing
}

// RegionCrossings maps [FromRegion][ToRegion] to a list of border crossings.
type RegionCrossings map[string]map[string][]*BorderCrossing

// BorderIndex provides lookup for border crossings between region pairs.
// It supports finding crossings near a line segment or closest to a point.
type BorderIndex struct {
	regionCrossings RegionCrossings // Map of region pairs to crossings
}

// NewBorderIndex creates a new empty border index.
func NewBorderIndex() *BorderIndex {
	return &BorderIndex{
		regionCrossings: make(RegionCrossings),
		//crossingLocations: make(CrossingLocations),
	}
}

// Add adds a collection of border crossings to the index.
// Crossings are organized by (FromRegion, ToRegion) pairs.
func (i *BorderIndex) Add(
	crossings types.BorderCrossingCollection,
) {
	for _, c := range crossings {
		bc := &BorderCrossing{
			FromRegion: c.FromRegion,
			ToRegion: c.ToRegion,
			OsmId: c.OsmId,
			RoadType: c.RoadType,
			Location: orb.Point{c.Lon, c.Lat},
		}
		i.regionCrossings.add(bc)
		//i.crossingLocations.add(bc)
	}
}

// FindCrossingLocations finds border crossings near a line segment between two regions.
//
// Parameters:
//   - fromRegion, toRegion: Region pair to search
//   - line: Line segment (typically from start to end of route)
//   - roadOrder: Priority order for road types (e.g., motorway first)
//   - limit: Maximum number of crossings to return
//   - roadTypeDelta: Distance threshold (meters) within which road type takes precedence
//   - dropDistance: Minimum distance (meters) between returned crossings
//
// Returns crossings sorted by road type priority and distance, deduplicated by dropDistance.
func (i *BorderIndex) FindCrossingLocations(
	ctx context.Context,
	fromRegion, toRegion string,
	line orb.LineString,
	roadOrder []string,
	limit int,
	roadTypeDelta float64,
	dropDistance float64,
) []*BorderCrossingResult {
	toMap, ok := i.regionCrossings[fromRegion]
	if !ok {
		return nil
	}
	arr, ok := toMap[toRegion]
	if !ok {
		return nil
	}

	roadFilter := make(map[types.RoadType]int)
	for idx, r := range roadOrder {
		roadFilter[types.RoadTypeFromString(r)] = idx
	}

	cands := make([]*BorderCrossingResult, 0, len(arr))
	for _, cand := range arr {
		if _, ok := roadFilter[cand.RoadType]; !ok {
			continue
		}

		dSq := planar.DistanceFromSegmentSquared(line[0], line[1], cand.Location)
		cands = append(cands, &BorderCrossingResult{
			DistanceSquared: dSq,
			BorderCrossing: cand,
		})
	}

	sort.Slice(cands, func(i, j int) bool {
		// If same road type, sort by distance
		if roadFilter[cands[i].BorderCrossing.RoadType] == roadFilter[cands[j].BorderCrossing.RoadType] {
			return cands[i].DistanceSquared < cands[j].DistanceSquared
		}

		// If not, check is we are within the thesshold, if so sort by road type
		distMtrs := geo.Distance(cands[i].BorderCrossing.Location, cands[j].BorderCrossing.Location)
		if distMtrs < roadTypeDelta {
			return roadFilter[cands[i].BorderCrossing.RoadType] < roadFilter[cands[j].BorderCrossing.RoadType]
		}

		// Else sort by distance
		return cands[i].DistanceSquared < cands[j].DistanceSquared
	})

	list := make([]*BorderCrossingResult, 0, limit)
	cnt := 0
	var lastPoint orb.Point
	for _, cand := range cands {
		if cnt == 0 {
			lastPoint = cand.BorderCrossing.Location
			list = append(list, cand)
			cnt++
			continue
		}

		distMtrs := geo.Distance(lastPoint, cand.BorderCrossing.Location)
		if distMtrs > dropDistance {
			lastPoint = cand.BorderCrossing.Location
			list = append(list, cand)
			cnt++
		}

		if cnt == limit {
			break
		}
	}

	/*if limit > 0 && len(cands) > limit {
		return cands[:limit]
	}*/
	return list
}

// FindClosestCrossing finds the nearest border crossing to a point.
//
// Parameters:
//   - fromRegion, toRegion: Region pair to search
//   - location: Reference point to measure distance from
//   - validRoadTypes: Optional filter for road types (empty means all)
//
// Returns the closest crossing or nil if none found.
func (i *BorderIndex) FindClosestCrossing(
	ctx context.Context,
	fromRegion, toRegion string,
	location orb.Point,
	validRoadTypes []string,
) *ClosestBorderCrossing {
	toMap, ok := i.regionCrossings[fromRegion]
	if !ok {
		return nil
	}
	arr, ok := toMap[toRegion]
	if !ok {
		return nil
	}

	roadFilter := make(map[types.RoadType]int)
	for idx, r := range validRoadTypes {
		roadFilter[types.RoadTypeFromString(r)] = idx
	}

	var crossing *ClosestBorderCrossing
	for _, bc := range arr {
		if len(roadFilter) > 0 {
			if _, ok := roadFilter[bc.RoadType]; !ok {
				continue
			}
		}

		dist := geo.Distance(location, bc.Location)
		if crossing == nil || dist < crossing.Distance {
			crossing = &ClosestBorderCrossing{
				Distance: dist,
				BorderCrossing: bc,
			}
		}
	}
	return crossing
}


// FindRegionPath finds a path of regions from source to destination.
// Uses breadth-first search through the border crossing graph.
// Returns nil if no path exists.
func (i *BorderIndex) FindRegionPath(
	ctx context.Context,
	fromRegion, toRegion string,
) []string {
	passed := make(map[string]struct{})
	endpoints := make(map[string][]string)

	// No crossing possible at all
	toMap, ok := i.regionCrossings[fromRegion]
	if !ok {
		return nil
	}

	// Step 1, check direct neighbours
	for newRegion := range toMap {
		endpoints[newRegion] = []string{fromRegion, newRegion}
		passed[newRegion] = struct{}{}
	}

	if regionList, ok := endpoints[toRegion]; ok {
		return regionList
	}

	// Step 2. Iterate to find path
	for {
		// If none found, we are done
		numAdded := 0

		// New list of endpoints
		newEndpoints := make(map[string][]string)

		for region, list := range endpoints {
			toMap, ok := i.regionCrossings[region]
			if !ok {
				continue
			}

			for newRegion := range toMap {
				if _, ok := passed[newRegion]; ok {
					continue
				}
				newEndpoints[newRegion] = append([]string{}, list...)
				newEndpoints[newRegion] = append(newEndpoints[newRegion], newRegion)
				passed[newRegion] = struct{}{}
				numAdded++
			}
		}

		if numAdded == 0 {
			break
		}
		endpoints = newEndpoints
	}

	if regionList, ok := endpoints[toRegion]; ok {
		return regionList
	}
	return nil
}


// add inserts a border crossing into the region crossings map.
func (rc *RegionCrossings) add(bc *BorderCrossing) {
	toMap, ok := (*rc)[bc.FromRegion]
	if !ok {
		toMap = make(map[string][]*BorderCrossing)
		(*rc)[bc.FromRegion] = toMap
	}
	toMap[bc.ToRegion] = append(toMap[bc.ToRegion], bc)
}

/*func (cl *CrossingLocations) add(bc *BorderCrossing) {
	toMap, ok := (*cl)[bc.FromRegion]
	if !ok {
		toMap = make(map[string]*rtreego.Rtree)
		(*cl)[bc.FromRegion] = toMap
	}
	tree, ok := toMap[bc.ToRegion]
	if !ok {
		tree = rtreego.NewTree(2, 10, 25)
		toMap[bc.ToRegion] = tree
	}
	tree.Insert(NewSpatialBorderCrossing(bc))
}*/

