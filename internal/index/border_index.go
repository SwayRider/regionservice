// border_index.go provides indexing for border crossings between regions.

package index

import (
	"context"
	"math"
	"sort"

	//"github.com/dhconnelly/rtreego"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geo"
	"github.com/swayrider/regionservice/internal/types"
)

// BorderCrossingResult holds a border crossing with its distance to a query line.
type BorderCrossingResult struct {
	DistanceMeters float64         // Distance in meters to the query line segment
	BorderCrossing *BorderCrossing // The border crossing
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
			ToRegion:   c.ToRegion,
			OsmId:      c.OsmId,
			RoadType:   c.RoadType,
			Location:   orb.Point{c.Lon, c.Lat},
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
//   - roadTypeDelta: Distance threshold (meters) between two crossings'
//     distances to the line within which they are regarded as equally
//     distant and ranked by road type priority instead of distance
//   - dropDistance: Minimum distance (meters) between returned crossings
//
// Returns crossings ranked by (distance bucket, road type priority, distance)
// and deduplicated by dropDistance. The ranking is a strict total order, so
// it is deterministic and independent of input order.
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

		cands = append(cands, &BorderCrossingResult{
			DistanceMeters: distanceToSegmentMeters(
				line[0], line[1], cand.Location),
			BorderCrossing: cand,
		})
	}

	// Rank by a strict total-order key so the result is deterministic and
	// independent of input order: (distance bucket, road type priority,
	// distance, osm id). Crossings in the same bucket are "equally distant
	// from the line" and ranked by road type; crossings in different buckets
	// are ranked purely by distance to the line.
	sort.Slice(cands, func(i, j int) bool {
		bi := distanceBucket(cands[i].DistanceMeters, roadTypeDelta)
		bj := distanceBucket(cands[j].DistanceMeters, roadTypeDelta)
		if bi != bj {
			return bi < bj
		}
		pi := roadFilter[cands[i].BorderCrossing.RoadType]
		pj := roadFilter[cands[j].BorderCrossing.RoadType]
		if pi != pj {
			return pi < pj
		}
		if cands[i].DistanceMeters != cands[j].DistanceMeters {
			return cands[i].DistanceMeters < cands[j].DistanceMeters
		}
		return cands[i].BorderCrossing.OsmId < cands[j].BorderCrossing.OsmId
	})

	// A negative limit must never reach make's capacity argument (it would
	// panic with "makeslice: cap out of range"). Clamp it defensively; the
	// handler layer already defaults non-positive limits, so this only guards
	// against direct callers.
	capacity := limit
	if capacity < 0 {
		capacity = 0
	}
	list := make([]*BorderCrossingResult, 0, capacity)
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

// distanceBucket returns the bucket a distance-to-line falls into. Crossings
// whose distance to the line falls in the same bucket are regarded as equally
// distant and ranked by road type priority; crossings in different buckets are
// ranked purely by distance. A non-positive delta disables grouping so the
// ordering is purely by distance.
func distanceBucket(distMeters, delta float64) float64 {
	if delta <= 0 {
		return distMeters
	}
	return math.Floor(distMeters / delta)
}

// distanceToSegmentMeters returns the great-circle distance in meters from
// point to the line segment [a, b]. The point is projected onto the segment
// in planar space and the haversine distance to that foot point is returned.
// The projection is exact for the short, sub-degree segments used for
// border-crossing search.
func distanceToSegmentMeters(a, b, p orb.Point) float64 {
	return geo.Distance(closestPointOnSegment(a, b, p), p)
}

// closestPointOnSegment returns the point on segment [a, b] closest to p,
// using a planar projection parameter clamped to [0, 1].
func closestPointOnSegment(a, b, p orb.Point) orb.Point {
	abx, aby := b[0]-a[0], b[1]-a[1]
	l2 := abx*abx + aby*aby
	if l2 == 0 {
		return a
	}
	t := ((p[0]-a[0])*abx + (p[1]-a[1])*aby) / l2
	t = math.Max(0, math.Min(1, t))
	return orb.Point{a[0] + t*abx, a[1] + t*aby}
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
				Distance:       dist,
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

const maxPathLengthMultiplier = 2

// FindRouteRegionPaths finds all acyclic paths from fromRegion to toRegion
// that are constrained to the provided set of allowed regions.
// Uses iterative DFS; path length is bounded at maxPathLengthMultiplier times
// the shortest allowed BFS path to prevent exponential exploration.
// Returns nil if no path exists within the allowed set.
func (i *BorderIndex) FindRouteRegionPaths(
	ctx context.Context,
	fromRegion, toRegion string,
	allowedRegions map[string]bool,
) [][]string {
	if !allowedRegions[fromRegion] || !allowedRegions[toRegion] {
		return nil
	}

	shortest := i.findShortestAllowedPath(fromRegion, toRegion, allowedRegions)
	if shortest == nil {
		return nil
	}
	maxLen := len(shortest) * maxPathLengthMultiplier

	var results [][]string
	stack := [][]string{{fromRegion}}

	for len(stack) > 0 {
		path := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		last := path[len(path)-1]

		if last == toRegion {
			results = append(results, path)
			continue
		}

		if len(path) >= maxLen {
			continue
		}

		for neighbor := range i.regionCrossings[last] {
			if !allowedRegions[neighbor] {
				continue
			}
			if regionInPath(path, neighbor) {
				continue
			}
			newPath := make([]string, len(path)+1)
			copy(newPath, path)
			newPath[len(path)] = neighbor
			stack = append(stack, newPath)
		}
	}

	return results
}

// findShortestAllowedPath is a BFS that finds the shortest path from fromRegion
// to toRegion considering only regions in allowed.
func (i *BorderIndex) findShortestAllowedPath(
	fromRegion, toRegion string,
	allowed map[string]bool,
) []string {
	toMap, ok := i.regionCrossings[fromRegion]
	if !ok {
		return nil
	}

	passed := make(map[string]struct{})
	endpoints := make(map[string][]string)

	for neighbor := range toMap {
		if !allowed[neighbor] {
			continue
		}
		endpoints[neighbor] = []string{fromRegion, neighbor}
		passed[neighbor] = struct{}{}
	}

	if path, ok := endpoints[toRegion]; ok {
		return path
	}

	for {
		numAdded := 0
		newEndpoints := make(map[string][]string)

		for region, list := range endpoints {
			toMap, ok := i.regionCrossings[region]
			if !ok {
				continue
			}
			for neighbor := range toMap {
				if _, seen := passed[neighbor]; seen {
					continue
				}
				if !allowed[neighbor] {
					continue
				}
				newEndpoints[neighbor] = append([]string{}, list...)
				newEndpoints[neighbor] = append(newEndpoints[neighbor], neighbor)
				passed[neighbor] = struct{}{}
				numAdded++
			}
		}

		if numAdded == 0 {
			break
		}
		endpoints = newEndpoints
	}

	if path, ok := endpoints[toRegion]; ok {
		return path
	}
	return nil
}

// regionInPath reports whether region already appears in path.
func regionInPath(path []string, region string) bool {
	for _, r := range path {
		if r == region {
			return true
		}
	}
	return false
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
