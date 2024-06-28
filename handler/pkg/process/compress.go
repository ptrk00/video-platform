package process

import (
	"bytes"
	"io"

	"github.com/ulikunitz/xz"
)

func CompressData(reader io.Reader) (io.Reader, error) {
	var buf bytes.Buffer
	writer, err := xz.NewWriter(&buf)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(writer, reader); err != nil {
		writer.Close()
		return nil, err
	}
	writer.Close()

	return &buf, nil
}
