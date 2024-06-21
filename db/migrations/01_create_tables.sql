-- +goose Up

-- videos
CREATE TABLE "files"(
    id                  SERIAL PRIMARY KEY,
    filename            VARCHAR(255) NOT NULL,
    filesize            BIGINT NOT NULL,
    upload_timestamp    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    content_type        VARCHAR(255),
    etag                VARCHAR(255),
    file_url            VARCHAR(255) NOT NULL,
     user_id            INTEGER,
    checksum            VARCHAR(255)
);


-- +goose Down
DROP TABLE "courses_students" CASCADE;