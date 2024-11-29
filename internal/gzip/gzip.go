// Package gzip provides middleware and utilities for handling Gzip compression and decompression
// in HTTP requests and responses. It allows automatic compression of HTTP responses.
package gzip

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
)

// CompressWriter is a custom ResponseWriter that compresses HTTP responses using Gzip.
// It wraps the standard http.ResponseWriter and a gzip.Writer to handle compression.
type CompressWriter struct {
	w  http.ResponseWriter // w is the underlying HTTP response writer.
	zw *gzip.Writer        // zw is the Gzip writer used to compress the response.
}

// NewCompressWriter initializes and returns a new CompressWriter.
// It creates a new gzip.Writer that wraps the provided
func NewCompressWriter(w http.ResponseWriter) *CompressWriter {
	return &CompressWriter{
		w:  w,
		zw: gzip.NewWriter(w),
	}
}

// Header returns the header map that will be sent by WriteHeader.
// It delegates the call to the underlying http.ResponseWriter.
func (c *CompressWriter) Header() http.Header {
	return c.w.Header()
}

// Write writes the data to the connection as part of an HTTP response.
// It compresses the data using the gzip.Writer before sending it.
func (c *CompressWriter) Write(p []byte) (int, error) {
	return c.zw.Write(p)
}

// WriteHeader sends an HTTP response header with the provided status code.
// If the status code is less than 300, it sets the "Content-Encoding" header to "gzip".
func (c *CompressWriter) WriteHeader(statusCode int) {
	if statusCode < 300 {
		c.w.Header().Set("Content-Encoding", "gzip")
	}
	c.w.WriteHeader(statusCode)
}

// Close closes the gzip.Writer to ensure all compressed data is flushed to the underlying writer.
func (c *CompressWriter) Close() error {
	return c.zw.Close()
}

// CompressReader is a custom io.ReadCloser that decompresses Gzip-encoded HTTP request bodies.
// It wraps an existing io.ReadCloser and a gzip.Reader to handle decompression.
type CompressReader struct {
	r  io.ReadCloser // r is the underlying reader for the HTTP request body.
	zr *gzip.Reader  // zr is the Gzip reader used to decompress the request body.
}

// NewCompressReader initializes and returns a new CompressReader.
// It creates a new gzip.Reader that wraps the provided io.ReadCloser.
// An error is returned if the provided reader does not contain valid Gzip data.
func NewCompressReader(r io.ReadCloser) (*CompressReader, error) {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return nil, err
	}

	return &CompressReader{
		r:  r,
		zr: zr,
	}, nil
}

// Read reads decompressed data from the underlying gzip.Reader.
// It implements the io.Reader interface.
func (c CompressReader) Read(p []byte) (n int, err error) {
	return c.zr.Read(p)
}

// Close closes both the underlying io.ReadCloser and the gzip.Reader.
func (c *CompressReader) Close() error {
	if err := c.r.Close(); err != nil {
		return err
	}
	return c.zr.Close()
}

// Middleware is an HTTP middleware that handles Gzip compression and decompression.
// It compresses HTTP responses if the client supports Gzip (indicated by the "Accept-Encoding" header)
// and decompresses HTTP request bodies if they are Gzipped (indicated by the "Content-Encoding" header).
func Middleware(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Preserve the original ResponseWriter.
		ow := w

		// Check if the client accepts Gzip encoding
		acceptEncoding := r.Header.Get("Accept-Encoding")
		supportsGzip := strings.Contains(acceptEncoding, "gzip")
		if supportsGzip {
			// Wrap the ResponseWriter with CompressWriter to handle Gzip compression.
			cw := NewCompressWriter(w)
			ow = cw
			defer cw.Close()
		}

		// Check if the request body is Gzipped
		contentEncoding := r.Header.Get("Content-Encoding")
		sendsGzip := strings.Contains(contentEncoding, "gzip")
		if sendsGzip {
			// Wrap the request body with CompressReader to handle Gzip decompression.
			cr, err := NewCompressReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			r.Body = cr
			defer cr.Close()
		}

		// Serve the HTTP request using the wrapped ResponseWriter and Request.
		h.ServeHTTP(ow, r)
	}
}
