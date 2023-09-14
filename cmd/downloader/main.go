package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/airbusgeo/geocube/cmd"
	geogrpc "github.com/airbusgeo/geocube/internal/grpc"
	"github.com/airbusgeo/geocube/internal/log"
	pb "github.com/airbusgeo/geocube/internal/pb"
	"github.com/airbusgeo/geocube/internal/svc"
)

func main() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	ctx, cancel := context.WithCancel(context.Background())
	runerr := make(chan error)

	go func() {
		runerr <- run(ctx)
	}()

	for {
		select {
		case err := <-runerr:
			if err != nil {
				log.Logger(ctx).Fatal("run error", zap.Error(err))
			}
			return
		case <-quit:
			cancel()
			go func() {
				time.Sleep(30 * time.Second)
				runerr <- fmt.Errorf("did not terminate after 30 seconds")
			}()
		}
	}
}

func run(ctx context.Context) error {
	downloaderConfig, err := newDownloaderAppConfig()
	if err != nil {
		return err
	}

	if err := cmd.InitGDAL(ctx, downloaderConfig.GDALConfig); err != nil {
		return fmt.Errorf("init gdal: %w", err)
	}

	// Create Geocube Service
	svc, err := svc.New(ctx, nil, nil, nil, "", "", downloaderConfig.CubeWorkers)
	if err != nil {
		return fmt.Errorf("svc.new: %w", err)
	}

	grpcServer := newGrpcServer(svc, downloaderConfig.MaxConnectionAge, downloaderConfig.ChunkSizeByte)

	log.Logger(ctx).Info("Geocube v" + geogrpc.GeocubeServerVersion)

	muxHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		grpcServer.ServeHTTP(w, r)
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", downloaderConfig.AppPort),
		Handler: h2c.NewHandler(muxHandler, &http2.Server{}),
	}

	go func() {
		var err error
		if downloaderConfig.TLS {
			err = srv.ListenAndServeTLS("/tls/tls.crt", "/tls/tls.key")
		} else {
			err = srv.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			log.Logger(ctx).Fatal("srv.ListenAndServe", zap.Error(err))
		}
	}()

	<-ctx.Done()
	sctx, cncl := context.WithTimeout(context.Background(), 30*time.Second)
	defer cncl()
	return srv.Shutdown(sctx)
}

func newGrpcServer(svc geogrpc.GeocubeDownloaderService, maxConnectionAgeValue int, chunkSizeBytes int) *grpc.Server {
	opts := []grpc.ServerOption{
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionAge:      time.Duration(maxConnectionAgeValue) * time.Second,
			MaxConnectionAgeGrace: time.Minute})}

	grpcServer := grpc.NewServer(opts...)
	pb.RegisterGeocubeDownloaderServer(grpcServer, geogrpc.NewDownloader(svc, maxConnectionAgeValue, chunkSizeBytes))
	return grpcServer
}

func newDownloaderAppConfig() (*serverConfig, error) {
	serverConfig := serverConfig{}

	flag.BoolVar(&serverConfig.TLS, "tls", false, "enable TLS protocol")
	flag.StringVar(&serverConfig.AppPort, "port", "8080", "geocube downloader port to use")
	flag.IntVar(&serverConfig.MaxConnectionAge, "maxConnectionAge", 15*60, "grpc max age connection in seconds")
	flag.IntVar(&serverConfig.CubeWorkers, "workers", 1, "number of workers to parallelize the processing of the slices of a cube (see also GdalMultithreading)")
	flag.IntVar(&serverConfig.ChunkSizeByte, "chunk-size", 1024*1024, "chunk size for grpc streaming of images in bytes. If an image is bigger than chunk_size_bytes, it is divided into chunks and streamed. Grpc recommends a chunk_size of 64kbytes, but in localhost, performances are better with a bigger chunk_size, such as 1Mbytes. By default, chunk_size is limited by Grpc to 4Mbytes.")
	serverConfig.GDALConfig = cmd.GDALConfigFlags()

	flag.Parse()

	if serverConfig.AppPort == "" {
		return nil, fmt.Errorf("failed to initialize --port application flag")
	}

	return &serverConfig, nil
}

type serverConfig struct {
	TLS              bool
	AppPort          string
	MaxConnectionAge int
	CubeWorkers      int
	ChunkSizeByte    int
	GDALConfig       *cmd.GDALConfig
}
