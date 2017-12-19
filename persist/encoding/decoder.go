// Copyright (c) 2016 Uber Technologies, Inc
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE

package encoding

import "github.com/m3db/m3db/persist/schema"

// FilesetDecoder decodes fileset related structures
type FilesetDecoder interface {
	// DecodeIndexInfo decodes the index info
	DecodeIndexInfo() (schema.IndexInfo, error)

	// DecodeIndexEntry decodes index entry
	DecodeIndexEntry() (schema.IndexEntry, error)

	// DecodeIndexSummary decodes index summary and returns an IndexSummaryIDBytesMetadata
	// which can be used in conjunction with the underlying buffer to quickly retrieve
	// a summary entry's series ID and index file offset
	DecodeIndexSummary() (schema.IndexSummary, IndexSummaryToken, error)
}

// CommitLogDecoder decodes commit log related structures
type CommitLogDecoder interface {
	// DecodeLogInfo decodes commit log info
	DecodeLogInfo() (schema.LogInfo, error)

	// DecodeLogMetadata decodes commit log metadata
	DecodeLogMetadata() (schema.LogMetadata, error)

	// DecodeLogEntry decodes commit log entry
	DecodeLogEntry() (schema.LogEntry, error)
}

// Decoder decodes persisted data
type Decoder interface {
	FilesetDecoder
	CommitLogDecoder

	// Reset resets the data stream to decode from
	Reset(stream DecoderStream)
}
