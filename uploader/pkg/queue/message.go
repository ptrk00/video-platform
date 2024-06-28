package queue

type Message struct {
	Bucket   string `json:"bucket"`
	Filename string `json:"filename"`
}
