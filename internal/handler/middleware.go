package handler

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

func GzipMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")

		contentEncoding := r.Header.Get("Content-Encoding")
		requestIsGzip := strings.EqualFold(contentEncoding, "gzip")

		if requestIsGzip {
			gzReader, err := gzip.NewReader(r.Body)
			if err != nil {
				respondWithError(w, "Ошибка распаковки запроса", http.StatusBadRequest)
				return
			}
			defer gzReader.Close()

			body, err := io.ReadAll(gzReader)
			if err != nil {
				respondWithError(w, "Ошибка чтения распакованного тела", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(strings.NewReader(string(body)))
		}

		if supportsGzip {
			gzWriter := NewGzipResponseWriter(w)
			defer gzWriter.Close()

			next.ServeHTTP(gzWriter, r)
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

type GzipResponseWriter struct {
	http.ResponseWriter
	buffer     *bytes.Buffer
	gzipWriter *gzip.Writer
	headerSent bool
	shouldGzip bool
}

func NewGzipResponseWriter(w http.ResponseWriter) *GzipResponseWriter {
	return &GzipResponseWriter{
		ResponseWriter: w,
		buffer:         new(bytes.Buffer),
		gzipWriter:     gzip.NewWriter(w),
		headerSent:     false,
		shouldGzip:     false,
	}
}

func (g *GzipResponseWriter) WriteHeader(statusCode int) {
	contentType := g.Header().Get("Content-Type")

	g.shouldGzip = strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "text/html")

	if g.shouldGzip {
		g.Header().Set("Content-Encoding", "gzip")
	}

	g.headerSent = true
	g.ResponseWriter.WriteHeader(statusCode)
}

func (g *GzipResponseWriter) Write(p []byte) (int, error) {
	if !g.headerSent {
		g.WriteHeader(http.StatusOK)
	}

	if g.shouldGzip {
		return g.gzipWriter.Write(p)
	}

	return g.ResponseWriter.Write(p)
}

func (g *GzipResponseWriter) Close() error {
	if g.shouldGzip {
		return g.gzipWriter.Close()
	}
	return nil
}
