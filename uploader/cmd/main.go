package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"net/http"
	"video-platform/uploader/pkg/auth"
	"video-platform/uploader/pkg/config"
	"video-platform/uploader/pkg/handlers"
	"video-platform/uploader/pkg/monitoring"
)

const (
	portOpt              = "PORT"
	minioHostOpt         = "MINIO_HOST"
	minioPortOpt         = "MINIO_PORT"
	minioUserOpt         = "MINIO_USER"
	minioPasswordOpt     = "MINIO_PASSWORD"
	minioBucketOpt       = "MINIO_BUCKET"
	videoFormFilenameOpt = "VIDEO_FORM_FILENAME"
	postgresDSNOpt       = "POSTGRES_DSN"
	jaegerEndpointOpt    = "JAEGER_ENDPOINT"
)

func buildConfig() *config.ServerConfig {
	return &config.ServerConfig{
		Port:              viper.GetInt(portOpt),
		MinioHost:         viper.GetString(minioHostOpt),
		MinioPort:         viper.GetInt(minioPortOpt),
		MinioUser:         viper.GetString(minioUserOpt),
		MinioPassword:     viper.GetString(minioPasswordOpt),
		MinioBucket:       viper.GetString(minioBucketOpt),
		VideoFormFilename: viper.GetString(videoFormFilenameOpt),
		PostgresDSN:       viper.GetString(postgresDSNOpt),
		JaegerEndpoint:    viper.GetString(jaegerEndpointOpt),
	}
}

var (
	l *zap.SugaredLogger
)

func init() {
	logger := zap.Must(zap.NewDevelopment())
	defer logger.Sync()
	l = logger.Sugar()

	viper.SetDefault(portOpt, 8080)
	viper.SetDefault(portOpt, "localhost")
	viper.SetDefault(portOpt, 9000)
	viper.SetConfigName("uploader")
	viper.SetConfigType("props")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	err := viper.ReadInConfig()
	if err != nil {
		l.Info("Did not find config file... Configuring from envs")
	}

}

func main() {
	l.Debug("Reading configuration")
	config := buildConfig()

	// Initialize OpenTelemetry tracer
	tp, err := monitoring.InitTracer(config)
	if err != nil {
		l.Fatal("Failed to initialize OpenTelemetry tracer", zap.Error(err))
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			l.Fatal("Failed to shutdown OpenTelemetry tracer", zap.Error(err))
		}
	}()

	// Connect to PostgreSQL
	db, err := sql.Open("pgx", "postgresql://postgres:5432/videos?user=postgres&password=postgres")
	if err != nil {
		l.Fatal("Failed to connect to PostgreSQL", zap.Error(err))
	}
	defer db.Close()

	// Initialize MinIO client
	minioClient, err := minio.New(fmt.Sprintf("%s:%d", config.MinioHost, config.MinioPort), &minio.Options{
		Creds:  credentials.NewStaticV4(config.MinioUser, config.MinioPassword, ""),
		Secure: false,
	})
	if err != nil {
		l.Error("Failed to initialize minio client", zap.Error(err))
		return
	}

	http.HandleFunc("/login", handlers.Login(db, l))
	http.Handle("/upload", auth.Authenticate(handlers.UploadFileHandler(config, db, minioClient, l), l))
	http.Handle("/files", auth.Authenticate(handlers.GetUserFiles(db, l), l))
	http.Handle("/download", auth.Authenticate(handlers.DownloadFile(db, minioClient, config.MinioBucket, l), l))

	// Expose the /metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	http.ListenAndServe(fmt.Sprintf(":%d", config.Port), otelhttp.NewHandler(http.DefaultServeMux, "Server"))
}
