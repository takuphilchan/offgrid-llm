package rag

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
)

func (p *DocumentParser) openArchive(content []byte) (*zip.Reader, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, err
	}
	if len(reader.File) > 10000 {
		return nil, &ParseError{Code: "document_limit", Message: "The document contains too many archive entries."}
	}
	const archiveLimit = 64 << 20
	var total uint64
	for _, file := range reader.File {
		if file.UncompressedSize64 > archiveLimit-total {
			return nil, &ParseError{Code: "document_limit", Message: "Expanded document data exceeds the 64 MiB limit."}
		}
		total += file.UncompressedSize64
	}
	return reader, nil
}

// ParseError is safe to return to document clients. It describes a document or
// capability problem rather than leaking process stderr or temporary paths.
type ParseError struct {
	Code    string
	Message string
}

func (e *ParseError) Error() string { return e.Message }

func classifyParseError(err error) error {
	var problem *ParseError
	if errors.As(err, &problem) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &ParseError{Code: "document_invalid", Message: "The document could not be read. It may be damaged, empty, encrypted, or use an unsupported encoding."}
}

func isParseLimit(err error) bool {
	var problem *ParseError
	return errors.As(err, &problem) && problem.Code == "document_limit"
}

func readParserBytes(r io.Reader, limit int64) ([]byte, error) {
	if limit < 0 {
		return nil, &ParseError{Code: "document_limit", Message: "Document extraction limit exceeded."}
	}
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, &ParseError{Code: "document_limit", Message: "Document extraction limit exceeded."}
	}
	return data, nil
}

type parserOutput struct {
	buffer   bytes.Buffer
	limit    int64
	exceeded bool
}

func (b *parserOutput) Len() int       { return b.buffer.Len() }
func (b *parserOutput) String() string { return b.buffer.String() }

func (b *parserOutput) Write(p []byte) (int, error) {
	if b.exceeded || int64(b.Len())+int64(len(p)) > b.limit {
		b.exceeded = true
		return 0, &ParseError{Code: "document_limit", Message: "Document extraction limit exceeded."}
	}
	return b.buffer.Write(p)
}
