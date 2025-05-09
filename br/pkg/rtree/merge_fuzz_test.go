// Copyright 2024 PingCAP, Inc. Licensed under Apache-2.0.

package rtree_test

import (
	"testing"

	backup "github.com/pingcap/kvproto/pkg/brpb"
	"github.com/pingcap/tidb/br/pkg/rtree"
	"github.com/pingcap/tidb/pkg/tablecodec"
)

func FuzzMerge(f *testing.F) {
	baseKeyA := tablecodec.EncodeIndexSeekKey(42, 1, nil)
	baseKeyB := tablecodec.EncodeIndexSeekKey(42, 1, nil)
	f.Add([]byte(baseKeyA), []byte(baseKeyB))
	f.Fuzz(func(t *testing.T, a, b []byte) {
		left := rtree.RangeStats{Range: rtree.Range{KeyRange: rtree.KeyRange{StartKey: a}, Files: []*backup.File{{TotalKvs: 1, TotalBytes: 1}}}}
		right := rtree.RangeStats{Range: rtree.Range{KeyRange: rtree.KeyRange{StartKey: b}, Files: []*backup.File{{TotalKvs: 1, TotalBytes: 1}}}}
		rtree.NeedsMerge(&left, &right, 42, 42)
	})
}
