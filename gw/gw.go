// Copyright (c) 2020 rookie-ninja
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package rk_gw

import (
	"context"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/rookie-ninja/rk-boot/api"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"net/http"
	"os"
	"strconv"
)

type GRpcGWEntry struct {
	logger              *zap.Logger
	httpPort            uint64
	gRpcPort            uint64
	enableCommonService bool
	regFuncs            []RegFunc
	dialOpts            []grpc.DialOption
	muxOpts             []runtime.ServeMuxOption
	server              *http.Server
}

type RegFunc func(context.Context, *runtime.ServeMux, string, []grpc.DialOption) error

type GRpcGWOption func(*GRpcGWEntry)

func WithHttpPort(port uint64) GRpcGWOption {
	return func(entry *GRpcGWEntry) {
		entry.httpPort = port
	}
}

func WithCommonService(enable bool) GRpcGWOption {
	return func(entry *GRpcGWEntry) {
		entry.enableCommonService = enable
	}
}

func WithGRpcPort(port uint64) GRpcGWOption {
	return func(entry *GRpcGWEntry) {
		entry.gRpcPort = port
	}
}

func WithRegFuncs(funcs ...RegFunc) GRpcGWOption {
	return func(entry *GRpcGWEntry) {
		entry.regFuncs = append(entry.regFuncs, funcs...)
	}
}

func WithDialOptions(opts ...grpc.DialOption) GRpcGWOption {
	return func(entry *GRpcGWEntry) {
		entry.dialOpts = append(entry.dialOpts, opts...)
	}
}

func NewGRpcGWEntry(opts ...GRpcGWOption) *GRpcGWEntry {
	entry := &GRpcGWEntry{
		logger: zap.NewNop(),
	}

	for i := range opts {
		opts[i](entry)
	}

	if entry.dialOpts == nil {
		entry.dialOpts = make([]grpc.DialOption, 0)
	}

	if entry.regFuncs == nil {
		entry.regFuncs = make([]RegFunc, 0)
	}

	if entry.enableCommonService {
		entry.regFuncs = append(entry.regFuncs, api.RegisterRkCommonServiceHandlerFromEndpoint)
	}

	return entry
}

func (entry *GRpcGWEntry) AddDialOptions(opts ...grpc.DialOption) {
	entry.dialOpts = append(entry.dialOpts, opts...)
}

func (entry *GRpcGWEntry) AddRegFuncs(funcs ...RegFunc) {
	entry.regFuncs = append(entry.regFuncs, funcs...)
}

func (entry *GRpcGWEntry) GetHttpPort() uint64 {
	return entry.httpPort
}

func (entry *GRpcGWEntry) GetGRpcPort() uint64 {
	return entry.gRpcPort
}

func (entry *GRpcGWEntry) GetServer() *http.Server {
	return entry.server
}

func (entry *GRpcGWEntry) Stop(logger *zap.Logger) {
	if entry.server != nil {
		logger.Info("stopping gRpc gateway",
			zap.Uint64("http_port", entry.httpPort),
			zap.Uint64("gRpc_port", entry.gRpcPort))
		entry.server.Shutdown(context.Background())
	}
}

func (entry *GRpcGWEntry) Start(logger *zap.Logger) {
	gRPCEndpoint := "0.0.0.0:" + strconv.FormatUint(entry.gRpcPort, 10)
	httpEndpoint := "0.0.0.0:" + strconv.FormatUint(entry.httpPort, 10)

	gwMux := runtime.NewServeMux(entry.muxOpts...)

	for i := range entry.regFuncs {
		err := entry.regFuncs[i](context.Background(), gwMux, gRPCEndpoint, entry.dialOpts)
		if err != nil {
			logger.Error("registering functions",
				zap.Uint64("http_port", entry.httpPort),
				zap.Uint64("gRpc_port", entry.gRpcPort),
				zap.Error(err))
			shutdownWithError(err)
		}
	}

	httpMux := http.NewServeMux()
	httpMux.Handle("/", gwMux)

	// Support head method
	entry.server = &http.Server{
		Addr:    httpEndpoint,
		Handler: headMethodHandler(httpMux),
	}

	go func(entry *GRpcGWEntry) {
		logger.Info("starting gRpc gateway",
			zap.Uint64("http_port", entry.httpPort),
			zap.Uint64("gRpc_port", entry.gRpcPort))
		if err := entry.server.ListenAndServe(); err != nil {
			entry.logger.Error("failed to start gRpc gateway",
				zap.Uint64("gRpc_port", entry.gRpcPort),
				zap.Uint64("http_port", entry.httpPort),
				zap.Error(err))
			shutdownWithError(err)
		}
	}(entry)
}

// Support HEAD request
func headMethodHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			return
		}
		h.ServeHTTP(w, r)
	})
}

func shutdownWithError(err error) {
	// log it
	os.Exit(1)
}