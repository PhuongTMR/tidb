// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package executor

import (
	"bytes"
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/pingcap/tidb/pkg/executor/internal/exec"
	"github.com/pingcap/tidb/pkg/expression"
	"github.com/pingcap/tidb/pkg/kv"
	"github.com/pingcap/tidb/pkg/meta/model"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/planner/util"
	"github.com/pingcap/tidb/pkg/sessionctx/stmtctx"
	"github.com/pingcap/tidb/pkg/table/tables"
	"github.com/pingcap/tidb/pkg/tablecodec"
	"github.com/pingcap/tidb/pkg/types"
	"github.com/pingcap/tidb/pkg/util/mock"
	"github.com/stretchr/testify/require"
)

func TestSplitIndex(t *testing.T) {
	tbInfo := &model.TableInfo{
		Name: ast.NewCIStr("t1"),
		ID:   rand.Int63(),
		Columns: []*model.ColumnInfo{
			{
				Name:         ast.NewCIStr("c0"),
				ID:           1,
				Offset:       1,
				DefaultValue: 0,
				State:        model.StatePublic,
				FieldType:    *types.NewFieldType(mysql.TypeLong),
			},
		},
	}
	idxCols := []*model.IndexColumn{{Name: tbInfo.Columns[0].Name, Offset: 0, Length: types.UnspecifiedLength}}
	idxInfo := &model.IndexInfo{
		ID:      2,
		Name:    ast.NewCIStr("idx1"),
		Table:   ast.NewCIStr("t1"),
		Columns: idxCols,
		State:   model.StatePublic,
	}
	firstIdxInfo0 := idxInfo.Clone()
	firstIdxInfo0.ID = 1
	firstIdxInfo0.Name = ast.NewCIStr("idx")
	tbInfo.Indices = []*model.IndexInfo{firstIdxInfo0, idxInfo}

	// Test for int index.
	// range is 0 ~ 100, and split into 10 region.
	// So 10 regions range is like below, left close right open interval:
	// region1: [-inf ~ 10)
	// region2: [10 ~ 20)
	// region3: [20 ~ 30)
	// region4: [30 ~ 40)
	// region5: [40 ~ 50)
	// region6: [50 ~ 60)
	// region7: [60 ~ 70)
	// region8: [70 ~ 80)
	// region9: [80 ~ 90)
	// region10: [90 ~ +inf)
	ctx := mock.NewContext()
	e := &SplitIndexRegionExec{
		BaseExecutor: exec.NewBaseExecutor(ctx, nil, 0),
		tableInfo:    tbInfo,
		indexInfo:    idxInfo,
		lower:        []types.Datum{types.NewDatum(0)},
		upper:        []types.Datum{types.NewDatum(100)},
		num:          10,
	}
	valueList, err := e.getSplitIdxKeys()
	sort.Slice(valueList, func(i, j int) bool { return bytes.Compare(valueList[i], valueList[j]) < 0 })
	require.NoError(t, err)
	require.Len(t, valueList, e.num+1)

	cases := []struct {
		value        int
		lessEqualIdx int
	}{
		{-1, 0},
		{0, 0},
		{1, 0},
		{10, 1},
		{11, 1},
		{20, 2},
		{21, 2},
		{31, 3},
		{41, 4},
		{51, 5},
		{61, 6},
		{71, 7},
		{81, 8},
		{91, 9},
		{100, 9},
		{1000, 9},
	}

	index := tables.NewIndex(tbInfo.ID, tbInfo, idxInfo)
	for _, ca := range cases {
		// test for minInt64 handle
		sc := ctx.GetSessionVars().StmtCtx
		idxValue, _, err := index.GenIndexKey(sc.ErrCtx(), sc.TimeZone(), []types.Datum{types.NewDatum(ca.value)}, kv.IntHandle(math.MinInt64), nil)
		require.NoError(t, err)
		idx := searchLessEqualIdx(valueList, idxValue)
		require.Equal(t, idx, ca.lessEqualIdx)

		// Test for max int64 handle.
		idxValue, _, err = index.GenIndexKey(sc.ErrCtx(), sc.TimeZone(), []types.Datum{types.NewDatum(ca.value)}, kv.IntHandle(math.MaxInt64), nil)
		require.NoError(t, err)
		idx = searchLessEqualIdx(valueList, idxValue)
		require.Equal(t, idx, ca.lessEqualIdx)
	}
	// Test for varchar index.
	// range is a ~ z, and split into 26 region.
	// So 26 regions range is like below:
	// region1: [-inf ~ b)
	// region2: [b ~ c)
	// .
	// .
	// .
	// region26: [y ~ +inf)
	e.lower = []types.Datum{types.NewDatum("a")}
	e.upper = []types.Datum{types.NewDatum("z")}
	e.num = 26
	// change index column type to varchar
	tbInfo.Columns[0].FieldType = *types.NewFieldType(mysql.TypeVarchar)

	valueList, err = e.getSplitIdxKeys()
	sort.Slice(valueList, func(i, j int) bool { return bytes.Compare(valueList[i], valueList[j]) < 0 })
	require.NoError(t, err)
	require.Len(t, valueList, e.num+1)

	cases2 := []struct {
		value        string
		lessEqualIdx int
	}{
		{"", 0},
		{"a", 0},
		{"abcde", 0},
		{"b", 1},
		{"bzzzz", 1},
		{"c", 2},
		{"czzzz", 2},
		{"z", 25},
		{"zabcd", 25},
	}

	for _, ca := range cases2 {
		// test for minInt64 handle
		sc := ctx.GetSessionVars().StmtCtx
		idxValue, _, err := index.GenIndexKey(sc.ErrCtx(), sc.TimeZone(), []types.Datum{types.NewDatum(ca.value)}, kv.IntHandle(math.MinInt64), nil)
		require.NoError(t, err)
		idx := searchLessEqualIdx(valueList, idxValue)
		require.Equal(t, idx, ca.lessEqualIdx)

		// Test for max int64 handle.
		idxValue, _, err = index.GenIndexKey(sc.ErrCtx(), sc.TimeZone(), []types.Datum{types.NewDatum(ca.value)}, kv.IntHandle(math.MaxInt64), nil)
		require.NoError(t, err)
		idx = searchLessEqualIdx(valueList, idxValue)
		require.Equal(t, idx, ca.lessEqualIdx)
	}

	// Test for timestamp index.
	// range is 2010-01-01 00:00:00 ~ 2020-01-01 00:00:00, and split into 10 region.
	// So 10 regions range is like below:
	// region1: [-inf					~ 2011-01-01 00:00:00)
	// region2: [2011-01-01 00:00:00 	~ 2012-01-01 00:00:00)
	// .
	// .
	// .
	// region10: [2019-01-01 00:00:00 	~ +inf)
	lowerTime := types.NewTime(types.FromDate(2010, 1, 1, 0, 0, 0, 0), mysql.TypeTimestamp, types.DefaultFsp)
	upperTime := types.NewTime(types.FromDate(2020, 1, 1, 0, 0, 0, 0), mysql.TypeTimestamp, types.DefaultFsp)
	e.lower = []types.Datum{types.NewDatum(lowerTime)}
	e.upper = []types.Datum{types.NewDatum(upperTime)}
	e.num = 10

	// change index column type to timestamp
	tbInfo.Columns[0].FieldType = *types.NewFieldType(mysql.TypeTimestamp)

	valueList, err = e.getSplitIdxKeys()
	sort.Slice(valueList, func(i, j int) bool { return bytes.Compare(valueList[i], valueList[j]) < 0 })
	require.NoError(t, err)
	require.Len(t, valueList, e.num+1)

	cases3 := []struct {
		value        types.CoreTime
		lessEqualIdx int
	}{
		{types.FromDate(2009, 11, 20, 12, 50, 59, 0), 0},
		{types.FromDate(2010, 1, 1, 0, 0, 0, 0), 0},
		{types.FromDate(2011, 12, 31, 23, 59, 59, 0), 1},
		{types.FromDate(2011, 2, 1, 0, 0, 0, 0), 1},
		{types.FromDate(2012, 3, 1, 0, 0, 0, 0), 2},
		{types.FromDate(2013, 4, 1, 0, 0, 0, 0), 3},
		{types.FromDate(2014, 5, 1, 0, 0, 0, 0), 4},
		{types.FromDate(2015, 6, 1, 0, 0, 0, 0), 5},
		{types.FromDate(2016, 8, 1, 0, 0, 0, 0), 6},
		{types.FromDate(2017, 9, 1, 0, 0, 0, 0), 7},
		{types.FromDate(2018, 10, 1, 0, 0, 0, 0), 8},
		{types.FromDate(2019, 11, 1, 0, 0, 0, 0), 9},
		{types.FromDate(2020, 12, 1, 0, 0, 0, 0), 9},
		{types.FromDate(2030, 12, 1, 0, 0, 0, 0), 9},
	}

	for _, ca := range cases3 {
		value := types.NewTime(ca.value, mysql.TypeTimestamp, types.DefaultFsp)
		// test for min int64 handle
		sc := ctx.GetSessionVars().StmtCtx
		idxValue, _, err := index.GenIndexKey(sc.ErrCtx(), sc.TimeZone(), []types.Datum{types.NewDatum(value)}, kv.IntHandle(math.MinInt64), nil)
		require.NoError(t, err)
		idx := searchLessEqualIdx(valueList, idxValue)
		require.Equal(t, idx, ca.lessEqualIdx)

		// Test for max int64 handle.
		idxValue, _, err = index.GenIndexKey(sc.ErrCtx(), sc.TimeZone(), []types.Datum{types.NewDatum(value)}, kv.IntHandle(math.MaxInt64), nil)
		require.NoError(t, err)
		idx = searchLessEqualIdx(valueList, idxValue)
		require.Equal(t, idx, ca.lessEqualIdx)
	}
}

func TestSplitTable(t *testing.T) {
	tbInfo := &model.TableInfo{
		Name: ast.NewCIStr("t1"),
		ID:   rand.Int63(),
		Columns: []*model.ColumnInfo{
			{
				Name:         ast.NewCIStr("c0"),
				ID:           1,
				Offset:       1,
				DefaultValue: 0,
				State:        model.StatePublic,
				FieldType:    *types.NewFieldType(mysql.TypeLong),
			},
		},
	}
	defer func(originValue int64) {
		minRegionStepValue = originValue
	}(minRegionStepValue)
	minRegionStepValue = 10
	// range is 0 ~ 100, and split into 10 region.
	// So 10 regions range is like below:
	// region1: [-inf ~ 10)
	// region2: [10 ~ 20)
	// region3: [20 ~ 30)
	// region4: [30 ~ 40)
	// region5: [40 ~ 50)
	// region6: [50 ~ 60)
	// region7: [60 ~ 70)
	// region8: [70 ~ 80)
	// region9: [80 ~ 90 )
	// region10: [90 ~ +inf)
	ctx := mock.NewContext()
	e := &SplitTableRegionExec{
		BaseExecutor: exec.NewBaseExecutor(ctx, nil, 0),
		tableInfo:    tbInfo,
		handleCols:   util.NewIntHandleCols(&expression.Column{RetType: types.NewFieldType(mysql.TypeLonglong)}),
		lower:        []types.Datum{types.NewDatum(0)},
		upper:        []types.Datum{types.NewDatum(100)},
		num:          10,
	}
	valueList, err := e.getSplitTableKeys()
	require.NoError(t, err)
	require.Len(t, valueList, e.num-1)

	cases := []struct {
		value        int
		lessEqualIdx int
	}{
		{-1, -1},
		{0, -1},
		{1, -1},
		{10, 0},
		{11, 0},
		{20, 1},
		{21, 1},
		{31, 2},
		{41, 3},
		{51, 4},
		{61, 5},
		{71, 6},
		{81, 7},
		{91, 8},
		{100, 8},
		{1000, 8},
	}

	recordPrefix := tablecodec.GenTableRecordPrefix(e.tableInfo.ID)
	for _, ca := range cases {
		// test for minInt64 handle
		key := tablecodec.EncodeRecordKey(recordPrefix, kv.IntHandle(ca.value))
		require.NoError(t, err)
		idx := searchLessEqualIdx(valueList, key)
		require.Equal(t, idx, ca.lessEqualIdx)
	}
}

func TestStepShouldLargeThanMinStep(t *testing.T) {
	ctx := mock.NewContext()
	tbInfo := &model.TableInfo{
		Name: ast.NewCIStr("t1"),
		ID:   rand.Int63(),
		Columns: []*model.ColumnInfo{
			{
				Name:         ast.NewCIStr("c0"),
				ID:           1,
				Offset:       1,
				DefaultValue: 0,
				State:        model.StatePublic,
				FieldType:    *types.NewFieldType(mysql.TypeLong),
			},
		},
	}
	e1 := &SplitTableRegionExec{
		BaseExecutor: exec.NewBaseExecutor(ctx, nil, 0),
		tableInfo:    tbInfo,
		handleCols:   util.NewIntHandleCols(&expression.Column{RetType: types.NewFieldType(mysql.TypeLonglong)}),
		lower:        []types.Datum{types.NewDatum(0)},
		upper:        []types.Datum{types.NewDatum(1000)},
		num:          10,
	}
	_, err := e1.getSplitTableKeys()
	require.Equal(t, "[executor:8212]Failed to split region ranges: the region size is too small, expected at least 1000, but got 100", err.Error())
}

func TestClusterIndexSplitTable(t *testing.T) {
	tbInfo := &model.TableInfo{
		Name:                ast.NewCIStr("t"),
		ID:                  1,
		IsCommonHandle:      true,
		CommonHandleVersion: 1,
		Indices: []*model.IndexInfo{
			{
				ID:      1,
				Primary: true,
				State:   model.StatePublic,
				Columns: []*model.IndexColumn{
					{Offset: 1},
					{Offset: 2},
				},
			},
		},
		Columns: []*model.ColumnInfo{
			{
				Name:      ast.NewCIStr("c0"),
				ID:        1,
				Offset:    0,
				State:     model.StatePublic,
				FieldType: *types.NewFieldType(mysql.TypeDouble),
			},
			{
				Name:      ast.NewCIStr("c1"),
				ID:        2,
				Offset:    1,
				State:     model.StatePublic,
				FieldType: *types.NewFieldType(mysql.TypeLonglong),
			},
			{
				Name:      ast.NewCIStr("c2"),
				ID:        3,
				Offset:    2,
				State:     model.StatePublic,
				FieldType: *types.NewFieldType(mysql.TypeLonglong),
			},
		},
	}
	defer func(originValue int64) {
		minRegionStepValue = originValue
	}(minRegionStepValue)
	minRegionStepValue = 3
	ctx := mock.NewContext()
	e := &SplitTableRegionExec{
		BaseExecutor: exec.NewBaseExecutor(ctx, nil, 0),
		tableInfo:    tbInfo,
		handleCols:   buildHandleColsForSplit(tbInfo),
		lower:        types.MakeDatums(1, 0),
		upper:        types.MakeDatums(1, 100),
		num:          10,
	}
	valueList, err := e.getSplitTableKeys()
	require.NoError(t, err)
	require.Len(t, valueList, e.num-1)

	cases := []struct {
		value        []types.Datum
		lessEqualIdx int
	}{
		// For lower-bound and upper-bound, because 0 and 100 are padding with 7 zeros,
		// the split points are not (i * 10) but approximation.
		{types.MakeDatums(1, -1), -1},
		{types.MakeDatums(1, 0), -1},
		{types.MakeDatums(1, 10), -1},
		{types.MakeDatums(1, 11), 0},
		{types.MakeDatums(1, 20), 0},
		{types.MakeDatums(1, 21), 1},

		{types.MakeDatums(1, 31), 2},
		{types.MakeDatums(1, 41), 3},
		{types.MakeDatums(1, 51), 4},
		{types.MakeDatums(1, 61), 5},
		{types.MakeDatums(1, 71), 6},
		{types.MakeDatums(1, 81), 7},
		{types.MakeDatums(1, 91), 8},
		{types.MakeDatums(1, 100), 8},
		{types.MakeDatums(1, 101), 8},
	}

	recordPrefix := tablecodec.GenTableRecordPrefix(e.tableInfo.ID)
	sc := stmtctx.NewStmtCtxWithTimeZone(time.Local)
	for _, ca := range cases {
		h, err := e.handleCols.BuildHandleByDatums(sc, ca.value)
		require.NoError(t, err)
		key := tablecodec.EncodeRecordKey(recordPrefix, h)
		require.NoError(t, err)
		idx := searchLessEqualIdx(valueList, key)
		require.Equal(t, idx, ca.lessEqualIdx)
	}
}

func searchLessEqualIdx(valueList [][]byte, value []byte) int {
	idx := -1
	for i, v := range valueList {
		if bytes.Compare(value, v) >= 0 {
			idx = i
			continue
		}
		break
	}
	return idx
}
