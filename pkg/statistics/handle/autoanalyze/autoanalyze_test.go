// Copyright 2023 PingCAP, Inc.
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

package autoanalyze_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/pingcap/failpoint"
	"github.com/pingcap/tidb/pkg/domain"
	"github.com/pingcap/tidb/pkg/domain/infosync"
	"github.com/pingcap/tidb/pkg/domain/serverinfo"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/sessionctx"
	"github.com/pingcap/tidb/pkg/sessionctx/vardef"
	"github.com/pingcap/tidb/pkg/sessionctx/variable"
	"github.com/pingcap/tidb/pkg/statistics"
	"github.com/pingcap/tidb/pkg/statistics/handle/autoanalyze"
	statstestutil "github.com/pingcap/tidb/pkg/statistics/handle/ddl/testutil"
	"github.com/pingcap/tidb/pkg/statistics/handle/util"
	"github.com/pingcap/tidb/pkg/statistics/handle/util/test"
	"github.com/pingcap/tidb/pkg/store/mockstore"
	"github.com/pingcap/tidb/pkg/testkit"
	"github.com/pingcap/tidb/pkg/testkit/testfailpoint"
	"github.com/pingcap/tidb/pkg/types"
	"github.com/pingcap/tidb/pkg/util/chunk"
	mockexec "github.com/pingcap/tidb/pkg/util/mock"
	"github.com/pingcap/tidb/pkg/util/sqlexec/mock"
	"github.com/stretchr/testify/require"
	"github.com/tikv/client-go/v2/oracle"
	"go.uber.org/mock/gomock"
)

// WrapAsSCtx wraps the MockRestrictedSQLExecutor into sessionctx.Context.
func WrapAsSCtx(exec *mock.MockRestrictedSQLExecutor) sessionctx.Context {
	sctx := mockexec.NewContext()
	sctx.SetValue(mock.RestrictedSQLExecutorKey{}, exec)
	return sctx
}

func TestEnableAutoAnalyzePriorityQueue(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec("create table t (a int)")
	tk.MustExec("insert into t values (1)")
	// Enable auto analyze priority queue.
	tk.MustExec("SET GLOBAL tidb_enable_auto_analyze_priority_queue=ON")
	require.True(t, vardef.EnableAutoAnalyzePriorityQueue.Load())
	h := dom.StatsHandle()
	err := statstestutil.HandleNextDDLEventWithTxn(h)
	require.NoError(t, err)
	require.NoError(t, h.DumpStatsDeltaToKV(true))
	is := dom.InfoSchema()
	require.NoError(t, h.Update(context.Background(), is))
	statistics.AutoAnalyzeMinCnt = 0
	defer func() {
		statistics.AutoAnalyzeMinCnt = 1000
	}()
	require.True(t, dom.StatsHandle().HandleAutoAnalyze())
}

func TestAutoAnalyzeLockedTable(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec("create table t (a int)")
	tk.MustExec("insert into t values (1)")
	h := dom.StatsHandle()
	err := statstestutil.HandleNextDDLEventWithTxn(h)
	require.NoError(t, err)
	require.NoError(t, h.DumpStatsDeltaToKV(true))
	// Lock the table.
	tk.MustExec("lock stats t")
	is := dom.InfoSchema()
	require.NoError(t, h.Update(context.Background(), is))
	statistics.AutoAnalyzeMinCnt = 0
	defer func() {
		statistics.AutoAnalyzeMinCnt = 1000
	}()
	// Try to analyze the locked table, it should not analyze the table.
	require.False(t, dom.StatsHandle().HandleAutoAnalyze())

	// Unlock the table.
	tk.MustExec("unlock stats t")
	require.NoError(t, h.DumpStatsDeltaToKV(true))
	require.NoError(t, h.Update(context.Background(), is))
	// Try again, it should analyze the table.
	require.True(t, dom.StatsHandle().HandleAutoAnalyze())
}

func TestAutoAnalyzeWithPredicateColumns(t *testing.T) {
	// Create a table and add it to stats cache.
	store, dom := testkit.CreateMockStoreAndDomain(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec("create table t (a int, b int)")
	tk.MustExec("insert into t values (1, 1)")
	tk.MustQuery("select * from t where a > 0").Check(testkit.Rows("1 1"))
	h := dom.StatsHandle()
	err := statstestutil.HandleNextDDLEventWithTxn(h)
	require.NoError(t, err)
	require.NoError(t, h.DumpColStatsUsageToKV())
	require.NoError(t, h.DumpStatsDeltaToKV(true))
	is := dom.InfoSchema()
	require.NoError(t, h.Update(context.Background(), is))
	statistics.AutoAnalyzeMinCnt = 0
	defer func() {
		statistics.AutoAnalyzeMinCnt = 1000
	}()

	// Check column_stats_usage.
	rows := tk.MustQuery(
		"show column_stats_usage where db_name = 'test' and table_name = 't' and last_used_at is not null",
	).Rows()
	require.Equal(t, 1, len(rows))
	require.Equal(t, "a", rows[0][3])

	// Set tidb_analyze_column_options to PREDICATE.
	tk.MustExec("set global tidb_analyze_column_options='PREDICATE'")

	// Trigger auto analyze.
	require.True(t, dom.StatsHandle().HandleAutoAnalyze())

	// Check analyze jobs.
	tk.MustQuery("select table_name, job_info from mysql.analyze_jobs order by id desc limit 1").Check(
		testkit.Rows("t auto analyze table column a with 256 buckets, 100 topn, 1 samplerate"),
	)
}

func TestDisableAutoAnalyze(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	tk := testkit.NewTestKit(t, store)
	disableAutoAnalyzeCase(t, tk, dom)
}

func TestDisableAutoAnalyzeWithAnalyzeAllColumnsOptions(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	tk := testkit.NewTestKit(t, store)
	// Set tidb_analyze_column_options to ALL.
	tk.MustExec("set global tidb_analyze_column_options='ALL'")
	disableAutoAnalyzeCase(t, tk, dom)
}

func disableAutoAnalyzeCase(t *testing.T, tk *testkit.TestKit, dom *domain.Domain) {
	tk.MustExec("use test")
	tk.MustExec("create table t (a int)")
	tk.MustExec("insert into t values (1)")
	h := dom.StatsHandle()
	err := statstestutil.HandleNextDDLEventWithTxn(h)
	require.NoError(t, err)
	require.NoError(t, h.DumpStatsDeltaToKV(true))
	is := dom.InfoSchema()
	require.NoError(t, h.Update(context.Background(), is))

	tk.MustExec("set @@global.tidb_enable_auto_analyze = 0")
	statistics.AutoAnalyzeMinCnt = 0
	defer func() {
		statistics.AutoAnalyzeMinCnt = 1000
	}()
	// Even auto analyze ratio is set to 0, we still need to analyze the unanalyzed tables.
	require.True(t, dom.StatsHandle().HandleAutoAnalyze())
	require.NoError(t, h.Update(context.Background(), is))

	// Try again, it should not analyze the table because it's already analyzed and auto analyze ratio is 0.
	require.False(t, dom.StatsHandle().HandleAutoAnalyze())

	// Index analyze doesn't depend on auto analyze ratio. Only control by tidb_enable_auto_analyze.
	// Even auto analyze ratio is set to 0, we still need to analyze the newly created index.
	tk.MustExec("alter table t add index ia(a)")
	require.Eventually(t, func() bool {
		return dom.StatsHandle().HandleAutoAnalyze()
	}, 10*time.Second, 100*time.Millisecond)
}

func TestAutoAnalyzeOnChangeAnalyzeVer(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec("create table t(a int, index idx(a))")
	tk.MustExec("insert into t values(1)")
	tk.MustExec("set @@global.tidb_analyze_version = 1")
	do := dom
	statistics.AutoAnalyzeMinCnt = 0
	defer func() {
		statistics.AutoAnalyzeMinCnt = 1000
	}()
	h := do.StatsHandle()
	err := statstestutil.HandleNextDDLEventWithTxn(h)
	require.NoError(t, err)
	require.NoError(t, h.DumpStatsDeltaToKV(true))
	is := do.InfoSchema()
	require.NoError(t, h.Update(context.Background(), is))
	// Auto analyze when global ver is 1.
	h.HandleAutoAnalyze()
	require.NoError(t, h.Update(context.Background(), is))
	tbl, err := is.TableByName(context.Background(), ast.NewCIStr("test"), ast.NewCIStr("t"))
	require.NoError(t, err)
	statsTbl1 := h.GetTableStats(tbl.Meta())
	// Check that all the version of t's stats are 1.
	statsTbl1.ForEachColumnImmutable(func(_ int64, col *statistics.Column) bool {
		require.Equal(t, int64(1), col.GetStatsVer())
		return false
	})
	statsTbl1.ForEachIndexImmutable(func(_ int64, idx *statistics.Index) bool {
		require.Equal(t, int64(1), idx.GetStatsVer())
		return false
	})
	require.Equal(t, 1, statsTbl1.StatsVer)
	tk.MustExec("set @@global.tidb_analyze_version = 2")
	tk.MustExec("insert into t values(1), (2), (3), (4)")
	require.NoError(t, h.DumpStatsDeltaToKV(true))
	require.NoError(t, h.Update(context.Background(), is))
	// Auto analyze t whose version is 1 after setting global ver to 2.
	h.HandleAutoAnalyze()
	require.NoError(t, h.Update(context.Background(), is))
	statsTbl1 = h.GetTableStats(tbl.Meta())
	require.Equal(t, int64(5), statsTbl1.RealtimeCount)
	// All of its statistics should still be version 1.
	statsTbl1.ForEachColumnImmutable(func(_ int64, col *statistics.Column) bool {
		require.Equal(t, int64(1), col.GetStatsVer())
		return false
	})
	statsTbl1.ForEachIndexImmutable(func(_ int64, idx *statistics.Index) bool {
		require.Equal(t, int64(1), idx.GetStatsVer())
		return false
	})
	require.Equal(t, 1, statsTbl1.StatsVer)
	// Add a new table after the analyze version set to 2.
	tk.MustExec("create table tt(a int, index idx(a))")
	tk.MustExec("insert into tt values(1), (2), (3), (4), (5)")
	err = statstestutil.HandleNextDDLEventWithTxn(h)
	require.NoError(t, err)
	require.NoError(t, h.DumpStatsDeltaToKV(true))
	is = do.InfoSchema()
	tbl2, err := is.TableByName(context.Background(), ast.NewCIStr("test"), ast.NewCIStr("tt"))
	require.NoError(t, err)
	require.NoError(t, h.Update(context.Background(), is))
	h.HandleAutoAnalyze()
	require.NoError(t, h.Update(context.Background(), is))
	statsTbl2 := h.GetTableStats(tbl2.Meta())
	// Since it's a newly created table. Auto analyze should analyze it's statistics to version2.
	statsTbl2.ForEachColumnImmutable(func(_ int64, col *statistics.Column) bool {
		require.Equal(t, int64(2), col.GetStatsVer())
		return false
	})
	statsTbl2.ForEachIndexImmutable(func(_ int64, idx *statistics.Index) bool {
		require.Equal(t, int64(2), idx.GetStatsVer())
		return false
	})
	require.Equal(t, 2, statsTbl2.StatsVer)
	tk.MustExec("set @@global.tidb_analyze_version = 1")
}

func TestTableAnalyzed(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	testKit := testkit.NewTestKit(t, store)
	testKit.MustExec("use test")
	testKit.MustExec("create table t (a int, index idx(a))")
	testKit.MustExec("insert into t values (1)")

	is := dom.InfoSchema()
	tbl, err := is.TableByName(context.Background(), ast.NewCIStr("test"), ast.NewCIStr("t"))
	require.NoError(t, err)
	tableInfo := tbl.Meta()
	h := dom.StatsHandle()

	require.NoError(t, h.Update(context.Background(), is))
	statsTbl := h.GetTableStats(tableInfo)
	require.False(t, statsTbl.LastAnalyzeVersion > 0)

	testKit.MustExec("analyze table t")
	require.NoError(t, h.Update(context.Background(), is))
	statsTbl = h.GetTableStats(tableInfo)
	require.True(t, statsTbl.LastAnalyzeVersion > 0)

	h.Clear()
	oriLease := h.Lease()
	// set it to non-zero so we will use load by need strategy
	h.SetLease(1)
	defer func() {
		h.SetLease(oriLease)
	}()
	require.NoError(t, h.Update(context.Background(), is))
	statsTbl = h.GetTableStats(tableInfo)
	require.True(t, statsTbl.LastAnalyzeVersion > 0)
}

func TestNeedAnalyzeTable(t *testing.T) {
	columns := map[int64]*statistics.Column{}
	columns[1] = &statistics.Column{StatsVer: statistics.Version2}
	tests := []struct {
		tbl    *statistics.Table
		ratio  float64
		result bool
		reason string
	}{
		// table was never analyzed and has reach the limit
		{
			tbl:    &statistics.Table{Version: oracle.GoTimeToTS(time.Now())},
			ratio:  0,
			result: true,
			reason: "table unanalyzed",
		},
		// table was never analyzed but has not reached the limit
		{
			tbl:    &statistics.Table{Version: oracle.GoTimeToTS(time.Now())},
			ratio:  0,
			result: true,
			reason: "table unanalyzed",
		},
		// table was already analyzed but auto analyze is disabled
		{
			tbl:    &statistics.Table{HistColl: *statistics.NewHistCollWithColsAndIdxs(0, 1, 1, columns, nil), LastAnalyzeVersion: 1},
			ratio:  0,
			result: false,
			reason: "",
		},
		// table was already analyzed but modify count is small
		{
			tbl:    &statistics.Table{HistColl: *statistics.NewHistCollWithColsAndIdxs(0, 1, 0, columns, nil), LastAnalyzeVersion: 1},
			ratio:  0.3,
			result: false,
			reason: "",
		},
		// table was already analyzed
		{
			tbl:    &statistics.Table{HistColl: *statistics.NewHistCollWithColsAndIdxs(0, 1, 1, columns, nil), LastAnalyzeVersion: 1},
			ratio:  0.3,
			result: true,
			reason: "too many modifications",
		},
	}
	for _, test := range tests {
		needAnalyze, reason := autoanalyze.NeedAnalyzeTable(test.tbl, test.ratio)
		require.Equal(t, test.result, needAnalyze)
		require.True(t, strings.HasPrefix(reason, test.reason))
	}
}

func TestAutoAnalyzeSkipColumnTypes(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec("create table t(a int, b int, c json, d text, e mediumtext, f blob, g mediumblob, index idx(d(10)))")
	statstestutil.HandleNextDDLEventWithTxn(dom.StatsHandle())
	tk.MustExec("insert into t values (1, 2, null, 'xxx', 'yyy', null, null)")
	tk.MustExec("select * from t where a = 1 and b = 1 and c = '1'")
	h := dom.StatsHandle()
	require.NoError(t, h.DumpStatsDeltaToKV(true))
	require.NoError(t, h.Update(context.Background(), dom.InfoSchema()))
	require.NoError(t, h.DumpColStatsUsageToKV())
	tk.MustExec("set @@global.tidb_analyze_skip_column_types = 'json,blob,mediumblob,text,mediumtext'")

	originalVal := statistics.AutoAnalyzeMinCnt
	statistics.AutoAnalyzeMinCnt = 0
	defer func() {
		statistics.AutoAnalyzeMinCnt = originalVal
	}()
	require.True(t, h.HandleAutoAnalyze())
	tk.MustQuery("select job_info from mysql.analyze_jobs where job_info like '%auto analyze table%'").Check(testkit.Rows("auto analyze table all indexes, columns a, b, d with 256 buckets, 100 topn, 1 samplerate"))
}

func TestAutoAnalyzeOnEmptyTable(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	tk := testkit.NewTestKit(t, store)

	oriStart := tk.MustQuery("select @@tidb_auto_analyze_start_time").Rows()[0][0].(string)
	oriEnd := tk.MustQuery("select @@tidb_auto_analyze_end_time").Rows()[0][0].(string)
	defer func() {
		tk.MustExec(fmt.Sprintf("set global tidb_auto_analyze_start_time='%v'", oriStart))
		tk.MustExec(fmt.Sprintf("set global tidb_auto_analyze_end_time='%v'", oriEnd))
	}()

	tt := time.Now().Add(-1 * time.Minute)
	h, m := tt.Hour(), tt.Minute()
	start, end := fmt.Sprintf("%02d:%02d +0000", h, m), fmt.Sprintf("%02d:%02d +0000", h, m)
	tk.MustExec(fmt.Sprintf("set global tidb_auto_analyze_start_time='%v'", start))
	tk.MustExec(fmt.Sprintf("set global tidb_auto_analyze_end_time='%v'", end))
	dom.StatsHandle().HandleAutoAnalyze()

	tk.MustExec("use test")
	tk.MustExec("create table t (a int, index idx(a))")
	// to pass the stats.Pseudo check in autoAnalyzeTable
	tk.MustExec("analyze table t")
	// to pass the AutoAnalyzeMinCnt check in autoAnalyzeTable
	tk.MustExec("insert into t values (1)" + strings.Repeat(", (1)", int(statistics.AutoAnalyzeMinCnt)))
	require.NoError(t, dom.StatsHandle().DumpStatsDeltaToKV(true))
	require.NoError(t, dom.StatsHandle().Update(context.Background(), dom.InfoSchema()))

	// test if it will be limited by the time range
	require.False(t, dom.StatsHandle().HandleAutoAnalyze())

	tk.MustExec("set global tidb_auto_analyze_start_time='00:00 +0000'")
	tk.MustExec("set global tidb_auto_analyze_end_time='23:59 +0000'")
	require.True(t, dom.StatsHandle().HandleAutoAnalyze())
}

func TestAutoAnalyzeOutOfSpecifiedTime(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	tk := testkit.NewTestKit(t, store)

	oriStart := tk.MustQuery("select @@tidb_auto_analyze_start_time").Rows()[0][0].(string)
	oriEnd := tk.MustQuery("select @@tidb_auto_analyze_end_time").Rows()[0][0].(string)
	defer func() {
		tk.MustExec(fmt.Sprintf("set global tidb_auto_analyze_start_time='%v'", oriStart))
		tk.MustExec(fmt.Sprintf("set global tidb_auto_analyze_end_time='%v'", oriEnd))
	}()

	tt := time.Now().Add(-1 * time.Minute)
	h, m := tt.Hour(), tt.Minute()
	start, end := fmt.Sprintf("%02d:%02d +0000", h, m), fmt.Sprintf("%02d:%02d +0000", h, m)
	tk.MustExec(fmt.Sprintf("set global tidb_auto_analyze_start_time='%v'", start))
	tk.MustExec(fmt.Sprintf("set global tidb_auto_analyze_end_time='%v'", end))
	dom.StatsHandle().HandleAutoAnalyze()

	tk.MustExec("use test")
	tk.MustExec("create table t (a int)")
	statstestutil.HandleNextDDLEventWithTxn(dom.StatsHandle())
	// to pass the stats.Pseudo check in autoAnalyzeTable
	tk.MustExec("analyze table t")
	// to pass the AutoAnalyzeMinCnt check in autoAnalyzeTable
	tk.MustExec("insert into t values (1)" + strings.Repeat(", (1)", int(statistics.AutoAnalyzeMinCnt)))
	require.NoError(t, dom.StatsHandle().DumpStatsDeltaToKV(true))
	require.NoError(t, dom.StatsHandle().Update(context.Background(), dom.InfoSchema()))

	require.False(t, dom.StatsHandle().HandleAutoAnalyze())
	tk.MustExec("analyze table t")

	tk.MustExec("alter table t add index ia(a)")
	require.False(t, dom.StatsHandle().HandleAutoAnalyze())

	tk.MustExec("set global tidb_auto_analyze_start_time='00:00 +0000'")
	tk.MustExec("set global tidb_auto_analyze_end_time='23:59 +0000'")
	require.True(t, dom.StatsHandle().HandleAutoAnalyze())
}

func makeFailpointRes(t *testing.T, v any) string {
	bytes, err := json.Marshal(v)
	require.NoError(t, err)
	return fmt.Sprintf("return(`%s`)", string(bytes))
}

func getMockedServerInfo() map[string]*serverinfo.ServerInfo {
	mockedAllServerInfos := map[string]*serverinfo.ServerInfo{
		"s1": {
			StaticInfo: serverinfo.StaticInfo{
				ID:   "s1",
				IP:   "127.0.0.1",
				Port: 4000,
			},
		},
		"s2": {
			StaticInfo: serverinfo.StaticInfo{
				ID:   "s2",
				IP:   "127.0.0.2",
				Port: 4000,
			},
		},
	}
	return mockedAllServerInfos
}

func TestCleanupCorruptedAnalyzeJobsOnCurrentInstance(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec := mock.NewMockRestrictedSQLExecutor(ctrl)

	require.NoError(t,
		failpoint.Enable(
			"github.com/pingcap/tidb/pkg/domain/infosync/mockGetServerInfo",
			makeFailpointRes(t, getMockedServerInfo()["s1"]),
		),
	)
	defer func() {
		failpoint.Disable("github.com/pingcap/tidb/pkg/domain/infosync/mockGetServerInfo")
	}()

	// Create a new chunk with capacity for three fields
	c := chunk.NewChunkWithCapacity([]*types.FieldType{
		types.NewFieldType(mysql.TypeLonglong), // id
		types.NewFieldType(mysql.TypeLonglong), // process_id
		types.NewFieldType(mysql.TypeVarchar),  // instance
	}, 3)

	// Append values for each field
	c.AppendInt64(0, int64(1)) // id
	c.AppendInt64(1, int64(1)) // process_id

	c.AppendInt64(0, int64(2)) // id
	c.AppendNull(1)            // process_id

	c.AppendInt64(0, int64(3)) // id
	c.AppendInt64(1, int64(3)) // process_id
	// Create a row from the chunk
	rows := []chunk.Row{c.GetRow(0), c.GetRow(1), c.GetRow(2)}

	// Set up the mock function to return the row
	exec.EXPECT().ExecRestrictedSQL(
		gomock.All(&test.CtxMatcher{}),
		util.UseCurrentSessionOpt,
		autoanalyze.SelectAnalyzeJobsOnCurrentInstanceSQL,
		"127.0.0.1:4000",
		gomock.Any(),
	).Return(rows, nil, nil)

	exec.EXPECT().ExecRestrictedSQL(
		gomock.All(&test.CtxMatcher{}),
		util.UseCurrentSessionOpt,
		autoanalyze.BatchUpdateAnalyzeJobSQL,
		[]any{[]string{"1"}},
	).Return(nil, nil, nil)

	err := autoanalyze.CleanupCorruptedAnalyzeJobsOnCurrentInstance(
		WrapAsSCtx(exec),
		map[uint64]struct{}{
			3: {},
			4: {},
		},
	)
	require.NoError(t, err)

	// Set up the mock function to return the row
	exec.EXPECT().ExecRestrictedSQL(
		gomock.All(&test.CtxMatcher{}),
		util.UseCurrentSessionOpt,
		autoanalyze.SelectAnalyzeJobsOnCurrentInstanceSQL,
		"127.0.0.1:4000",
	).Return(rows, nil, nil)

	exec.EXPECT().ExecRestrictedSQL(
		gomock.All(&test.CtxMatcher{}),
		util.UseCurrentSessionOpt,
		autoanalyze.BatchUpdateAnalyzeJobSQL,
		[]any{[]string{"1", "3"}},
	).Return(nil, nil, nil)

	// No running analyze jobs on current instance.
	err = autoanalyze.CleanupCorruptedAnalyzeJobsOnCurrentInstance(
		WrapAsSCtx(exec),
		map[uint64]struct{}{},
	)
	require.NoError(t, err)
}

func TestCleanupCorruptedAnalyzeJobsOnDeadInstances(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	exec := mock.NewMockRestrictedSQLExecutor(ctrl)

	require.NoError(
		t,
		failpoint.Enable(
			"github.com/pingcap/tidb/pkg/domain/infosync/mockGetAllServerInfo",
			makeFailpointRes(t, getMockedServerInfo()),
		),
	)
	defer func() {
		require.NoError(
			t, failpoint.Disable("github.com/pingcap/tidb/pkg/domain/infosync/mockGetAllServerInfo"),
		)
	}()
	// Create a new chunk with capacity for three fields
	c := chunk.NewChunkWithCapacity([]*types.FieldType{
		types.NewFieldType(mysql.TypeLonglong), // id
		types.NewFieldType(mysql.TypeVarchar),  // instance
	}, 3)

	// Append values for each field
	c.AppendInt64(0, int64(1))          // id
	c.AppendString(1, "127.0.0.1:4000") // instance

	c.AppendInt64(0, int64(2))         // id
	c.AppendString(1, "10.0.0.1:4000") // unknown instance

	c.AppendInt64(0, int64(3))          // id
	c.AppendString(1, "127.0.0.1:4000") // valid instance
	// Create a row from the chunk
	rows := []chunk.Row{c.GetRow(0), c.GetRow(1), c.GetRow(2)}
	// Set up the mock function to return the row
	exec.EXPECT().ExecRestrictedSQL(
		gomock.All(&test.CtxMatcher{}),
		util.UseCurrentSessionOpt,
		autoanalyze.SelectAnalyzeJobsSQL,
		gomock.Any(),
	).Return(rows, nil, nil)

	exec.EXPECT().ExecRestrictedSQL(
		gomock.All(&test.CtxMatcher{}),
		util.UseCurrentSessionOpt,
		autoanalyze.BatchUpdateAnalyzeJobSQL,
		[]any{[]string{"2"}},
	).Return(nil, nil, nil)

	err := autoanalyze.CleanupCorruptedAnalyzeJobsOnDeadInstances(
		WrapAsSCtx(exec),
	)
	require.NoError(t, err)
}

func TestSkipAutoAnalyzeOutsideTheAvailableTime(t *testing.T) {
	store, dom := testkit.CreateMockStoreAndDomain(t)
	tk := testkit.NewTestKit(t, store)
	ttStart := time.Now().Add(-2 * time.Hour)
	ttEnd := time.Now().Add(-1 * time.Hour)
	for i := range 2 {
		dbName := fmt.Sprintf("db%d", i)
		tk.MustExec(fmt.Sprintf("create database %s", dbName))
		for j := range 2 {
			tableName := fmt.Sprintf("table%d", j)
			tk.MustExec(fmt.Sprintf("create table %s.%s (a int)", dbName, tableName))
		}
	}
	se, err := dom.SysSessionPool().Get()
	require.NoError(t, err)
	require.False(t,
		autoanalyze.RandomPickOneTableAndTryAutoAnalyze(
			se.(sessionctx.Context),
			dom.StatsHandle(),
			dom.SysProcTracker(),
			0.6,
			variable.Dynamic,
			ttStart,
			ttEnd,
		),
	)
}

const tiflashReplicaLease = 600 * time.Millisecond

func TestAutoAnalyzeWithVectorIndex(t *testing.T) {
	store := testkit.CreateMockStoreWithSchemaLease(t, tiflashReplicaLease, mockstore.WithMockTiFlash(2))
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")

	tiflash := infosync.NewMockTiFlash()
	infosync.SetMockTiFlash(tiflash)
	defer func() {
		tiflash.Lock()
		tiflash.StatusServer.Close()
		tiflash.Unlock()
	}()
	dom := domain.GetDomain(tk.Session())
	h := dom.StatsHandle()

	testfailpoint.Enable(t, "github.com/pingcap/tidb/pkg/ddl/MockCheckColumnarIndexProcess", `return(1)`)
	statistics.AutoAnalyzeMinCnt = 0
	defer func() {
		statistics.AutoAnalyzeMinCnt = 1000
	}()

	tk.MustExec("use test")
	tk.MustExec("create table t (a int, b vector, c vector(3), d vector(4));")
	tk.MustExec("insert into t values(1, '[1, 2]', '[1, 3, 4]', '[1, 4, 5, 6]')")
	tk.MustExec("SET GLOBAL tidb_enable_auto_analyze_priority_queue=off")
	tk.MustExec("analyze table t all columns")
	tbl, err := dom.InfoSchema().TableByName(context.Background(), ast.NewCIStr("test"), ast.NewCIStr("t"))
	require.NoError(t, err)
	tableInfo := tbl.Meta()
	statsTbl := h.GetTableStats(tableInfo)
	require.True(t, statsTbl.LastAnalyzeVersion > 0)
	require.NoError(t, h.Update(context.Background(), dom.InfoSchema()))
	tk.MustExec("alter table t add index idx(a)")
	// Normal Index can trigger auto analyze.
	require.True(t, h.HandleAutoAnalyze())
	tk.MustExec("alter table t set tiflash replica 1")
	testkit.SetTiFlashReplica(t, dom, "test", "t")
	tk.MustExec("alter table t add vector index vecIdx1((vec_cosine_distance(d))) USING HNSW;")
	// Vector Index can not trigger auto analyze.
	require.False(t, h.HandleAutoAnalyze())
}
