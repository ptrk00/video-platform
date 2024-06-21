package main

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"github.com/ulikunitz/xz"
	"go.uber.org/zap"
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

type ServerConfig struct {
	MinioHost         string
	MinioPort         int
	MinioUser         string
	MinioPassword     string
	MinioSourceBucket string
	MinioDestBucket   string
	NatsURL           string
	EncryptionKey     string
}

type Message struct {
	Bucket string `json:"bucket"`
	Filename   string `json:"filename"`
}

func buildConfig() *ServerConfig {
	return &ServerConfig{
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
		handleMessage(msg, minioClient, config)
	})
	if err != nil {
		l.Fatal("Failed to subscribe to subject", zap.Error(err))
		return
	}

	select {} // Keep the service running
}

func handleMessage(msg *nats.Msg, minioClient *minio.Client, config *ServerConfig) {
	var message Message
	err := json.Unmarshal(msg.Data, &message)
	if err != nil {
		l.Error("Failed to unmarshal message", zap.Error(err))
		return
	}

	l.Infof("Processing file with ETag: %s from bucket: %s", message.Filename, message.Bucket)

	// Download the file
	object, err := minioClient.GetObject(context.Background(), message.Bucket, message.Filename, minio.GetObjectOptions{})
	if err != nil {
		l.Error("Failed to get object from MinIO", zap.Error(err))
		return
	}
	defer object.Close()

	// Encrypt the file
	encryptedData, err := encryptData(object, config.EncryptionKey)
	if err != nil {
		l.Error("Failed to encrypt data", zap.Error(err))
		return
	}

	// Compress the file
	compressedData, err := compressData(encryptedData)
	if err != nil {
		l.Error("Failed to compress data", zap.Error(err))
		return
	}

	// Store the file in the destination bucket
	_, err = minioClient.PutObject(context.Background(), config.MinioDestBucket, message.Filename, compressedData, -1, minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		l.Error("Failed to put object to MinIO", zap.Error(err))
		return
	}

	l.Infof("Successfully processed and stored file with ETag: %s to bucket: %s", message.Filename, config.MinioDestBucket)
}

func encryptData(reader io.Reader, key string) (io.Reader, error) {
	block, err := aes.NewCipher([]byte(createHash(key)))
	if err != nil {
		l.Error("aes failed")
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		l.Error("gcm failed")
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		l.Error("nonce failed")
		return nil, err
	}

	var buf bytes.Buffer
	buf.Write(nonce)

	data, err := io.ReadAll(reader)
	if err != nil {
		l.Error("reader failed")
		return nil, err
	}

	encryptedData := gcm.Seal(nil, nonce, data, nil)
	buf.Write(encryptedData)

	return &buf, nil
}

func compressData(reader io.Reader) (io.Reader, error) {
	var buf bytes.Buffer
	writer, err := xz.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(writer, reader); err != nil {
		writer.Close()
		return nil, err
	}
	writer.Close()

	return &buf, nil
}

func createHash(key string) string {
	hasher := md5.New()
	hasher.Write([]byte(key))
	return hex.EncodeToString(hasher.Sum(nil))
}
