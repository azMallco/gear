package gear

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// Compressible interface is use to enable compress response context.
type Compressible interface {
	// Compressible checks the response Content-Type and Content-Length to
	// determine whether to compress.
	// `length == 0` means response body maybe stream, or will be writed later.
	Compressible(contentType string, contentLength int) bool
}

// DefaultCompress is defalut Compress implemented. Use it to enable compress:
//
//  app.Set("AppCompress", &gear.DefaultCompress{})
//
type DefaultCompress struct{}

// Compressible implemented Compress interface.
// Recommend https://github.com/teambition/compressible-go.
//
//  import "github.com/teambition/compressible-go"
//
//  app := gear.New()
//  app.Set("AppCompress", compressible.WithThreshold(1024))
//
//  // Add a static middleware
//  app.Use(static.New(static.Options{
//  	Root:   "./",
//  	Prefix: "/",
//  }))
//  app.Error(app.Listen(":3000")) // http://127.0.0.1:3000/
//
func (d *DefaultCompress) Compressible(contentType string, contentLength int) bool {
	if contentLength > 0 && contentLength <= 1024 {
		return false
	}
	return contentType != ""
}

// http.ResponseWriter wrapper
type compressWriter struct {
	compress   Compressible
	encoding   string
	writer     io.WriteCloser
	rw         http.ResponseWriter
	bodyLength *int
}

func newCompress(res *Response, c Compressible, acceptEncoding string) *compressWriter {
	encodings := strings.Split(acceptEncoding, ",")
	encoding := strings.TrimSpace(encodings[0])
	switch encoding {
	case "gzip", "deflate":
		return &compressWriter{
			compress:   c,
			rw:         res.rw,
			encoding:   encoding,
			bodyLength: &res.bodyLength,
		}
	default:
		return nil
	}
}

func (cw *compressWriter) WriteHeader(code int) {
	defer cw.rw.WriteHeader(code)

	switch code {
	case http.StatusNoContent, http.StatusResetContent, http.StatusNotModified:
		return
	}

	header := cw.Header()
	if cw.compress.Compressible(header.Get(HeaderContentType), *cw.bodyLength) {
		var w io.WriteCloser

		switch cw.encoding {
		case "gzip":
			w, _ = gzip.NewWriterLevel(cw.rw, gzip.DefaultCompression)
		case "deflate":
			w, _ = flate.NewWriter(cw.rw, flate.DefaultCompression)
		}

		if w != nil {
			cw.writer = w
			header.Set(HeaderVary, HeaderAcceptEncoding)
			header.Set(HeaderContentEncoding, cw.encoding)
			header.Del(HeaderContentLength)
		}
	}
}

func (cw *compressWriter) Header() http.Header {
	return cw.rw.Header()
}

func (cw *compressWriter) Write(b []byte) (int, error) {
	if cw.writer != nil {
		return cw.writer.Write(b)
	}
	return cw.rw.Write(b)
}

func (cw *compressWriter) Close() error {
	if cw.writer != nil {
		return cw.writer.Close()
	}
	return nil
}
