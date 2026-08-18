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
// All RegionService endpoints require a user JWT or a service client token with
// the "region:query" scope. The health Ping endpoint is public:
//   - SearchPoint: Find regions containing a coordinate
//   - SearchBox: Find regions intersecting a bounding box
//   - SearchRadius: Find regions within a radius of a point
//   - FindCrossingLocations: Find border crossings between two regions
//   - FindRegionPath: Find a path of regions between two regions
package main

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"github.com/swayrider/grpcclients"
	"github.com/swayrider/grpcclients/authclient"
	healthv1 "github.com/swayrider/protos/health/v1"
	regionv1 "github.com/swayrider/protos/region/v1"
	"github.com/swayrider/regionservice/internal/bootstrap"
	"github.com/swayrider/regionservice/internal/geodata"
	"github.com/swayrider/regionservice/internal/index"
	"github.com/swayrider/regionservice/internal/server"
	"github.com/swayrider/swlib/app"
	"github.com/swayrider/swlib/cache"
	log "github.com/swayrider/swlib/logger"
)

// Configuration field constants.
const (
	FldGeoDataDir = "geodata-dir" // CLI flag name for geodata root directory
	EnvGeoDataDir = "GEODATA_DIR" // Environment variable name for geodata root directory

	jwtPublicKeys cache.LocalCacheKey = "jwt_public_keys"
)

func main() {
	keyChan := make(chan []string)

	ri := index.NewRegionIndex()
	bi := index.NewBorderIndex()

	application := app.New("regionservice").
		WithDefaultConfigFields(app.BackendServiceFields, app.FlagGroupOverrides{}).
		WithServiceClients(
			app.NewServiceClient("authservice", authServiceClientCtor),
		).
		WithConfigFields(
			app.NewStringConfigField(FldGeoDataDir, EnvGeoDataDir, "Root directory containing geodata", ""),
		).
		WithConfigFields(app.RateLimitConfigFields()...).
		WithAppData("RegionIndex", ri).
		WithAppData("BorderIndex", bi)

	grpcConfig := newGrpcConfig(application)

	application = application.
		WithBackgroundRoutines(
			publicKeyListener(keyChan),
			publicKeyFetcher(keyChan),
			app.RateLimitEvictor(grpcConfig),
		).
		WithInitializers(bootstrapFn, app.RateLimiterInitializer(grpcConfig)).
		WithGrpc(grpcConfig)
	application.Run()
}

// newGrpcConfig builds the gRPC server config with JWT authentication enabled.
// Every RegionService endpoint (declared in internal/server/server.go) requires
// a valid user JWT or a service client token with the "region:query" scope;
// only the health Ping endpoint is public.
func newGrpcConfig(application app.App) *app.GrpcConfig {
	return app.NewGrpcConfig(
		app.AuthInterceptor|app.RateLimitInterceptor,
		getPublicKeys,
		app.GrpcServiceHooks{
			ServiceRegistrar:   grpcRegionRegistrar,
			ServiceHTTPHandler: grpcRegionGateway(application),
		},
		app.GrpcServiceHooks{
			ServiceRegistrar:   grpcHealthRegistrar,
			ServiceHTTPHandler: grpcHealthGateway(application),
		},
	)
}

// authServiceClientCtor creates a new auth service gRPC client.
// This client is used to fetch JWT public keys for token verification.
func authServiceClientCtor(a app.App) grpcclients.Client {
	lg := a.Logger().Derive(log.WithFunction("authServiceClientCtor"))
	clnt, err := authclient.New(
		app.ServiceClientHostAndPort(a, "authservice"))
	if err != nil {
		lg.Fatalf("failed to create authservice client: %v", err)
	}
	return clnt
}

// publicKeyListener is a background routine that listens for JWT public key
// updates and stores them in the local cache.
func publicKeyListener(keyChan chan []string) func(app.App) {
	return func(a app.App) {
		ctx := a.BackgroundContext()
		defer a.BackgroundWaitGroup().Done()
		for {
			select {
			case <-ctx.Done():
				return
			case keys := <-keyChan:
				cache.LCSet(jwtPublicKeys, keys)
			}
		}
	}
}

// publicKeyFetcher is a background routine that periodically fetches JWT public
// keys from the authservice and sends them to the listener via the channel.
func publicKeyFetcher(keyChan chan []string) func(app.App) {
	return func(a app.App) {
		ctx := a.BackgroundContext()
		defer a.BackgroundWaitGroup().Done()
		clnt := app.GetServiceClient[*authclient.Client](a, "authservice")
		authclient.PublicKeyFetcher(ctx, clnt, keyChan)
	}
}

// getPublicKeys retrieves JWT public keys from the local cache.
// This function is called by the gRPC auth interceptor to verify tokens.
func getPublicKeys() ([]string, error) {
	keysIface, ok := cache.LCGet(jwtPublicKeys)
	if !ok {
		return nil, fmt.Errorf("no public keys found")
	}
	keys, ok := keysIface.([]string)
	if !ok {
		return nil, fmt.Errorf("invalid public keys")
	}
	return keys, nil
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
