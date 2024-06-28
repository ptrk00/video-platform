package main

import (
	"fmt"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"video-platform/handler/pkg/config"
	"video-platform/handler/pkg/queue"
)

const (
	minioHostOpt         = "MINIO_HOST"
	minioPortOpt         = "MINIO_PORT"
	minioUserOpt         = "MINIO_USER"
	minioPasswordOpt     = "MINIO_PASSWORD"
	minioSourceBucketOpt = "MINIO_SOURCE_BUCKET"
	minioDestBucketOpt   = "MINIO_DEST_BUCKET"
	natsURLOpt           = "NATS_URL"
	encryptionKeyOpt     = "ENCRYPTION_KEY"
)

func buildConfig() *config.ServerConfig {
	return &config.ServerConfig{
		MinioHost:         viper.GetString(minioHostOpt),
		MinioPort:         viper.GetInt(minioPortOpt),
		MinioUser:         viper.GetString(minioUserOpt),
		MinioPassword:     viper.GetString(minioPasswordOpt),
		MinioSourceBucket: viper.GetString(minioSourceBucketOpt),
		MinioDestBucket:   viper.GetString(minioDestBucketOpt),
		NatsURL:           viper.GetString(natsURLOpt),
		EncryptionKey:     viper.GetString(encryptionKeyOpt),
	}
}

var l *zap.SugaredLogger

func init() {
	logger := zap.Must(zap.NewDevelopment())
	defer logger.Sync()
	l = logger.Sugar()

	viper.SetDefault(minioHostOpt, "minio")
	viper.SetDefault(minioPortOpt, 9000)
	viper.SetDefault(minioSourceBucketOpt, "videos")
	viper.SetDefault(minioDestBucketOpt, "backup")
	viper.SetDefault(natsURLOpt, "nats://admin:admin@nats:4222")
	viper.SetConfigName("processor")
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

	// Connect to MinIO
	minioClient, err := minio.New(fmt.Sprintf("%s:%d", config.MinioHost, config.MinioPort), &minio.Options{
		Creds:  credentials.NewStaticV4(config.MinioUser, config.MinioPassword, ""),
		Secure: false,
	})
	if err != nil {
		l.Fatal("Failed to initialize minio client", zap.Error(err))
		return
	}

	// Connect to NATS
	nc, err := nats.Connect(config.NatsURL)
	if err != nil {
		l.Fatal("Failed to connect to NATS", zap.Error(err))
		return
	}
	defer nc.Close()

	// Subscribe to JetStream
	js, err := nc.JetStream()
	if err != nil {
		l.Fatal("Failed to get JetStream context", zap.Error(err))
		return
	}

	_, err = js.Subscribe("videos.uploaded", func(msg *nats.Msg) {
		queue.HandleMessage(msg, minioClient, config, l)
	})
	if err != nil {
		l.Fatal("Failed to subscribe to subject", zap.Error(err))
		return
	}

	select {} // Keep the service running
}
