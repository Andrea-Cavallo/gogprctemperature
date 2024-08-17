package main

import (
	"context"
	"fmt"
	"go_with_grpc/pkg/telemetry"
	"go_with_grpc/pkg/temperature"
	"go_with_grpc/temperature_grpc_server/mongodb/config"
	"go_with_grpc/temperature_grpc_server/service"
	"google.golang.org/grpc"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof" // Importa il package per abilitare pprof
	"os"
	"os/signal"
	"syscall"
)

const (
	grpcAddress = ":50051"
	tcp         = "tcp"
)

func main() {

	// Crea un context con la gestione dei segnali per interrompere il server gRPC
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Avvia un server HTTP separato per il profiling pprof
	go func() {
		log.Println("Starting pprof server on :6060")
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			log.Fatalf("Failed to start pprof server: %v", err)
		}
	}()

	// Inizializza e configura OpenTelemetry
	shutdownTelemetry := initializeTelemetry()
	defer func() {
		if err := shutdownTelemetry(context.Background()); err != nil {
			log.Fatalf("Failed to shut down OpenTelemetry: %v", err)
		}
	}()

	// Avvia il server gRPC
	if err := startGRPCServer(ctx, grpcAddress); err != nil {
		log.Fatalf("Failed to start gRPC server: %v", err)
	}

	// Attendo segnali di fumo per interrompere il server
	<-ctx.Done()
	log.Println("Server stopped.")
}

func initializeTelemetry() func(context.Context) error {
	return telemetry.InitTelemetry()
}

// startGRPCServer starts a gRPC server on the specified address.
// It takes a context and the address as input parameters.
// It listens on the specified address and handles incoming gRPC requests.
// It registers the TemperatureServiceServer provided by service.NewServer.
// It shuts down the gRPC server and closes the MongoDB client when the context is done.
// It returns an error if any error occurs during the process.
func startGRPCServer(ctx context.Context, address string) error {
	listener, err := net.Listen(tcp, address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}
	defer listener.Close()

	grpcServer := grpc.NewServer()

	go func() {
		<-ctx.Done()
		log.Println("Shutting down gRPC server...")
		grpcServer.GracefulStop()
		log.Println("gRPC server stopped.")
		config.CloseMongoClient()
		log.Println("Closing MongoDB client... done. Goodbye! ^^")
	}()

	temperature.RegisterTemperatureServiceServer(grpcServer, service.NewServer())
	log.Printf("gRPC server listening on %s\n", address)
	if err := grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("failed to serve gRPC server: %w", err)
	}

	return nil
}
