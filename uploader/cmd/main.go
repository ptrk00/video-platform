package main

import (
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/spf13/viper"
	"go.uber.org/zap"
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
)

type ServerConfig struct {
	Port              int
	MinioHost         string
	MinioPort         int
	MinioUser         string
	MinioPassword     string
	MinioBucket       string
	VideoFormFilename string
	PostgresDSN       string
}

func buildConfig() *ServerConfig {
	return &ServerConfig{
		Port:              viper.GetInt(portOpt),
		MinioHost:         viper.GetString(minioHostOpt),
		MinioPort:         viper.GetInt(minioPortOpt),
		MinioUser:         viper.GetString(minioUserOpt),
		MinioPassword:     viper.GetString(minioPasswordOpt),
		MinioBucket:       viper.GetString(minioBucketOpt),
		VideoFormFilename: viper.GetString(videoFormFilenameOpt),
		PostgresDSN:       viper.GetString(postgresDSNOpt),
	}
}

var l *zap.SugaredLogger

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

func publishMessage(bucketname, filename string) {
	// Connect to a NATS server
	nc, err := nats.Connect("nats://admin:admin@nats:4222")
	if err != nil {
		zap.Error(err)
	}
	defer nc.Close()

	// Get JetStream context
	js, err := nc.JetStream()
	if err != nil {
		zap.Error(err)
	}

	// Publish a message
	msg := nats.NewMsg("videos.uploaded")
	msg.Data = []byte(fmt.Sprintf("%s:%s", bucketname, filename))
	msg.Header.Add("time", time.Now().String())

	// Send the message
	ack, err := js.PublishMsg(msg)
	if err != nil {
		zap.Error(err)
	}

	log.Printf("Published message on subject %s with sequence %d\n", msg.Subject, ack.Sequence)
}

func computeChecksum(reader io.Reader) (string, string, error) {
	hashMd5 := md5.New()
	hashSha256 := sha256.New()
	tee := io.MultiWriter(hashMd5, hashSha256)

	if _, err := io.Copy(tee, reader); err != nil {
		return "", "", err
	}

	return hex.EncodeToString(hashMd5.Sum(nil)), hex.EncodeToString(hashSha256.Sum(nil)), nil
}

func storeFileMetadata(db *sql.DB, filename string, filesize int64, contentType, etag, fileURL, checksum string, userID int) error {
	query := `INSERT INTO files (filename, filesize, content_type, etag, file_url, checksum, user_id) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := db.Exec(query, filename, filesize, contentType, etag, fileURL, checksum, userID)
	return err
}

func main() {
	l.Debug("Reading configuration")
	config := buildConfig()

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

	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		l.Debugw("Handling video upload")
		if r.Method != "POST" {
			http.Error(w, "Unsupported method", http.StatusMethodNotAllowed)
			return
		}

		// Parse the multipart form
		r.ParseMultipartForm(10 << 20) // Limit upload size

		file, handler, err := r.FormFile(config.VideoFormFilename)
		if err != nil {
			l.Errorw("Could not parse the multipart file",
				zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error parsing file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Get the file size and content type
		fileSize := handler.Size
		contentType := handler.Header.Get("Content-Type")

		// Create a new reader to compute the checksum and upload the file
		file.Seek(0, io.SeekStart)
		_, sha256Checksum, err := computeChecksum(file)
		if err != nil {
			l.Errorw("Could not compute checksum", zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error computing checksum", http.StatusInternalServerError)
			return
		}

		// Upload the file to MinIO
		file.Seek(0, io.SeekStart)
		l.Infow("Uploading file", zap.String("bucketname", config.MinioBucket),
			zap.String("filename", handler.Filename))
		info, err := minioClient.PutObject(r.Context(), config.MinioBucket, handler.Filename, file, fileSize, minio.PutObjectOptions{ContentType: contentType})
		if err != nil {
			l.Errorw("Could not upload file", zap.String("bucketname", config.MinioBucket),
				zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error uploading file", http.StatusInternalServerError)
			return
		}

		// Get the file URL and ETag
		fileURL := fmt.Sprintf("http://%s/%s/%s", config.MinioHost, config.MinioBucket, handler.Filename)
		etag := info.ETag

		// Store metadata in PostgreSQL
		err = storeFileMetadata(db, handler.Filename, fileSize, contentType, etag, fileURL, sha256Checksum, 1) // Assuming userID is 1 for this example
		if err != nil {
			l.Errorw("Could not store file metadata", zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error storing file metadata", http.StatusInternalServerError)
			return
		}

		l.Infow("Successfully uploaded file", zap.String("bucketname", config.MinioBucket),
			zap.String("filename", handler.Filename))
		fmt.Fprintf(w, "Successfully uploaded %s\n", handler.Filename)
		publishMessage(config.MinioBucket, handler.Filename)
	})

	http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil)
}
