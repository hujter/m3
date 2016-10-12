// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package series

import (
	"errors"
	"io"
	"time"

	"github.com/m3db/m3db/clock"
	"github.com/m3db/m3db/context"
	"github.com/m3db/m3db/encoding"
	"github.com/m3db/m3db/storage/block"
	"github.com/m3db/m3db/ts"
	xio "github.com/m3db/m3db/x/io"
	xerrors "github.com/m3db/m3x/errors"
	xtime "github.com/m3db/m3x/time"
)

var (
	errTooFuture = errors.New("datapoint is too far in the future")
	errTooPast   = errors.New("datapoint is too far in the past")
	timeZero     time.Time
)

const (
	// bucketsLen is three to contain the following buckets:
	// 1. Bucket before current window, can be drained or not-yet-drained
	// 2. Bucket currently taking writes
	// 3. Bucket for the future that can be taking writes that is head of
	// the current block if write is for the future within bounds
	bucketsLen = 3
)

type dbBuffer struct {
	opts         Options
	nowFn        clock.NowFn
	drainFn      databaseBufferDrainFn
	buckets      [bucketsLen]dbBufferBucket
	blockSize    time.Duration
	bufferPast   time.Duration
	bufferFuture time.Duration
}

type databaseBufferDrainFn func(start time.Time, encoder encoding.Encoder)

func newDatabaseBuffer(drainFn databaseBufferDrainFn, opts Options) databaseBuffer {
	b := &dbBuffer{
		opts:         opts,
		nowFn:        opts.ClockOptions().NowFn(),
		drainFn:      drainFn,
		blockSize:    opts.RetentionOptions().BlockSize(),
		bufferPast:   opts.RetentionOptions().BufferPast(),
		bufferFuture: opts.RetentionOptions().BufferFuture(),
	}
	b.Reset()
	return b
}

func (b *dbBuffer) Reset() {
	b.forEachBucketAsc(b.nowFn(), func(bucket *dbBufferBucket, start time.Time) {
		bucket.opts = b.opts
		bucket.resetTo(start)
	})
}

func (b *dbBuffer) Write(
	ctx context.Context,
	timestamp time.Time,
	value float64,
	unit xtime.Unit,
	annotation []byte,
) error {
	now := b.nowFn()
	futureLimit := now.Add(1 * b.bufferFuture)
	pastLimit := now.Add(-1 * b.bufferPast)
	if !futureLimit.After(timestamp) {
		return xerrors.NewInvalidParamsError(errTooFuture)
	}
	if !pastLimit.Before(timestamp) {
		return xerrors.NewInvalidParamsError(errTooPast)
	}

	bucketStart := timestamp.Truncate(b.blockSize)
	bucketIdx := (timestamp.UnixNano() / int64(b.blockSize)) % bucketsLen

	bucket := &b.buckets[bucketIdx]
	_, _, needsReset := b.bucketState(now, bucket, bucketStart)
	if needsReset {
		// Needs reset
		b.DrainAndReset(false)
	}

	return bucket.write(timestamp, value, unit, annotation)
}

func (b *dbBuffer) IsEmpty() bool {
	now := b.nowFn()
	canReadAny := false
	b.forEachBucketAsc(now, func(bucket *dbBufferBucket, current time.Time) {
		if !canReadAny {
			canReadAny, _, _ = b.bucketState(now, bucket, current)
		}
	})
	return !canReadAny
}

func (b *dbBuffer) NeedsDrain() bool {
	now := b.nowFn()
	needsDrainAny := false
	b.forEachBucketAsc(now, func(bucket *dbBufferBucket, current time.Time) {
		if !needsDrainAny {
			_, needsDrainAny, _ = b.bucketState(now, bucket, current)
		}
	})
	return needsDrainAny
}

func (b *dbBuffer) bucketState(
	now time.Time,
	bucket *dbBufferBucket,
	bucketStart time.Time,
) (bool, bool, bool) {
	notDrainedHasValues := !bucket.drained && !bucket.lastWriteAt.IsZero()

	shouldRead := notDrainedHasValues
	needsReset := !bucket.start.Equal(bucketStart)
	needsDrain := (notDrainedHasValues && needsReset) ||
		(notDrainedHasValues && bucketStart.Add(b.blockSize).Before(now.Add(-1*b.bufferPast)))

	return shouldRead, needsDrain, needsReset
}

func (b *dbBuffer) DrainAndReset(forced bool) {
	now := b.nowFn()
	b.forEachBucketAsc(now, func(bucket *dbBufferBucket, current time.Time) {
		_, needsDrain, needsReset := b.bucketState(now, bucket, current)
		if forced && !bucket.drained && !bucket.lastWriteAt.IsZero() {
			// If the bucket is not empty and hasn't been drained, force it to drain.
			needsDrain, needsReset = true, true
		}
		if needsDrain {
			bucket.sort()

			// After we sort there is always only a single encoder
			encoder := bucket.encoders[0].encoder
			encoder.Seal()
			b.drainFn(bucket.start, encoder)
			bucket.drained = true
		}

		if needsReset {
			// Reset bucket
			bucket.resetTo(current)
		}
	})
}

func (b *dbBuffer) forEachBucketAsc(now time.Time, fn func(*dbBufferBucket, time.Time)) {
	pastMostBucketStart := now.Truncate(b.blockSize).Add(-1 * b.blockSize)
	bucketNum := (pastMostBucketStart.UnixNano() / int64(b.blockSize)) % bucketsLen
	for i := int64(0); i < bucketsLen; i++ {
		idx := int((bucketNum + i) % bucketsLen)
		fn(&b.buckets[idx], pastMostBucketStart.Add(time.Duration(i)*b.blockSize))
	}
}

func (b *dbBuffer) ReadEncoded(ctx context.Context, start, end time.Time) [][]xio.SegmentReader {
	now := b.nowFn()

	// TODO(r): pool these results arrays
	var results [][]xio.SegmentReader
	b.forEachBucketAsc(now, func(bucket *dbBufferBucket, current time.Time) {
		shouldRead, _, _ := b.bucketState(now, bucket, current)
		if !shouldRead {
			return
		}
		if !start.Before(bucket.start.Add(b.blockSize)) {
			return
		}
		if !bucket.start.Before(end) {
			return
		}

		unmerged := make([]xio.SegmentReader, 0, len(bucket.encoders))
		b.readBucketStreams(ctx, bucket, now, current, func(stream xio.SegmentReader) {
			unmerged = append(unmerged, stream)
		})

		results = append(results, unmerged)
	})

	return results
}

func (b *dbBuffer) FetchBlocks(ctx context.Context, starts []time.Time) []block.FetchBlockResult {
	var res []block.FetchBlockResult

	now := b.nowFn()
	b.forEachBucketAsc(now, func(bucket *dbBufferBucket, current time.Time) {
		shouldRead, _, _ := b.bucketState(now, bucket, current)
		if !shouldRead {
			return
		}
		found := false
		// starts have only a few items, linear search should be okay time-wise to
		// avoid allocating a map here.
		for _, start := range starts {
			if start == bucket.start {
				found = true
				break
			}
		}
		if !found {
			return
		}
		readers := make([]xio.SegmentReader, 0, len(bucket.encoders))
		b.readBucketStreams(ctx, bucket, now, current, func(stream xio.SegmentReader) {
			readers = append(readers, stream)
		})
		res = append(res, block.NewFetchBlockResult(bucket.start, readers, nil))
	})

	return res
}

func (b *dbBuffer) FetchBlocksMetadata(
	ctx context.Context,
	includeSizes bool,
	includeChecksums bool,
) []block.FetchBlockMetadataResult {
	var res []block.FetchBlockMetadataResult

	now := b.nowFn()
	b.forEachBucketAsc(now, func(bucket *dbBufferBucket, current time.Time) {
		shouldRead, _, _ := b.bucketState(now, bucket, current)
		if !shouldRead {
			return
		}
		var size int64
		b.readBucketStreams(ctx, bucket, now, current, func(stream xio.SegmentReader) {
			segment := stream.Segment()
			size += int64(len(segment.Head) + len(segment.Tail))
		})
		// If we have no data in this bucket, return early without appending it to the result.
		if size == 0 {
			return
		}
		var pSize *int64
		if includeSizes {
			pSize = &size
		}
		// NB(xichen): intentionally returning nil checksums for buckets
		// because they haven't been flushed yet
		res = append(res, block.NewFetchBlockMetadataResult(bucket.start, pSize, nil, nil))
	})

	return res
}

type dbBufferBucketStreamFn func(stream xio.SegmentReader)

func (b *dbBuffer) readBucketStreams(
	ctx context.Context,
	bucket *dbBufferBucket,
	now, current time.Time,
	streamFn dbBufferBucketStreamFn,
) {
	for i := range bucket.encoders {
		stream := bucket.encoders[i].encoder.Stream()
		if stream == nil {
			// TODO(r): log an error and emit a metric, this is pretty bad as this
			// encoder should have values if "shouldRead" returned true
			continue
		}

		ctx.RegisterCloser(context.CloserFn(stream.Close))

		streamFn(stream)
	}
}

type dbBufferBucket struct {
	opts        Options
	start       time.Time
	encoders    []inOrderEncoder
	lastWriteAt time.Time
	outOfOrder  bool
	drained     bool
}

type inOrderEncoder struct {
	lastWriteAt time.Time
	encoder     encoding.Encoder
}

func (b *dbBufferBucket) resetTo(start time.Time) {
	bopts := b.opts.DatabaseBlockOptions()
	encoder := bopts.EncoderPool().Get()
	encoder.Reset(start, bopts.DatabaseBlockAllocSize())
	first := inOrderEncoder{
		lastWriteAt: timeZero,
		encoder:     encoder,
	}

	b.start = start
	b.encoders = append(b.encoders[:0], first)
	b.lastWriteAt = timeZero
	b.outOfOrder = false
	b.drained = false
}

func (b *dbBufferBucket) write(timestamp time.Time, value float64, unit xtime.Unit, annotation []byte) error {
	if !b.outOfOrder && timestamp.Before(b.lastWriteAt) {
		// Can never revert from out of order until reset or sorted
		b.outOfOrder = true
	}
	if timestamp.After(b.lastWriteAt) {
		b.lastWriteAt = timestamp
	}

	var target *inOrderEncoder
	for i := range b.encoders {
		if timestamp == b.encoders[i].lastWriteAt {
			// NB(xichen): we discard datapoints with the same timestamps as the ones we've
			// already encoded.
			return nil
		}
		if timestamp.After(b.encoders[i].lastWriteAt) {
			target = &b.encoders[i]
			break
		}
	}
	if target == nil {
		bopts := b.opts.DatabaseBlockOptions()
		blockSize := b.opts.RetentionOptions().BlockSize()
		encoder := bopts.EncoderPool().Get()
		encoder.Reset(timestamp.Truncate(blockSize), bopts.DatabaseBlockAllocSize())
		next := inOrderEncoder{encoder: encoder}
		b.encoders = append(b.encoders, next)
		target = &b.encoders[len(b.encoders)-1]
	}

	datapoint := ts.Datapoint{
		Timestamp: timestamp,
		Value:     value,
	}
	if err := target.encoder.Encode(datapoint, unit, annotation); err != nil {
		return err
	}
	target.lastWriteAt = timestamp
	return nil
}

func (b *dbBufferBucket) sort() {
	if !b.outOfOrder {
		// Already sorted
		return
	}

	bopts := b.opts.DatabaseBlockOptions()
	encoder := bopts.EncoderPool().Get()
	encoder.Reset(b.start, bopts.DatabaseBlockAllocSize())

	readers := make([]io.Reader, len(b.encoders))
	for i := range b.encoders {
		readers[i] = b.encoders[i].encoder.Stream()
	}

	iter := b.opts.MultiReaderIteratorPool().Get()
	iter.Reset(readers)
	for iter.Next() {
		dp, unit, annotation := iter.Current()
		b.lastWriteAt = dp.Timestamp
		encoder.Encode(dp, unit, annotation)
	}
	iter.Close()

	for i := range b.encoders {
		b.encoders[i].encoder.Close()
	}

	b.encoders = append(b.encoders[:0], inOrderEncoder{
		lastWriteAt: b.lastWriteAt,
		encoder:     encoder,
	})
	b.outOfOrder = false
}