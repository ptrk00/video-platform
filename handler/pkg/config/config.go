package config

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
