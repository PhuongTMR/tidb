// Copyright 2022 PingCAP, Inc.
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

package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/planner"
	"github.com/pingcap/tidb/pkg/planner/core"
	"github.com/pingcap/tidb/pkg/planner/core/base"
	"github.com/pingcap/tidb/pkg/planner/core/resolve"
	"github.com/pingcap/tidb/pkg/planner/property"
	"github.com/pingcap/tidb/pkg/planner/util/costusage"
	"github.com/pingcap/tidb/pkg/planner/util/optimizetrace"
	"github.com/pingcap/tidb/pkg/sessiontxn"
	"github.com/pingcap/tidb/pkg/testkit"
	"github.com/pingcap/tidb/tests/realtikvtest"
	"github.com/stretchr/testify/require"
)

func TestCostModelVer2ScanRowSize(t *testing.T) {
	store := testkit.CreateMockStore(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec(`create table t (pk int, a int, b int, c int, d int, primary key(pk), index ab(a, b), index abc(a, b, c))`)
	tk.MustExec("insert into t values (1, 1, 1, 1, 1)")
	tk.MustExec(`set @@tidb_cost_model_version=2`)
	tk.MustExec("set global tidb_enable_collect_execution_info=1;")

	cases := []struct {
		query       string
		scanFormula string
	}{
		// index scan row-size on idx_ab is always equal to row-size(index_ab)
		{"select a from t use index(ab) where a=1", "(scan(1*logrowsize(32)*tikv_scan_factor(40.7)))*1.00"},
		{"select a, b from t use index(ab) where a=1", "(scan(1*logrowsize(32)*tikv_scan_factor(40.7)))*1.00"},
		{"select b from t use index(ab) where a=1 and b=1", "(scan(1*logrowsize(32)*tikv_scan_factor(40.7)))*1.00"},
		// index scan row-size on idx_abc is always equal to row-size(index_abc)
		{"select a from t use index(abc) where a=1", "(scan(1*logrowsize(48)*tikv_scan_factor(40.7)))*1.00"},
		{"select a from t use index(abc) where a=1 and b=1", "(scan(1*logrowsize(48)*tikv_scan_factor(40.7)))*1.00"},
		{"select a, b from t use index(abc) where a=1 and b=1", "(scan(1*logrowsize(48)*tikv_scan_factor(40.7)))*1.00"},
		{"select a, b, c from t use index(abc) where a=1 and b=1 and c=1", "(scan(1*logrowsize(48)*tikv_scan_factor(40.7)))*1.00"},
		// table scan row-size is always equal to row-size(*)
		{"select a from t use index(primary) where a=1", "((scan(1*logrowsize(80)*tikv_scan_factor(40.7))) + (scan(1000*logrowsize(80)*tikv_scan_factor(40.7))))*1.00"},
		{"select a, d from t use index(primary) where a=1", "((scan(1*logrowsize(80)*tikv_scan_factor(40.7))) + (scan(1000*logrowsize(80)*tikv_scan_factor(40.7))))*1.00"},
		{"select * from t use index(primary) where a=1", "((scan(1*logrowsize(80)*tikv_scan_factor(40.7))) + (scan(1000*logrowsize(80)*tikv_scan_factor(40.7))))*1.00"},
	}
	for _, c := range cases {
		rs := tk.MustQuery("explain analyze format=true_card_cost " + c.query).Rows()
		scan := rs[len(rs)-1]
		formula := scan[3]
		require.Equal(t, formula, c.scanFormula)
	}

	tk.MustQuery("explain select a from t where a=1").Check(testkit.Rows(
		`IndexReader_6 10.00 root  index:IndexRangeScan_5`, // use idx_ab automatically since it has the smallest row-size in all access paths.
		`└─IndexRangeScan_5 10.00 cop[tikv] table:t, index:ab(a, b) range:[1,1], keep order:false, stats:pseudo`))
	tk.MustQuery("explain select a, b, c from t where a=1").Check(testkit.Rows(
		`IndexReader_6 10.00 root  index:IndexRangeScan_5`, // use idx_abc automatically
		`└─IndexRangeScan_5 10.00 cop[tikv] table:t, index:abc(a, b, c) range:[1,1], keep order:false, stats:pseudo`))
}

func TestCostModelTraceVer2(t *testing.T) {
	store := testkit.CreateMockStore(t)
	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec(`create table t (a int primary key, b int, c int, key(b))`)
	vals := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		vals = append(vals, fmt.Sprintf("(%v, %v, %v)", i, i, i))
	}
	tk.MustExec(fmt.Sprintf("insert into t values %v", strings.Join(vals, ", ")))
	tk.MustExec("analyze table t")
	tk.MustExec("set @@tidb_cost_model_version=2")

	for _, q := range []string{
		"select * from t",
		"select * from t where a<4",
		"select * from t use index(b) where b<4",
		"select * from t where a<4 order by b",
		"select * from t where a<4 order by b limit 3",
		"select sum(a) from t where a<4 group by b, c",
		"select max(a), b, c from t where a<4 group by b, c",
		"select * from t t1, t t2",
		"select * from t t1, t t2 where t1.a=t2.a",
		"select /*+ tidb_inlj(t1, t2) */ * from t t1, t t2 where t1.b=t2.b",
	} {
		plan := tk.MustQuery("explain analyze format='true_card_cost' " + q).Rows()
		planCost, err := strconv.ParseFloat(plan[0][2].(string), 64)
		require.Nil(t, err)

		// check the accuracy of factor costs
		ok := false
		warns := tk.MustQuery("show warnings").Rows()
		for _, warn := range warns {
			msg := warn[2].(string)
			if strings.HasPrefix(msg, "factor costs: ") {
				costData := msg[len("factor costs: "):]
				var factorCosts map[string]float64
				require.Nil(t, json.Unmarshal([]byte(costData), &factorCosts))
				var sum float64
				for _, factorCost := range factorCosts {
					sum += factorCost
				}
				absDiff := math.Abs(sum - planCost)
				if absDiff < 5 || absDiff/planCost < 0.01 {
					ok = true
				}
			}
		}
		require.True(t, ok)
	}
}

func BenchmarkGetPlanCost(b *testing.B) {
	store := testkit.CreateMockStore(b)
	tk := testkit.NewTestKit(b, store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t (a int, b int);")

	p := parser.New()
	sql := "select sum(t1.b), t1.a from t t1, t t2 where t1.a>0 and t2.a>10 and t1.b=t2.b group by t1.a order by t1.a limit 5"
	stmt, err := p.ParseOneStmt(sql, "", "")
	if err != nil {
		b.Fatal(err)
	}
	sctx := tk.Session()
	sctx.GetSessionVars().CostModelVersion = 2
	is := sessiontxn.GetTxnManager(sctx).GetTxnInfoSchema()
	nodeW := resolve.NewNodeW(stmt)
	plan, _, err := planner.Optimize(context.TODO(), sctx, nodeW, is)
	if err != nil {
		b.Fatal(err)
	}
	phyPlan := plan.(base.PhysicalPlan)
	_, err = core.GetPlanCost(phyPlan, property.RootTaskType, optimizetrace.NewDefaultPlanCostOption().WithCostFlag(costusage.CostFlagRecalculate))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = core.GetPlanCost(phyPlan, property.RootTaskType, optimizetrace.NewDefaultPlanCostOption().WithCostFlag(costusage.CostFlagRecalculate))
	}
}

func TestTableScanCostWithForce(t *testing.T) {
	store, dom := realtikvtest.CreateMockStoreAndDomainAndSetup(t)
	defer func() {
		tk := testkit.NewTestKit(t, store)
		tk.MustExec("use test")
		tk.MustExec("drop table if exists t")
		dom.StatsHandle().Clear()
	}()

	tk := testkit.NewTestKit(t, store)
	tk.MustExec("use test")
	tk.MustExec("drop table if exists t")
	tk.MustExec("create table t(a int, b int, primary key (a))")

	// Insert some data
	tk.MustExec("insert into t values (1,1),(2,2),(3,3),(4,4),(5,5)")

	// Analyze table to update statistics
	tk.MustExec("analyze table t")

	// Test TableFullScan with and without FORCE INDEX
	rs := tk.MustQuery("explain analyze format=verbose select * from t").Rows()
	planCost1 := rs[0][2].(string)
	rs = tk.MustQuery("explain analyze format=verbose select * from t force index(PRIMARY)").Rows()
	planCost2 := rs[0][2].(string)

	// Query with FORCE should be more expensive than query without
	require.Less(t, planCost1, planCost2)

	// Test TableRangeScan with and without FORCE INDEX
	rs = tk.MustQuery("explain analyze format=verbose select * from t where a > 1").Rows()
	planCost1 = rs[0][2].(string)
	rs = tk.MustQuery("explain analyze format=verbose select * from t force index(PRIMARY) where a > 1").Rows()
	planCost2 = rs[0][2].(string)

	// Query costs should be equal since FORCE cost penalty does not apply to range scan
	require.Equal(t, planCost1, planCost2)
}
