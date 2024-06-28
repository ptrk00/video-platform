package process

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
)

func ComputeChecksum(ctx context.Context, reader io.Reader) (string, string, error) {
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
