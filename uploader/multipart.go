package uploader

import (
	"io"

	"github.com/TrueCloudLab/frostfs-http-gw/uploader/multipart"
	"go.uber.org/zap"
)

// MultipartFile provides standard ReadCloser interface and also allows one to
// get file name, it's used for multipart uploads.
type MultipartFile interface {
	io.ReadCloser
	FileName() string
}

func fetchMultipartFile(l *zap.Logger, r io.Reader, boundary string) (MultipartFile, error) {
	// To have a custom buffer (3mb) the custom multipart reader is used.
	// https://github.com/nspcc-dev/neofs-http-gw/issues/148
	reader := multipart.NewReader(r, boundary)

	for {
		part, err := reader.NextPart()
		if err != nil {
			return nil, err
		}

		name := part.FormName()
		if name == "" {
			l.Debug("ignore part, empty form name")
			continue
		}

		filename := part.FileName()

		// ignore multipart/form-data values
		if filename == "" {
			l.Debug("ignore part, empty filename", zap.String("form", name))

			continue
		}

		return part, nil
	}
}
