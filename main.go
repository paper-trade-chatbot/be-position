package main

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/paper-trade-chatbot/be-position/cache"
	"github.com/paper-trade-chatbot/be-position/cronjob"
	"github.com/paper-trade-chatbot/be-position/database"
	"github.com/paper-trade-chatbot/be-position/service/position"
	positionGrpc "github.com/paper-trade-chatbot/be-proto/position"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	"github.com/paper-trade-chatbot/be-position/config"
	"github.com/paper-trade-chatbot/be-position/global"
	"github.com/paper-trade-chatbot/be-position/logging"
	"github.com/paper-trade-chatbot/be-position/pubsub"
	"github.com/paper-trade-chatbot/be-position/server"
)

func main() {

	// We're running, turn on the liveness indication flag.
	global.Alive = true

	// Create root context.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup logging module.
	// NOTE: This should always be first.
	logging.Initialize(ctx)
	defer logging.Finalize()

	// // Setup redis
	cache.Initialize(ctx)
	defer cache.Finalize()

	// // Setup quote redis
	// redisQuote.Initialize(ctx)
	// defer redisQuote.Finalize()

	// // Setup cache module.
	// redisCluster.Initialize(ctx)
	// defer redisCluster.Finalize()

	// // Setup database module.
	database.Initialize(ctx)
	defer database.Finalize()

	// service.Initialize(ctx)
	// defer service.Finalize(ctx)

	pubsub.Initialize(ctx)
	defer pubsub.Finalize(ctx)

	initConfig()

	grpcAddress := fmt.Sprintf("%s:%s",
		config.GetString("GRPC_SERVER_LISTEN_ADDRESS"),
		config.GetString("GRPC_SERVER_LISTEN_PORT"))
	listener := server.CreateGRpcServer(ctx, grpcAddress)

	recoveryOpt := grpc_recovery.WithRecoveryHandlerContext(
		func(ctx context.Context, p interface{}) error {
			logging.Error(ctx, "[PANIC] %s\n\n%s", p, string(debug.Stack()))
			return status.Errorf(codes.Internal, "%s", p)
		},
	)
	grpc := grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_recovery.StreamServerInterceptor(recoveryOpt),
		)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_recovery.UnaryServerInterceptor(recoveryOpt),
		)),
	)
	reflection.Register(grpc)

	positionInstance := position.New()
	positionGrpc.RegisterPositionServiceServer(grpc, positionInstance)

	// Create HTTP server instance to listen on all interfaces.
	address := fmt.Sprintf("%s:%s",
		config.GetString("SERVER_LISTEN_ADDRESS"),
		config.GetString("SERVER_LISTEN_PORT"))
	httpServer := server.CreateHttpServer(ctx, address)

	// run cron job
	go cronjob.Cron()

	go func() {
		logging.Info(ctx, "grpc serving")
		if err := grpc.Serve(*listener); err != nil {
			panic(err)
		}
	}()

	// Now that we finished initializing all necessary modules,
	// let's turn on the readiness indication flag.
	global.Ready = true

	// Start servicing requests.
	logging.Info(ctx, "Initialization complete, listening on %s...", address)
	if err := httpServer.ListenAndServe(); err != nil {
		logging.Info(ctx, err.Error())
	}

}

func initConfig() {
	global.Initialize()

}