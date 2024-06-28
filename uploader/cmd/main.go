package main

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	_ "github.com/jackc/pgx/v4/stdlib"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
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
	jwtSecret            = "supersecretkey"
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
	JaegerEndpoint    string
}

type Message struct {
	Bucket   string `json:"bucket"`
	Filename string `json:"filename"`
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Claims struct {
	Username string `json:"username"`
	ID       int    `json:"id"`
	jwt.StandardClaims
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
		JaegerEndpoint:    viper.GetString(jaegerEndpointOpt),
	}
}

var (
	l               *zap.SugaredLogger
	fileUploadCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "file_upload_count",
			Help: "Number of files uploaded",
		},
	)
)

func initTracer(config *ServerConfig) (*sdktrace.TracerProvider, error) {
	exp, err := jaeger.New(jaeger.WithCollectorEndpoint(jaeger.WithEndpoint(config.JaegerEndpoint)))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("uploader"),
		)),
	)
	otel.SetTracerProvider(tp)
	return tp, nil
}

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

	// Register Prometheus metrics
	prometheus.MustRegister(fileUploadCount)
}

func publishMessage(ctx context.Context, bucketname, filename string) {
	tracer := otel.Tracer("uploader")
	ctx, span := tracer.Start(ctx, "publishMessage")
	defer span.End()

	// Connect to a NATS server
	nc, err := nats.Connect("nats://admin:admin@nats:4222")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to connect to NATS")
		zap.Error(err)
		return
	}
	defer nc.Close()

	// Get JetStream context
	js, err := nc.JetStream()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to get JetStream context")
		zap.Error(err)
		return
	}

	// Create the message
	message := Message{
		Bucket:   bucketname,
		Filename: filename,
	}
	data, err := json.Marshal(message)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to marshal message")
		zap.Error(err)
		return
	}

	// Publish the message
	msg := nats.NewMsg("videos.uploaded")
	msg.Data = data
	msg.Header.Add("time", time.Now().String())

	// Send the message
	ack, err := js.PublishMsg(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to publish message")
		zap.Error(err)
		return
	}

	span.SetAttributes(attribute.Int64("nats.sequence", int64(ack.Sequence)))
	log.Printf("Published message on subject %s with sequence %d\n", msg.Subject, ack.Sequence)
}

func computeChecksum(ctx context.Context, reader io.Reader) (string, string, error) {
	_, span := otel.Tracer("uploader").Start(ctx, "computeChecksum")
	defer span.End()

	hashMd5 := md5.New()
	hashSha256 := sha256.New()
	tee := io.MultiWriter(hashMd5, hashSha256)

	if _, err := io.Copy(tee, reader); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Failed to compute checksum")
		return "", "", err
	}

	return hex.EncodeToString(hashMd5.Sum(nil)), hex.EncodeToString(hashSha256.Sum(nil)), nil
}

func storeFileMetadata(ctx context.Context, db *sql.DB, filename string, filesize int64, contentType, etag, fileURL, checksum string, userID int) error {
	tracer := otel.Tracer("uploader")
	_, span := tracer.Start(ctx, "storeFileMetadata")
	defer span.End()

	// Add attributes to the span
	span.SetAttributes(
		attribute.String("filename", filename),
		attribute.Int64("filesize", filesize),
		attribute.String("content_type", contentType),
		attribute.String("etag", etag),
		attribute.String("file_url", fileURL),
		attribute.String("checksum", checksum),
		attribute.Int("user_id", userID),
	)

	// Add an event to the span
	span.AddEvent("Storing file metadata in the database", trace.WithAttributes(
		attribute.String("filename", filename),
		attribute.Int64("filesize", filesize),
		attribute.String("content_type", contentType),
	))

	query := `INSERT INTO files (filename, filesize, content_type, etag, file_url, checksum, user_id) VALUES ($1, $2, $3, $4, $5, $6, $7)`
	_, err := db.ExecContext(ctx, query, filename, filesize, contentType, etag, fileURL, checksum, userID)
	if err != nil {
		span.SetStatus(codes.Error, "Failed to execute query")
		span.RecordError(err)
	}
	return err
}

func generateJWT(username string, id int) (string, error) {
	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &Claims{
		Username: username,
		ID:       id,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func login(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var creds Credentials
		err := json.NewDecoder(r.Body).Decode(&creds)
		if err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		// Query the user from the database
		var storedPassword string
		var userID int
		err = db.QueryRow("SELECT id, password FROM app_users WHERE username=$1", creds.Username).Scan(&userID, &storedPassword)
		if err != nil {
			if err == sql.ErrNoRows {
				l.Error(err)
				http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			} else {
				l.Error(err)
				http.Error(w, "Server error", http.StatusInternalServerError)
			}
			return
		}

		// Compare the stored hashed password with the provided password
		err = bcrypt.CompareHashAndPassword([]byte(storedPassword), []byte(creds.Password))
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		token, err := generateJWT(creds.Username, userID)
		if err != nil {
			http.Error(w, "Error generating token", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{"token": token})
	}
}

func authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			l.Info("no auth header")
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenStr := authHeader[len("Bearer "):]

		// Parse and validate the token
		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check the token against the OPA policy
		_, err = checkOPAPolicy(tokenStr)
		if err != nil {
			l.Errorf("error checking opa policy: %v", err)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "username", claims.Username)
		if claims.Username == "admin" {
			ctx = context.WithValue(ctx, "admin", true)
		} else {
			ctx = context.WithValue(ctx, "admin", false)
		}
		ctx = context.WithValue(ctx, "id", claims.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func checkOPAPolicy(tokenStr string) (map[string]interface{}, error) {
	ctx := context.Background()
	input := map[string]interface{}{
		"input": map[string]interface{}{
			"token": tokenStr,
		},
	}

	inputData, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	opaURL := "http://opa:8181/v1/data/authz/allow"
	req, err := http.NewRequestWithContext(ctx, "POST", opaURL, bytes.NewBuffer(inputData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authorization failed: %s", resp.Status)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	allowed, ok := result["result"].(bool)
	if !ok || !allowed {
		return nil, fmt.Errorf("authorization failed")
	}
	l.Info(result)
	return result, nil
}

func getUserFiles(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value("id").(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		isAdmin, _ := r.Context().Value("admin").(bool)
		var rows *sql.Rows
		var err error
		if isAdmin {
			rows, err = db.Query("SELECT filename, filesize, content_type, etag, file_url, checksum, upload_timestamp FROM files")
			if err != nil {
				l.Error(err)
				http.Error(w, "Server error", http.StatusInternalServerError)
				return
			}
		} else {
			rows, err = db.Query("SELECT filename, filesize, content_type, etag, file_url, checksum, upload_timestamp FROM files WHERE user_id=$1", userID)
			if err != nil {
				l.Error(err)
				http.Error(w, "Server error", http.StatusInternalServerError)
				return
			}
		}
		defer rows.Close()

		var files []map[string]interface{}
		for rows.Next() {
			var file map[string]interface{}
			var filename, contentType, etag, fileURL, checksum string
			var uploaded_timestamp time.Time

			var filesize int64
			if err := rows.Scan(&filename, &filesize, &contentType, &etag, &fileURL, &checksum, &uploaded_timestamp); err != nil {
				l.Error(err)
				http.Error(w, "Server error", http.StatusInternalServerError)
				return
			}

			deleted := time.Now().After(uploaded_timestamp.Add(2*time.Minute))
			l.Infof("Time now is %s", time.Now())
			l.Infof("File %s is marked as %t due to %s uploaded_tiemstamp", filename, deleted, uploaded_timestamp.String())
			file = map[string]interface{}{
				"filename":     filename,
				"filesize":     filesize,
				"content_type": contentType,
				"etag":         etag,
				"file_url":     fileURL,
				"checksum":     checksum,
				"deleted": 		deleted,
			}
			files = append(files, file)
		}
		if err := rows.Err(); err != nil {
			l.Error(err)
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(files)
	}
}

func downloadFile(db *sql.DB, minioClient *minio.Client, bucketName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := r.Context().Value("id").(int)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		etag := r.URL.Query().Get("etag")
		if etag == "" {
			http.Error(w, "Missing etag", http.StatusBadRequest)
			return
		}

		// Verify that the file belongs to the user
		var filename, contentType string
		var err error
		isAdmin, _ := r.Context().Value("admin").(bool)

		if isAdmin {
			err = db.QueryRow("SELECT filename, content_type FROM files WHERE etag=$1", etag).Scan(&filename, &contentType)
		} else {
			err = db.QueryRow("SELECT filename, content_type FROM files WHERE etag=$1 AND user_id=$2", etag, userID).Scan(&filename, &contentType)
		}

		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "File not found", http.StatusNotFound)
			} else {
				l.Error(err)
				http.Error(w, "Server error", http.StatusInternalServerError)
			}
			return
		}

		// Get the file from MinIO
		archived := r.URL.Query().Get("archived")
		if archived == "true" {
			l.Info("Changing bucket name to backup")
			bucketName = "backup"
		} else {
			bucketName = "videos"
		}
		object, err := minioClient.GetObject(context.Background(), bucketName, filename, minio.GetObjectOptions{})
		if err != nil {
			l.Error(err)
			http.Error(w, "Error retrieving file", http.StatusInternalServerError)
			return
		}
		defer object.Close()

		// Set the content type and other headers, then write the file to the response
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		if _, err := io.Copy(w, object); err != nil {
			l.Error(err)
			http.Error(w, "Error writing file to response", http.StatusInternalServerError)
			return
		}
	}
}

func uploadFileHandler(config *ServerConfig, db *sql.DB, minioClient *minio.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer("uploader").Start(r.Context(), "HandleUpload")
		defer span.End()

		username := r.Context().Value("username").(string)
		userID := r.Context().Value("id").(int)

		span.SetAttributes(
			attribute.String("username", username),
			attribute.Int("user_id", userID),
		)

		l.Debugw("Handling video upload", "username", username)
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
		_, sha256Checksum, err := computeChecksum(ctx, file)
		if err != nil {
			l.Errorw("Could not compute checksum", zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error computing checksum", http.StatusInternalServerError)
			return
		}

		// Upload the file to MinIO
		file.Seek(0, io.SeekStart)
		l.Infow("Uploading file", zap.String("bucketname", config.MinioBucket),
			zap.String("filename", handler.Filename))
		info, err := minioClient.PutObject(ctx, config.MinioBucket, handler.Filename, file, fileSize, minio.PutObjectOptions{ContentType: contentType})
		if err != nil {
			l.Errorw("Could not upload file", zap.String("bucketname", config.MinioBucket),
				zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error uploading file", http.StatusInternalServerError)
			return
		}

		// Increment the Prometheus counter
		fileUploadCount.Inc()

		// Get the file URL and ETag
		fileURL := fmt.Sprintf("http://%s/%s/%s", config.MinioHost, config.MinioBucket, handler.Filename)
		etag := info.ETag

		// Store metadata in PostgreSQL
		err = storeFileMetadata(ctx, db, handler.Filename, fileSize, contentType, etag, fileURL, sha256Checksum, userID)
		if err != nil {
			l.Errorw("Could not store file metadata", zap.String("filename", handler.Filename), zap.Error(err))
			http.Error(w, "Error storing file metadata", http.StatusInternalServerError)
			return
		}

		l.Infow("Successfully uploaded file", zap.String("bucketname", config.MinioBucket),
			zap.String("filename", handler.Filename), zap.String("username", username))
		fmt.Fprintf(w, "Successfully uploaded %s\n", handler.Filename)
		publishMessage(ctx, config.MinioBucket, handler.Filename)
	}
}

func main() {
	l.Debug("Reading configuration")
	config := buildConfig()

	// Initialize OpenTelemetry tracer
	tp, err := initTracer(config)
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

	http.HandleFunc("/login", login(db))
	http.Handle("/upload", authenticate(uploadFileHandler(config, db, minioClient)))
	http.Handle("/files", authenticate(getUserFiles(db)))
	http.Handle("/download", authenticate(downloadFile(db, minioClient, config.MinioBucket)))

	// Expose the /metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	http.ListenAndServe(fmt.Sprintf(":%d", config.Port), otelhttp.NewHandler(http.DefaultServeMux, "Server"))
}
