// Copyright (c) 2018 Uber Technologies, Inc.
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

package uninitialized

import (
	"testing"
	"time"

	"github.com/m3db/m3/src/dbnode/storage/bootstrap"
	"github.com/m3db/m3/src/dbnode/storage/bootstrap/result"
	"github.com/m3db/m3/src/dbnode/storage/namespace"
	"github.com/m3db/m3/src/dbnode/topology"
	topotestutils "github.com/m3db/m3/src/dbnode/topology/testutil"
	"github.com/m3db/m3cluster/shard"
	"github.com/m3db/m3x/ident"
	"github.com/m3db/m3x/instrument"
	xtime "github.com/m3db/m3x/time"
	"github.com/stretchr/testify/require"
)

var (
	testNamespaceID    = ident.StringID("testnamespace")
	testDefaultRunOpts = bootstrap.NewRunOptions()
	notSelfID1         = "not-self-1"
	notSelfID2         = "not-self-2"
	notSelfID3         = "not-self-3"
)

func TestUnitializedSourceAvailableDataAndAvailableIndex(t *testing.T) {
	var (
		blockSize                  = 2 * time.Hour
		shards                     = []uint32{0, 1, 2, 3}
		blockStart                 = time.Now().Truncate(blockSize)
		shardTimeRangesToBootstrap = result.ShardTimeRanges{}
		bootstrapRanges            = xtime.Ranges{}.AddRange(xtime.Range{
			Start: blockStart,
			End:   blockStart.Add(blockSize),
		})
	)
	nsMetadata, err := namespace.NewMetadata(testNamespaceID, namespace.NewOptions())
	require.NoError(t, err)

	for _, shard := range shards {
		shardTimeRangesToBootstrap[shard] = bootstrapRanges
	}

	testCases := []struct {
		title                             string
		majorityReplicas                  int
		hosts                             topotestutils.SourceAvailableHosts
		bootstrapReadConsistency          topology.ReadConsistencyLevel
		shardsTimeRangesToBootstrap       result.ShardTimeRanges
		expectedAvailableShardsTimeRanges result.ShardTimeRanges
	}{
		// Snould return that it can bootstrap everything because
		// it's a new namespace.
		{
			title:            "Single node - Shard initializing",
			majorityReplicas: 1,
			hosts: []topotestutils.SourceAvailableHost{
				topotestutils.SourceAvailableHost{
					Name:        topotestutils.SelfID,
					Shards:      shards,
					ShardStates: shard.Initializing,
				},
			},
			shardsTimeRangesToBootstrap:       shardTimeRangesToBootstrap,
			expectedAvailableShardsTimeRanges: shardTimeRangesToBootstrap,
		},
		// Snould return that it can't bootstrap anything because we don't
		// know how to handle unknown shard states.
		{
			title:            "Single node - Shard unknown",
			majorityReplicas: 1,
			hosts: []topotestutils.SourceAvailableHost{
				topotestutils.SourceAvailableHost{
					Name:        topotestutils.SelfID,
					Shards:      shards,
					ShardStates: shard.Unknown,
				},
			},
			shardsTimeRangesToBootstrap:       shardTimeRangesToBootstrap,
			expectedAvailableShardsTimeRanges: result.ShardTimeRanges{},
		},
		// Snould return that it can't bootstrap anything because it's not
		// a new namespace.
		{
			title:            "Single node - Shard leaving",
			majorityReplicas: 1,
			hosts: []topotestutils.SourceAvailableHost{
				topotestutils.SourceAvailableHost{
					Name:        topotestutils.SelfID,
					Shards:      shards,
					ShardStates: shard.Leaving,
				},
			},
			shardsTimeRangesToBootstrap:       shardTimeRangesToBootstrap,
			expectedAvailableShardsTimeRanges: result.ShardTimeRanges{},
		},
		// Snould return that it can't bootstrap anything because it's not
		// a new namespace.
		{
			title:            "Single node - Shard available",
			majorityReplicas: 1,
			hosts: []topotestutils.SourceAvailableHost{
				topotestutils.SourceAvailableHost{
					Name:        topotestutils.SelfID,
					Shards:      shards,
					ShardStates: shard.Available,
				},
			},
			shardsTimeRangesToBootstrap:       shardTimeRangesToBootstrap,
			expectedAvailableShardsTimeRanges: result.ShardTimeRanges{},
		},
		// Snould return that it can bootstrap everything because
		// it's a new namespace.
		{
			title:            "Multi node - Brand new namespace (all nodes initializing)",
			majorityReplicas: 2,
			hosts: []topotestutils.SourceAvailableHost{
				topotestutils.SourceAvailableHost{
					Name:        topotestutils.SelfID,
					Shards:      shards,
					ShardStates: shard.Initializing,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID1,
					Shards:      shards,
					ShardStates: shard.Initializing,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID2,
					Shards:      shards,
					ShardStates: shard.Initializing,
				},
			},
			shardsTimeRangesToBootstrap:       shardTimeRangesToBootstrap,
			expectedAvailableShardsTimeRanges: shardTimeRangesToBootstrap,
		},
		// Snould return that it can bootstrap everything because
		// it's a new namespace (one of the nodes hasn't completed
		// initializing yet.)
		{
			title:            "Multi node - Recently created namespace (one node still initializing)",
			majorityReplicas: 2,
			hosts: []topotestutils.SourceAvailableHost{
				topotestutils.SourceAvailableHost{
					Name:        topotestutils.SelfID,
					Shards:      shards,
					ShardStates: shard.Initializing,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID1,
					Shards:      shards,
					ShardStates: shard.Available,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID2,
					Shards:      shards,
					ShardStates: shard.Available,
				},
			},
			shardsTimeRangesToBootstrap:       shardTimeRangesToBootstrap,
			expectedAvailableShardsTimeRanges: shardTimeRangesToBootstrap,
		},
		// Snould return that it can't bootstrap anything because it's not
		// a new namespace.
		{
			title:            "Multi node - Initialized namespace (no nodes initializing)",
			majorityReplicas: 2,
			hosts: []topotestutils.SourceAvailableHost{
				topotestutils.SourceAvailableHost{
					Name:        topotestutils.SelfID,
					Shards:      shards,
					ShardStates: shard.Available,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID1,
					Shards:      shards,
					ShardStates: shard.Available,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID2,
					Shards:      shards,
					ShardStates: shard.Available,
				},
			},
			shardsTimeRangesToBootstrap:       shardTimeRangesToBootstrap,
			expectedAvailableShardsTimeRanges: result.ShardTimeRanges{},
		},
		// Snould return that it can't bootstrap anything because it's not
		// a new namespace, we're just doing a node replace.
		{
			title:            "Multi node - Node replace (one node leaving, one initializing)",
			majorityReplicas: 2,
			hosts: []topotestutils.SourceAvailableHost{
				topotestutils.SourceAvailableHost{
					Name:        topotestutils.SelfID,
					Shards:      shards,
					ShardStates: shard.Available,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID1,
					Shards:      shards,
					ShardStates: shard.Leaving,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID2,
					Shards:      shards,
					ShardStates: shard.Available,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID3,
					Shards:      shards,
					ShardStates: shard.Initializing,
				},
			},
			shardsTimeRangesToBootstrap:       shardTimeRangesToBootstrap,
			expectedAvailableShardsTimeRanges: result.ShardTimeRanges{},
		},
		// Snould return that it can't bootstrap anything because we don't
		// know how to interpret the unknown host.
		{
			title:            "Multi node - One node unknown",
			majorityReplicas: 2,
			hosts: []topotestutils.SourceAvailableHost{
				topotestutils.SourceAvailableHost{
					Name:        topotestutils.SelfID,
					Shards:      shards,
					ShardStates: shard.Available,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID1,
					Shards:      shards,
					ShardStates: shard.Available,
				},
				topotestutils.SourceAvailableHost{
					Name:        notSelfID2,
					Shards:      shards,
					ShardStates: shard.Unknown,
				},
			},
			shardsTimeRangesToBootstrap:       shardTimeRangesToBootstrap,
			expectedAvailableShardsTimeRanges: result.ShardTimeRanges{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {

			var (
				srcOpts  = NewOptions().SetInstrumentOptions(instrument.NewOptions())
				src      = newUninitializedSource(srcOpts)
				runOpts  = testDefaultRunOpts.SetInitialTopologyState(tc.hosts.TopologyState(tc.majorityReplicas))
				dataRes  = src.AvailableData(nsMetadata, tc.shardsTimeRangesToBootstrap, runOpts)
				indexRes = src.AvailableIndex(nsMetadata, tc.shardsTimeRangesToBootstrap, runOpts)
			)

			require.Equal(t, tc.expectedAvailableShardsTimeRanges, dataRes)
			require.Equal(t, tc.expectedAvailableShardsTimeRanges, indexRes)
		})
	}
}
