package storage

import (
	"context"
	"database/sql"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func StoreFileMetadata(ctx context.Context, db *sql.DB, filename string, filesize int64, contentType, etag, fileURL, checksum string, userID int) error {
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
