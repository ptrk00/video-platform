-- +goose Up

-- courses_students
CREATE TABLE "courses_students"(
    "id"            BIGINT NOT NULL,
    "course_id"     BIGINT NOT NULL,
    "student_id"    BIGINT NOT NULL,
    "enrolled_at"   TIMESTAMP(0) WITHOUT TIME ZONE NOT NULL,
    "current_video" BIGINT NOT NULL DEFAULT '0'
);
ALTER TABLE
    "courses_students" ADD PRIMARY KEY("id");
COMMENT
ON COLUMN
    "courses_students"."current_video" IS 'index of current video';

-- users
CREATE TABLE "users"(
    "id"            BIGINT NOT NULL,
    "created_at"    TIMESTAMP(0) WITHOUT TIME ZONE NOT NULL,
    "verified"      BOOLEAN NOT NULL
);
ALTER TABLE
    "users" ADD PRIMARY KEY("id");

-- videos
CREATE TABLE "videos"(
    "id"                BIGINT NOT NULL,
    "s3_path"           VARCHAR(255) NOT NULL,
    "name"              VARCHAR(255) NOT NULL,
    "course_id"         BIGINT NOT NULL,
    "byte_size"         BIGINT NOT NULL,
    "duration_seconds"  BIGINT NOT NULL,
    "processesd"        BOOLEAN NOT NULL DEFAULT '0',
    "ts_files_count"    BIGINT NULL,
    "sha256"            VARCHAR(255) NOT NULL,
    "created_at"        TIMESTAMP(0) WITHOUT TIME ZONE NOT NULL,
    "deleted_at"        TIMESTAMP(0) WITHOUT TIME ZONE NULL
);
CREATE INDEX "videos_course_id_index" ON
    "videos"("course_id");
CREATE INDEX "videos_name_index" ON
    "videos"("name");
ALTER TABLE
    "videos" ADD PRIMARY KEY("id");
ALTER TABLE
    "videos" ADD CONSTRAINT "videos_s3_path_unique" UNIQUE("s3_path");
COMMENT
ON COLUMN
    "videos"."processesd" IS 'has video been processed?';

-- courses
CREATE TABLE "courses"(
    "id"           BIGINT NOT NULL,
    "name"         VARCHAR(255) NOT NULL,
    "owner_id"     BIGINT NOT NULL,
    "videos_count" BIGINT NOT NULL,
    "prize_usd"    BIGINT NOT NULL
);
ALTER TABLE
    "courses" ADD PRIMARY KEY("id");

-- ratings
CREATE TABLE "ratings"(
    "id"        BIGINT NOT NULL,
    "star_num"  INTEGER NOT NULL,
    "video_id"  BIGINT NOT NULL,
    "comment"   VARCHAR(255) NULL,
    "author_id" BIGINT NOT NULL
);
CREATE INDEX "ratings_video_id_index" ON
    "ratings"("video_id");
ALTER TABLE
    "ratings" ADD PRIMARY KEY("id");

-- payments
CREATE TABLE "payments"(
    "id"         BIGINT NOT NULL,
    "course_id"  BIGINT NOT NULL,
    "student_id" BIGINT NOT NULL,
    "created_at" TIMESTAMP(0) WITHOUT TIME ZONE NOT NULL,
    "stripe_id"  BIGINT NOT NULL,
    "successful" BOOLEAN NOT NULL
);
ALTER TABLE
    "payments" ADD PRIMARY KEY("id");
ALTER TABLE
    "ratings" ADD CONSTRAINT "ratings_author_id_foreign" FOREIGN KEY("author_id") REFERENCES "users"("id");
ALTER TABLE
    "courses_students" ADD CONSTRAINT "courses_students_course_id_foreign" FOREIGN KEY("course_id") REFERENCES "courses"("id");
ALTER TABLE
    "videos" ADD CONSTRAINT "videos_course_id_foreign" FOREIGN KEY("course_id") REFERENCES "courses"("id");
ALTER TABLE
    "payments" ADD CONSTRAINT "payments_student_id_foreign" FOREIGN KEY("student_id") REFERENCES "users"("id");
ALTER TABLE
    "courses_students" ADD CONSTRAINT "courses_students_student_id_foreign" FOREIGN KEY("student_id") REFERENCES "users"("id");
ALTER TABLE
    "courses" ADD CONSTRAINT "courses_owner_id_foreign" FOREIGN KEY("owner_id") REFERENCES "users"("id");
ALTER TABLE
    "ratings" ADD CONSTRAINT "ratings_video_id_foreign" FOREIGN KEY("video_id") REFERENCES "videos"("id");
ALTER TABLE
    "payments" ADD CONSTRAINT "payments_course_id_foreign" FOREIGN KEY("course_id") REFERENCES "courses"("id");

-- +goose Down
DROP TABLE "courses_students" CASCADE;
DROP TABLE "users" CASCADE;
DROP TABLE "videos" CASCADE;
DROP TABLE "courses" CASCADE;
DROP TABLE "ratings" CASCADE;
DROP TABLE "payments" CASCADE;