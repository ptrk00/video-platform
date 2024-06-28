package monitoring

import "github.com/prometheus/client_golang/prometheus"

var FileUploadCount = prometheus.NewCounter(
	prometheus.CounterOpts{
		Name: "file_upload_count",
		Help: "Number of files uploaded",
	},
)

func init() {
	prometheus.MustRegister(FileUploadCount)
}
