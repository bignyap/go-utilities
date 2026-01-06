// Package grpc provides OpenTelemetry instrumentation for gRPC servers and clients.
// It wraps the official otelgrpc package with convenient helper functions.
//
// The otelgrpc package uses stats handlers for instrumentation, which is the
// recommended approach for gRPC instrumentation in OpenTelemetry.
package grpc

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
)

// NewServerHandler returns a gRPC stats handler for server-side OpenTelemetry instrumentation.
// It automatically creates spans for each RPC call and records metrics.
//
// Usage:
//
//	server := grpc.NewServer(
//	    grpc.StatsHandler(otelgrpc.NewServerHandler()),
//	)
func NewServerHandler(opts ...otelgrpc.Option) stats.Handler {
	return otelgrpc.NewServerHandler(opts...)
}

// NewClientHandler returns a gRPC stats handler for client-side OpenTelemetry instrumentation.
// It automatically creates spans for each outgoing RPC call and records metrics.
//
// Usage:
//
//	conn, err := grpc.Dial(address,
//	    grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
//	)
func NewClientHandler(opts ...otelgrpc.Option) stats.Handler {
	return otelgrpc.NewClientHandler(opts...)
}

// NewServerWithTelemetry creates a new gRPC server with OpenTelemetry instrumentation.
// It applies the stats handler for automatic tracing and metrics.
//
// Usage:
//
//	server := otelgrpc.NewServerWithTelemetry()
//	pb.RegisterMyServiceServer(server, &myService{})
func NewServerWithTelemetry(opts ...grpc.ServerOption) *grpc.Server {
	// Prepend telemetry stats handler to any provided options
	telemetryOpts := []grpc.ServerOption{
		grpc.StatsHandler(NewServerHandler()),
	}

	allOpts := append(telemetryOpts, opts...)
	return grpc.NewServer(allOpts...)
}

// DialOptionsWithTelemetry returns gRPC dial options with OpenTelemetry instrumentation.
// Use these options when creating a gRPC client connection.
//
// Usage:
//
//	opts := otelgrpc.DialOptionsWithTelemetry()
//	conn, err := grpc.Dial(address, opts...)
func DialOptionsWithTelemetry() []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithStatsHandler(NewClientHandler()),
	}
}
