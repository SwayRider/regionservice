# regionservice

Geographic region management service for the SwayRider platform. Provides spatial queries to determine which routing regions contain specific coordinates, and finds optimal border crossing points between adjacent regions.

## Architecture

The regionservice exposes two server interfaces:

| Interface | Port | Purpose |
| --------- | ---- | ------- |
| REST/HTTP | 8080 | HTTP API via gRPC-gateway |
| gRPC | 8081 | Internal service-to-service communication |

### Dependencies

None beyond the geodata volume mount.

### Data Loading

On startup, the service loads geodata from the local filesystem based on a manifest file:

1. Fetches the manifest for the configured tag
2. Loads core and extended contours for each region into the **Region Index**
3. Loads border crossing points into the **Border Index**

The service maintains two in-memory spatial indexes:
- **Region Index**: R-tree based index for fast point-in-polygon and bounding box queries
- **Border Index**: Index of border crossing locations between adjacent regions

## Configuration

Configuration is provided via environment variables or CLI flags.

### Server Configuration

| Environment Variable | CLI Flag | Default | Description |
| -------------------- | -------- | ------- | ----------- |
| `HTTP_PORT` | `-http-port` | 8080 | REST API port |
| `GRPC_PORT` | `-grpc-port` | 8081 | gRPC port |

### Geodata Configuration

| Environment Variable | CLI Flag | Default | Description |
| -------------------- | -------- | ------- | ----------- |
| `GEODATA_DIR` | `-geodata-dir` | | Root directory containing versioned geodata (volume mount) |
| `TAG` | `-tag` | | Tag/version subdirectory to load (empty = latest) |

## API Reference

The API is defined in the Protocol Buffer files at `backend/protos/region/v1/` and `backend/protos/health/v1/`.

All endpoints are public and require no authentication.

---

### Health Endpoints

#### Ping

Simple health check that returns HTTP 200.

- **Endpoint:** `GET /api/v1/health/ping`
- **Access:** Public

---

### Region Search Endpoints

#### Search Point

Finds all regions containing a specific coordinate.

- **Endpoint:** `POST /api/v1/region/search-point`
- **Access:** Public

```bash
curl --request POST \
  --url http://localhost:8080/api/v1/region/search-point \
  --header 'content-type: application/json' \
  --data '{
    "location": {
      "lat": 41.3851,
      "lon": 2.1734
    },
    "includeExtended": true
  }'
```

Response:
```json
{
  "coreRegions": ["iberian-peninsula"],
  "extendedRegions": ["west-europe"]
}
```

- `coreRegions`: Regions where the point is within the core coverage area
- `extendedRegions`: Regions where the point is within the extended (overlap) area

#### Search Box

Finds all regions intersecting a bounding box.

- **Endpoint:** `POST /api/v1/region/search-box`
- **Access:** Public

```bash
curl --request POST \
  --url http://localhost:8080/api/v1/region/search-box \
  --header 'content-type: application/json' \
  --data '{
    "box": {
      "bottomLeft": {
        "lat": 40.0,
        "lon": -4.0
      },
      "topRight": {
        "lat": 44.0,
        "lon": 3.0
      }
    },
    "includeExtended": true
  }'
```

Response:
```json
{
  "coreRegions": ["iberian-peninsula"],
  "extendedRegions": ["west-europe"]
}
```

#### Search Radius

Finds all regions within a radius of a coordinate.

- **Endpoint:** `POST /api/v1/region/search-radius`
- **Access:** Public

```bash
curl --request POST \
  --url http://localhost:8080/api/v1/region/search-radius \
  --header 'content-type: application/json' \
  --data '{
    "location": {
      "lat": 42.5,
      "lon": -1.5
    },
    "radiusKm": 100,
    "includeExtended": true
  }'
```

Response:
```json
{
  "coreRegions": ["iberian-peninsula"],
  "extendedRegions": ["west-europe"]
}
```

---

### Border Crossing Endpoints

#### Find Crossing Locations

Finds border crossing points between two adjacent regions, optimized for a given travel path.

- **Endpoint:** `POST /api/v1/region/find-crossing-locations`
- **Access:** Public

```bash
curl --request POST \
  --url http://localhost:8080/api/v1/region/find-crossing-locations \
  --header 'content-type: application/json' \
  --data '{
    "fromRegion": "iberian-peninsula",
    "toRegion": "west-europe",
    "fromLocation": {
      "lat": 40.4168,
      "lon": -3.7038
    },
    "toLocation": {
      "lat": 48.8566,
      "lon": 2.3522
    },
    "limit": 3,
    "simpleConfig": {
      "roadTypeOrder": ["MOTORWAY", "TRUNK", "PRIMARY"],
      "roadTypeDelta": 10000,
      "dropDistance": 1000
    }
  }'
```

Response:
```json
{
  "crossings": [
    {
      "fromRegion": "iberian-peninsula",
      "toRegion": "west-europe",
      "roadType": "MOTORWAY",
      "osmId": 123456789,
      "location": {
        "lat": 42.7889,
        "lon": -1.6403
      }
    }
  ]
}
```

**Configuration Options:**

Simple configuration (`simpleConfig`):
- `roadTypeOrder`: Preferred road types in order (MOTORWAY, TRUNK, PRIMARY, SECONDARY)
- `roadTypeDelta`: Distance threshold for road type preference (meters)
- `dropDistance`: Minimum distance between returned crossings (meters)

Advanced configuration (`advancedConfig`):
- `definitions`: Array of distance-based configurations for different border distances

#### Find Region Path

Finds the sequence of regions to traverse between two regions.

- **Endpoint:** `POST /api/v1/region/find-region-path`
- **Access:** Public

```bash
curl --request POST \
  --url http://localhost:8080/api/v1/region/find-region-path \
  --header 'content-type: application/json' \
  --data '{
    "fromRegion": "iberian-peninsula",
    "toRegion": "central-europe"
  }'
```

Response:
```json
{
  "path": ["iberian-peninsula", "west-europe", "central-europe"]
}
```

---

### Ping

Simple endpoint that returns HTTP 200.

- **Endpoint:** `GET /api/v1/region/ping`
- **Access:** Public

## Geodata Structure

The geodata directory must be mounted at the path configured by `GEODATA_DIR`. It follows this structure:

```
<GEODATA_DIR>/
└── <tag>/                        # Version tag, e.g. 2025-12-01
    ├── manifest.yml              # Manifest describing available regions
    ├── contours/
    │   ├── iberian-peninsula-core.geojson
    │   ├── iberian-peninsula-extended.geojson
    │   └── ...
    └── border-crossings/
        ├── iberian-peninsula--west-europe.csv
        └── ...
```

### Region Types

- **Core Region**: Primary coverage area for routing. Points in core regions are routed using that region's Valhalla instance.
- **Extended Region**: Overlap area that extends into adjacent regions. Used for cross-region routing to ensure seamless transitions.

## Building

```bash
# Generate protobuf code (run from repo root)
make proto

# Build the service
cd backend
go build ./services/regionservice/cmd/regionservice

# Run the service
go run ./services/regionservice/cmd/regionservice
```

## Docker

```bash
# Build container (from repo root)
make services-regionservice-container
```

## Development

For local development with Docker Compose infrastructure:

1. Start base infrastructure: `cd infra/dev/layer-00 && docker-compose up -d`
2. Start SwayRider services: `cd infra/dev/layer-20 && docker-compose up -d`

Development ports:
- REST API: 34003
- gRPC: 34103
