.PHONY: minio
minio:
	docker run -p 9000:9000 -p 9001:9001 -d --name minio1 \
  	-e "MINIO_ROOT_USER=minioadmin" \
  	-e "MINIO_ROOT_PASSWORD=minioadmin" \
  	minio/minio server /data --console-address ":9001"

.PHONY: run
run:
	set -a && . $(service)/.env && go run $(service)/cmd/main.go

.PHONY: build-image
build-image:
	docker build -t uploader .

.PHONY: run-image
run-image:
	docker run --name uploader -p 8080:8080 --env-file .env uploader

.PHONY: db
db:
	PGPASSWORD=postgres psql -h localhost -U postgres -d videos

.PHONY: migrate
migrate:
	GOOSE_MIGRATION_DIR=db/migrations goose postgres "postgresql://localhost:5432/videos?user=postgres&password=postgres" up