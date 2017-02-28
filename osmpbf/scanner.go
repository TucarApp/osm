package osmpbf

import (
	"context"
	"io"

	osm "github.com/paulmach/go.osm"
)

var _ osm.Scanner = &Scanner{}

// Scanner provides a convenient interface reading a stream of osm data
// from a file or url. Successive calls to the Scan method will step through the data.
//
// Scanning stops unrecoverably at EOF, the first I/O error, the first xml error or
// the context being cancelled. When a scan stops, the reader may have advanced
// arbitrarily far past the last token.
//
// The Scanner API is based on bufio.Scanner
// https://golang.org/pkg/bufio/#Scanner
type Scanner struct {
	ctx    context.Context
	closed bool

	decoder *decoder
	started bool
	procs   int
	next    osm.Element
	err     error
}

// New returns a new Scanner to read from r.
// procs indicates amount of paralellism, when reading blocks
// which will off load the unzipping/decoding to multiple cpus.
func New(ctx context.Context, r io.Reader, procs int) *Scanner {
	if ctx == nil {
		ctx = context.Background()
	}

	s := &Scanner{
		ctx:   ctx,
		procs: procs,
	}
	s.decoder = newDecoder(ctx, r)
	return s
}

// FullyScannedBytes returns the number of bytes that have been read
// and fully scanned. OSM protobuf files contain data blocks with
// 8000 nodes each. The returned value contains the bytes for the blocks
// that have been fully scanned.
//
// A user can use this number of seek forward in a file
// and begin reading mid-data. Note that while elements are usually sorted
// by Type, ID, Version in OMS protobuf files, versions of given element may
// span blocks.
func (s *Scanner) FullyScannedBytes() int64 {
	return s.decoder.cOffset
}

// Close cleans up all the reading goroutines, it does not
// close the underlying reader.
func (s *Scanner) Close() error {
	s.closed = true
	return s.decoder.Close()
}

// Scan advances the Scanner to the next element, which will then be available
// through the Element method. It returns false when the scan stops, either
// by reaching the end of the input, an io error, an xml error or the context
// being cancelled. After Scan returns false, the Err method will return any
// error that occurred during scanning, except that if it was io.EOF, Err will
// return nil.
func (s *Scanner) Scan() bool {
	if !s.started {
		s.started = true
		s.err = s.decoder.Start(s.procs)
	}

	if s.err != nil || s.closed || s.ctx.Err() != nil {
		return false
	}

	s.next, s.err = s.decoder.Next()
	if s.err != nil {
		return false
	}

	return true
}

// Element returns the most recent token generated by a call to Scan
// as a new osm Element.
func (s *Scanner) Element() osm.Element {
	return s.next
}

// Err returns the first non-EOF error that was encountered by the Scanner.
func (s *Scanner) Err() error {
	if s.err == io.EOF {
		return nil
	}

	if s.err != nil {
		return s.err
	}

	if s.closed {
		return osm.ErrScannerClosed
	}

	return s.ctx.Err()
}
