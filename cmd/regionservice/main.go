// Package main implements the regionservice binary.
//
// The regionservice provides geospatial region lookup and border crossing
// functionality for the SwayRider platform. It enables finding which regions
// contain a given point, box, or radius, and finding border crossing locations
// between regions.
//
// # Service Components
//
// The service initializes several components on startup:
//   - GeoDataReader for reading geodata files (region contours, border crossings)
//   - RegionIndex for spatial queries on region geometries
//   - BorderIndex for border crossing lookups between regions
//
// # Bootstrap Process
//
// On startup, the service reads and indexes geodata from the local filesystem:
//  1. Reads the manifest file from the geodata directory
//  2. Reads core and extended contours for each region
//  3. Reads border crossing data for region pairs
//  4. Builds spatial indices for efficient queries
//
// # Endpoints
//
// All endpoints are public (no authentication required):
//   - SearchPoint: Find regions containing a coordinate
//   - SearchBox: Find regions intersecting a bounding box
//   - SearchRadius: Find regions within a radius of a point
//   - FindCrossingLocations: Find border crossings between two regions
//   - FindRegionPath: Find a path of regions between two regions
package main

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	healthv1 "github.com/swayrider/protos/health/v1"
	regionv1 "github.com/swayrider/protos/region/v1"
	"github.com/swayrider/regionservice/internal/bootstrap"
	"github.com/swayrider/regionservice/internal/geodata"
	"github.com/swayrider/regionservice/internal/index"
	"github.com/swayrider/regionservice/internal/server"
	"github.com/swayrider/swlib/app"
	log "github.com/swayrider/swlib/logger"
)

// Configuration field constants.
const (
	FldGeoDataDir = "geodata-dir" // CLI flag name for geodata root directory
	EnvGeoDataDir = "GEODATA_DIR" // Environment variable name for geodata root directory
)

func main() {
	ri := index.NewRegionIndex()
	bi := index.NewBorderIndex()

	application := app.New("regionservice").
		WithDefaultConfigFields(app.BackendServiceFields, app.FlagGroupOverrides{}).
		WithConfigFields(
			app.NewStringConfigField(FldGeoDataDir, EnvGeoDataDir, "Root directory containing geodata", ""),
		).
		WithAppData("RegionIndex", ri).
		WithAppData("BorderIndex", bi).
		WithInitializers(bootstrapFn)

	grpcConfig := app.NewGrpcConfig(
		app.NoInterceptor, nil,
		app.GrpcServiceHooks{
			ServiceRegistrar:   grpcRegionRegistrar,
			ServiceHTTPHandler: grpcRegionGateway(application),
		},
		app.GrpcServiceHooks{
			ServiceRegistrar:   grpcHealthRegistrar,
			ServiceHTTPHandler: grpcHealthGateway(application),
		},
	)
	application = application.WithGrpc(grpcConfig)
	application.Run()
}

// bootstrapFn loads geodata from the filesystem and builds the spatial indices.
func bootstrapFn(a app.App) error {
	lg := a.Logger().Derive(log.WithFunction("bootstrap"))
	lg.Infoln("Bootstrapping service ...")

	geodataDir := app.GetConfigField[string](a.Config(), FldGeoDataDir)
	reader := geodata.NewGeoDataReader(geodataDir, a.Logger())
	ri := app.GetAppData[*index.RegionIndex](a, "RegionIndex")
	bi := app.GetAppData[*index.BorderIndex](a, "BorderIndex")

	err := bootstrap.Bootstrap(reader, ri, bi)
	if err != nil {
		lg.Fatalf("failed to bootstrap: %v", err)
	}
	return err
}

// grpcRegionRegistrar registers the RegionService gRPC server with the registrar.
func grpcRegionRegistrar(r grpc.ServiceRegistrar, a app.App) {
	ri := app.GetAppData[*index.RegionIndex](a, "RegionIndex")
	bi := app.GetAppData[*index.BorderIndex](a, "BorderIndex")
	srv := server.NewRegionServer(ri, bi, a.Logger())
	regionv1.RegisterRegionServiceServer(r, srv)
}

// grpcHealthRegistrar registers the HealthService gRPC server with the registrar.
func grpcHealthRegistrar(r grpc.ServiceRegistrar, a app.App) {
	srv := server.NewHealthServer(a.Logger())
	healthv1.RegisterHealthServiceServer(r, srv)
}

// grpcRegionGateway returns an HTTP handler that registers the RegionService
// REST gateway endpoints with the gRPC-gateway multiplexer.
func grpcRegionGateway(a app.App) app.ServiceHTTPHandler {
	return func(
		ctx context.Context,
		mux *runtime.ServeMux,
		endpoint string,
		opts []grpc.DialOption,
	) error {
		lg := a.Logger().Derive(log.WithFunction("RegionServiceHTTPHandler"))
		if err := regionv1.RegisterRegionServiceHandlerFromEndpoint(
			ctx, mux, endpoint, opts,
		); err != nil {
			lg.Fatalf("failed to register region gRPC gateway: %v", err)
		}
		return nil
	}
}

// grpcHealthGateway returns an HTTP handler that registers the HealthService
// REST gateway endpoints with the gRPC-gateway multiplexer.
func grpcHealthGateway(a app.App) app.ServiceHTTPHandler {
	return func(
		ctx context.Context,
		mux *runtime.ServeMux,
		endpoint string,
		opts []grpc.DialOption,
	) error {
		lg := a.Logger().Derive(log.WithFunction("HealthServiceHTTPHandler"))
		if err := healthv1.RegisterHealthServiceHandlerFromEndpoint(
			ctx, mux, endpoint, opts,
		); err != nil {
			lg.Fatalf("failed to register health gRPC gateway: %v", err)
		}
		return nil
	}
}
