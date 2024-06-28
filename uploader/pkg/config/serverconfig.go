package config 

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