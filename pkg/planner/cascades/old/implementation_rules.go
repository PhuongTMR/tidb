// Copyright 2018 PingCAP, Inc.
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

package old

import (
	"math"

	"github.com/pingcap/tidb/pkg/expression"
	"github.com/pingcap/tidb/pkg/planner/cascades/pattern"
	plannercore "github.com/pingcap/tidb/pkg/planner/core"
	"github.com/pingcap/tidb/pkg/planner/core/operator/logicalop"
	"github.com/pingcap/tidb/pkg/planner/core/operator/physicalop"
	impl "github.com/pingcap/tidb/pkg/planner/implementation"
	"github.com/pingcap/tidb/pkg/planner/memo"
	"github.com/pingcap/tidb/pkg/planner/property"
	"github.com/pingcap/tidb/pkg/util/dbterror/plannererrors"
)

// ImplementationRule defines the interface for implementation rules.
type ImplementationRule interface {
	// Match checks if current GroupExpr matches this rule under required physical property.
	Match(expr *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool)
	// OnImplement generates physical plan using this rule for current GroupExpr. Note that
	// childrenReqProps of generated physical plan should be set correspondingly in this function.
	OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error)
}

var defaultImplementationMap = map[pattern.Operand][]ImplementationRule{
	pattern.OperandTableDual: {
		&ImplTableDual{},
	},
	pattern.OperandMemTableScan: {
		&ImplMemTableScan{},
	},
	pattern.OperandProjection: {
		&ImplProjection{},
	},
	pattern.OperandTableScan: {
		&ImplTableScan{},
	},
	pattern.OperandIndexScan: {
		&ImplIndexScan{},
	},
	pattern.OperandTiKVSingleGather: {
		&ImplTiKVSingleReadGather{},
	},
	pattern.OperandShow: {
		&ImplShow{},
	},
	pattern.OperandSelection: {
		&ImplSelection{},
	},
	pattern.OperandSort: {
		&ImplSort{},
	},
	pattern.OperandAggregation: {
		&ImplHashAgg{},
	},
	pattern.OperandLimit: {
		&ImplLimit{},
	},
	pattern.OperandTopN: {
		&ImplTopN{},
		&ImplTopNAsLimit{},
	},
	pattern.OperandJoin: {
		&ImplHashJoinBuildLeft{},
		&ImplHashJoinBuildRight{},
		&ImplMergeJoin{},
	},
	pattern.OperandUnionAll: {
		&ImplUnionAll{},
	},
	pattern.OperandApply: {
		&ImplApply{},
	},
	pattern.OperandMaxOneRow: {
		&ImplMaxOneRow{},
	},
	pattern.OperandWindow: {
		&ImplWindow{},
	},
}

// ImplTableDual implements LogicalTableDual as PhysicalTableDual.
type ImplTableDual struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplTableDual) Match(_ *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	return prop.IsSortItemEmpty()
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplTableDual) OnImplement(expr *memo.GroupExpr, _ *property.PhysicalProperty) ([]memo.Implementation, error) {
	logicProp := expr.Group.Prop
	logicDual := expr.ExprNode.(*logicalop.LogicalTableDual)
	dual := physicalop.PhysicalTableDual{RowCount: logicDual.RowCount}.Init(logicDual.SCtx(), logicProp.Stats, logicDual.QueryBlockOffset())
	dual.SetSchema(logicProp.Schema)
	return []memo.Implementation{impl.NewTableDualImpl(dual)}, nil
}

// ImplMemTableScan implements LogicalMemTable as PhysicalMemTable.
type ImplMemTableScan struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplMemTableScan) Match(_ *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	return prop.IsSortItemEmpty()
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplMemTableScan) OnImplement(
	expr *memo.GroupExpr,
	reqProp *property.PhysicalProperty,
) ([]memo.Implementation, error) {
	logic := expr.ExprNode.(*logicalop.LogicalMemTable)
	logicProp := expr.Group.Prop
	physical := physicalop.PhysicalMemTable{
		DBName:    logic.DBName,
		Table:     logic.TableInfo,
		Columns:   logic.TableInfo.Columns,
		Extractor: logic.Extractor,
	}.Init(logic.SCtx(), logicProp.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt), logic.QueryBlockOffset())
	physical.SetSchema(logicProp.Schema)
	return []memo.Implementation{impl.NewMemTableScanImpl(physical)}, nil
}

// ImplProjection implements LogicalProjection as PhysicalProjection.
type ImplProjection struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplProjection) Match(_ *memo.GroupExpr, _ *property.PhysicalProperty) (matched bool) {
	return true
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplProjection) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	logicProp := expr.Group.Prop
	logicProj := expr.ExprNode.(*logicalop.LogicalProjection)
	childProp, ok := logicProj.TryToGetChildProp(reqProp)
	if !ok {
		return nil, nil
	}
	proj := physicalop.PhysicalProjection{
		Exprs:            logicProj.Exprs,
		CalculateNoDelay: logicProj.CalculateNoDelay,
	}.Init(logicProj.SCtx(), logicProp.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt), logicProj.QueryBlockOffset(), childProp)
	proj.SetSchema(logicProp.Schema)
	return []memo.Implementation{impl.NewProjectionImpl(proj)}, nil
}

// ImplTiKVSingleReadGather implements TiKVSingleGather
// as PhysicalTableReader or PhysicalIndexReader.
type ImplTiKVSingleReadGather struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplTiKVSingleReadGather) Match(_ *memo.GroupExpr, _ *property.PhysicalProperty) (matched bool) {
	return true
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplTiKVSingleReadGather) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	logicProp := expr.Group.Prop
	sg := expr.ExprNode.(*logicalop.TiKVSingleGather)
	if sg.IsIndexGather {
		reader := plannercore.GetPhysicalIndexReader(sg, logicProp.Schema, logicProp.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt), reqProp)
		return []memo.Implementation{impl.NewIndexReaderImpl(reader, sg.Source)}, nil
	}
	reader := plannercore.GetPhysicalTableReader(sg, logicProp.Schema, logicProp.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt), reqProp)
	return []memo.Implementation{impl.NewTableReaderImpl(reader, sg.Source)}, nil
}

// ImplTableScan implements TableScan as PhysicalTableScan.
type ImplTableScan struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplTableScan) Match(expr *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	ts := expr.ExprNode.(*logicalop.LogicalTableScan)
	return prop.IsSortItemEmpty() || (len(prop.SortItems) == 1 && ts.HandleCols != nil && prop.SortItems[0].Col.EqualColumn(ts.HandleCols.GetCol(0)))
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplTableScan) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	logicProp := expr.Group.Prop
	logicalScan := expr.ExprNode.(*logicalop.LogicalTableScan)
	ts := plannercore.GetPhysicalScan4LogicalTableScan(logicalScan, logicProp.Schema, logicProp.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt))
	if !reqProp.IsSortItemEmpty() {
		ts.KeepOrder = true
		ts.Desc = reqProp.SortItems[0].Desc
	}
	tblCols, tblColHists := logicalScan.Source.TblCols, logicalScan.Source.TblColHists
	return []memo.Implementation{impl.NewTableScanImpl(ts, tblCols, tblColHists)}, nil
}

// ImplIndexScan implements IndexScan as PhysicalIndexScan.
type ImplIndexScan struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplIndexScan) Match(expr *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	is := expr.ExprNode.(*logicalop.LogicalIndexScan)
	return is.MatchIndexProp(prop)
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplIndexScan) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	logicalScan := expr.ExprNode.(*logicalop.LogicalIndexScan)
	is := plannercore.GetPhysicalIndexScan4LogicalIndexScan(logicalScan, expr.Group.Prop.Schema, expr.Group.Prop.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt))
	if !reqProp.IsSortItemEmpty() {
		is.KeepOrder = true
		if reqProp.SortItems[0].Desc {
			is.Desc = true
		}
	}
	return []memo.Implementation{impl.NewIndexScanImpl(is, logicalScan.Source.TblColHists)}, nil
}

// ImplShow is the implementation rule which implements LogicalShow to
// PhysicalShow.
type ImplShow struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplShow) Match(_ *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	return prop.IsSortItemEmpty()
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplShow) OnImplement(expr *memo.GroupExpr, _ *property.PhysicalProperty) ([]memo.Implementation, error) {
	logicProp := expr.Group.Prop
	show := expr.ExprNode.(*logicalop.LogicalShow)
	// TODO(zz-jason): unifying LogicalShow and PhysicalShow to a single
	// struct. So that we don't need to create a new PhysicalShow object, which
	// can help us to reduce the gc pressure of golang runtime and improve the
	// overall performance.
	showPhys := plannercore.PhysicalShow{
		ShowContents: show.ShowContents,
		Extractor:    show.Extractor,
	}.Init(show.SCtx())
	showPhys.SetSchema(logicProp.Schema)
	return []memo.Implementation{impl.NewShowImpl(showPhys)}, nil
}

// ImplSelection is the implementation rule which implements LogicalSelection
// to PhysicalSelection.
type ImplSelection struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplSelection) Match(_ *memo.GroupExpr, _ *property.PhysicalProperty) (matched bool) {
	return true
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplSelection) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	logicalSel := expr.ExprNode.(*logicalop.LogicalSelection)
	physicalSel := physicalop.PhysicalSelection{
		Conditions: logicalSel.Conditions,
	}.Init(logicalSel.SCtx(), expr.Group.Prop.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt), logicalSel.QueryBlockOffset(), reqProp.CloneEssentialFields())
	switch expr.Group.EngineType {
	case pattern.EngineTiDB:
		return []memo.Implementation{impl.NewTiDBSelectionImpl(physicalSel)}, nil
	case pattern.EngineTiKV:
		return []memo.Implementation{impl.NewTiKVSelectionImpl(physicalSel)}, nil
	default:
		return nil, plannererrors.ErrInternal.GenWithStack("Unsupported EngineType '%s' for Selection.", expr.Group.EngineType.String())
	}
}

// ImplSort is the implementation rule which implements LogicalSort
// to PhysicalSort or NominalSort.
type ImplSort struct {
}

// Match implements ImplementationRule match interface.
func (*ImplSort) Match(expr *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	ls := expr.ExprNode.(*logicalop.LogicalSort)
	return plannercore.MatchItems(prop, ls.ByItems)
}

// OnImplement implements ImplementationRule OnImplement interface.
// If all of the sort items are columns, generate a NominalSort, otherwise
// generate a PhysicalSort.
func (*ImplSort) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	ls := expr.ExprNode.(*logicalop.LogicalSort)
	if newProp, canUseNominal := plannercore.GetPropByOrderByItems(ls.ByItems); canUseNominal {
		newProp.ExpectedCnt = reqProp.ExpectedCnt
		ns := physicalop.NominalSort{}.Init(
			ls.SCtx(), expr.Group.Prop.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt), ls.QueryBlockOffset(), newProp)
		return []memo.Implementation{impl.NewNominalSortImpl(ns)}, nil
	}
	ps := physicalop.PhysicalSort{ByItems: ls.ByItems}.Init(
		ls.SCtx(),
		expr.Group.Prop.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt),
		ls.QueryBlockOffset(),
		&property.PhysicalProperty{ExpectedCnt: math.MaxFloat64},
	)
	return []memo.Implementation{impl.NewSortImpl(ps)}, nil
}

// ImplHashAgg is the implementation rule which implements LogicalAggregation
// to PhysicalHashAgg.
type ImplHashAgg struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplHashAgg) Match(_ *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	// TODO: deal with the hints when we have implemented StreamAgg.
	return prop.IsSortItemEmpty()
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplHashAgg) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	la := expr.ExprNode.(*logicalop.LogicalAggregation)
	hashAgg := plannercore.NewPhysicalHashAgg(
		la,
		expr.Group.Prop.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt),
		&property.PhysicalProperty{ExpectedCnt: math.MaxFloat64},
	)
	hashAgg.SetSchema(expr.Group.Prop.Schema.Clone())
	switch expr.Group.EngineType {
	case pattern.EngineTiDB:
		return []memo.Implementation{impl.NewTiDBHashAggImpl(hashAgg)}, nil
	case pattern.EngineTiKV:
		return []memo.Implementation{impl.NewTiKVHashAggImpl(hashAgg)}, nil
	default:
		return nil, plannererrors.ErrInternal.GenWithStack("Unsupported EngineType '%s' for HashAggregation.", expr.Group.EngineType.String())
	}
}

// ImplLimit is the implementation rule which implements LogicalLimit
// to PhysicalLimit.
type ImplLimit struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplLimit) Match(_ *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	return prop.IsSortItemEmpty()
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplLimit) OnImplement(expr *memo.GroupExpr, _ *property.PhysicalProperty) ([]memo.Implementation, error) {
	logicalLimit := expr.ExprNode.(*logicalop.LogicalLimit)
	newProp := &property.PhysicalProperty{ExpectedCnt: float64(logicalLimit.Count + logicalLimit.Offset)}
	physicalLimit := physicalop.PhysicalLimit{
		Offset: logicalLimit.Offset,
		Count:  logicalLimit.Count,
	}.Init(logicalLimit.SCtx(), expr.Group.Prop.Stats, logicalLimit.QueryBlockOffset(), newProp)
	physicalLimit.SetSchema(expr.Group.Prop.Schema.Clone())
	return []memo.Implementation{impl.NewLimitImpl(physicalLimit)}, nil
}

// ImplTopN is the implementation rule which implements LogicalTopN
// to PhysicalTopN.
type ImplTopN struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplTopN) Match(expr *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	topN := expr.ExprNode.(*logicalop.LogicalTopN)
	if expr.Group.EngineType != pattern.EngineTiDB {
		return prop.IsSortItemEmpty()
	}
	return plannercore.MatchItems(prop, topN.ByItems)
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplTopN) OnImplement(expr *memo.GroupExpr, _ *property.PhysicalProperty) ([]memo.Implementation, error) {
	lt := expr.ExprNode.(*logicalop.LogicalTopN)
	resultProp := &property.PhysicalProperty{ExpectedCnt: math.MaxFloat64}
	topN := physicalop.PhysicalTopN{
		ByItems: lt.ByItems,
		Count:   lt.Count,
		Offset:  lt.Offset,
	}.Init(lt.SCtx(), expr.Group.Prop.Stats, lt.QueryBlockOffset(), resultProp)
	switch expr.Group.EngineType {
	case pattern.EngineTiDB:
		return []memo.Implementation{impl.NewTiDBTopNImpl(topN)}, nil
	case pattern.EngineTiKV:
		return []memo.Implementation{impl.NewTiKVTopNImpl(topN)}, nil
	default:
		return nil, plannererrors.ErrInternal.GenWithStack("Unsupported EngineType '%s' for TopN.", expr.Group.EngineType.String())
	}
}

// ImplTopNAsLimit is the implementation rule which implements LogicalTopN
// as PhysicalLimit with required order property.
type ImplTopNAsLimit struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplTopNAsLimit) Match(expr *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	topN := expr.ExprNode.(*logicalop.LogicalTopN)
	_, canUseLimit := plannercore.GetPropByOrderByItems(topN.ByItems)
	return canUseLimit && plannercore.MatchItems(prop, topN.ByItems)
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplTopNAsLimit) OnImplement(expr *memo.GroupExpr, _ *property.PhysicalProperty) ([]memo.Implementation, error) {
	lt := expr.ExprNode.(*logicalop.LogicalTopN)
	newProp := &property.PhysicalProperty{ExpectedCnt: float64(lt.Count + lt.Offset)}
	newProp.SortItems = make([]property.SortItem, len(lt.ByItems))
	for i, item := range lt.ByItems {
		newProp.SortItems[i].Col = item.Expr.(*expression.Column)
		newProp.SortItems[i].Desc = item.Desc
	}
	physicalLimit := physicalop.PhysicalLimit{
		Offset: lt.Offset,
		Count:  lt.Count,
	}.Init(lt.SCtx(), expr.Group.Prop.Stats, lt.QueryBlockOffset(), newProp)
	physicalLimit.SetSchema(expr.Group.Prop.Schema.Clone())
	return []memo.Implementation{impl.NewLimitImpl(physicalLimit)}, nil
}

func getImplForHashJoin(expr *memo.GroupExpr, prop *property.PhysicalProperty, innerIdx int, useOuterToBuild bool) memo.Implementation {
	join := expr.ExprNode.(*logicalop.LogicalJoin)
	chReqProps := make([]*property.PhysicalProperty, 2)
	chReqProps[0] = &property.PhysicalProperty{ExpectedCnt: math.MaxFloat64}
	chReqProps[1] = &property.PhysicalProperty{ExpectedCnt: math.MaxFloat64}
	stats := expr.Group.Prop.Stats
	if prop.ExpectedCnt < stats.RowCount {
		expCntScale := prop.ExpectedCnt / stats.RowCount
		chReqProps[1-innerIdx].ExpectedCnt = expr.Children[1-innerIdx].Prop.Stats.RowCount * expCntScale
	}
	hashJoin := plannercore.NewPhysicalHashJoin(join, innerIdx, useOuterToBuild, stats.ScaleByExpectCnt(prop.ExpectedCnt), chReqProps...)
	hashJoin.SetSchema(expr.Group.Prop.Schema)
	return impl.NewHashJoinImpl(hashJoin)
}

// ImplHashJoinBuildLeft implements LogicalJoin to PhysicalHashJoin which uses the left child to build hash table.
type ImplHashJoinBuildLeft struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplHashJoinBuildLeft) Match(expr *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	switch expr.ExprNode.(*logicalop.LogicalJoin).JoinType {
	case logicalop.InnerJoin, logicalop.LeftOuterJoin, logicalop.RightOuterJoin:
		return prop.IsSortItemEmpty()
	default:
		return false
	}
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplHashJoinBuildLeft) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	join := expr.ExprNode.(*logicalop.LogicalJoin)
	switch join.JoinType {
	case logicalop.InnerJoin:
		return []memo.Implementation{getImplForHashJoin(expr, reqProp, 0, false)}, nil
	case logicalop.LeftOuterJoin:
		return []memo.Implementation{getImplForHashJoin(expr, reqProp, 1, true)}, nil
	case logicalop.RightOuterJoin:
		return []memo.Implementation{getImplForHashJoin(expr, reqProp, 0, false)}, nil
	default:
		return nil, nil
	}
}

// ImplHashJoinBuildRight implements LogicalJoin to PhysicalHashJoin which uses the right child to build hash table.
type ImplHashJoinBuildRight struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplHashJoinBuildRight) Match(_ *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	return prop.IsSortItemEmpty()
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplHashJoinBuildRight) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	join := expr.ExprNode.(*logicalop.LogicalJoin)
	switch join.JoinType {
	case logicalop.SemiJoin, logicalop.AntiSemiJoin,
		logicalop.LeftOuterSemiJoin, logicalop.AntiLeftOuterSemiJoin:
		return []memo.Implementation{getImplForHashJoin(expr, reqProp, 1, false)}, nil
	case logicalop.InnerJoin:
		return []memo.Implementation{getImplForHashJoin(expr, reqProp, 1, false)}, nil
	case logicalop.LeftOuterJoin:
		return []memo.Implementation{getImplForHashJoin(expr, reqProp, 1, false)}, nil
	case logicalop.RightOuterJoin:
		return []memo.Implementation{getImplForHashJoin(expr, reqProp, 0, true)}, nil
	}
	return nil, nil
}

// ImplMergeJoin implements LogicalMergeJoin to PhysicalMergeJoin.
type ImplMergeJoin struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplMergeJoin) Match(_ *memo.GroupExpr, _ *property.PhysicalProperty) (matched bool) {
	return true
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplMergeJoin) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	join := expr.ExprNode.(*logicalop.LogicalJoin)
	physicalMergeJoins := physicalop.GetMergeJoin(join, reqProp, expr.Schema(), expr.Group.Prop.Stats, expr.Children[0].Prop.Stats, expr.Children[1].Prop.Stats)
	mergeJoinImpls := make([]memo.Implementation, 0, len(physicalMergeJoins))
	for _, physicalPlan := range physicalMergeJoins {
		physicalMergeJoin := physicalPlan.(*physicalop.PhysicalMergeJoin)
		mergeJoinImpls = append(mergeJoinImpls, impl.NewMergeJoinImpl(physicalMergeJoin))
	}
	return mergeJoinImpls, nil
}

// ImplUnionAll implements LogicalUnionAll to PhysicalUnionAll.
type ImplUnionAll struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplUnionAll) Match(_ *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	return prop.IsSortItemEmpty()
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplUnionAll) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	logicalUnion := expr.ExprNode.(*logicalop.LogicalUnionAll)
	chReqProps := make([]*property.PhysicalProperty, len(expr.Children))
	for i := range expr.Children {
		chReqProps[i] = &property.PhysicalProperty{ExpectedCnt: reqProp.ExpectedCnt}
	}
	physicalUnion := physicalop.PhysicalUnionAll{}.Init(
		logicalUnion.SCtx(),
		expr.Group.Prop.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt),
		logicalUnion.QueryBlockOffset(),
		chReqProps...,
	)
	physicalUnion.SetSchema(expr.Group.Prop.Schema)
	return []memo.Implementation{impl.NewUnionAllImpl(physicalUnion)}, nil
}

// ImplApply implements LogicalApply to PhysicalApply
type ImplApply struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplApply) Match(expr *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	return prop.AllColsFromSchema(expr.Children[0].Prop.Schema)
}

// OnImplement implements ImplementationRule OnImplement interface
func (*ImplApply) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	la := expr.ExprNode.(*logicalop.LogicalApply)
	join := plannercore.GetHashJoin(nil, la, reqProp)
	physicalApply := plannercore.PhysicalApply{
		PhysicalHashJoin: *join,
		OuterSchema:      la.CorCols,
	}.Init(
		la.SCtx(),
		expr.Group.Prop.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt),
		la.QueryBlockOffset(),
		&property.PhysicalProperty{ExpectedCnt: math.MaxFloat64, SortItems: reqProp.SortItems},
		&property.PhysicalProperty{ExpectedCnt: math.MaxFloat64})
	physicalApply.SetSchema(expr.Group.Prop.Schema)
	return []memo.Implementation{impl.NewApplyImpl(physicalApply)}, nil
}

// ImplMaxOneRow implements LogicalMaxOneRow to PhysicalMaxOneRow.
type ImplMaxOneRow struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplMaxOneRow) Match(_ *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	return prop.IsSortItemEmpty()
}

// OnImplement implements ImplementationRule OnImplement interface
func (*ImplMaxOneRow) OnImplement(expr *memo.GroupExpr, _ *property.PhysicalProperty) ([]memo.Implementation, error) {
	mor := expr.ExprNode.(*logicalop.LogicalMaxOneRow)
	physicalMaxOneRow := physicalop.PhysicalMaxOneRow{}.Init(
		mor.SCtx(),
		expr.Group.Prop.Stats,
		mor.QueryBlockOffset(),
		&property.PhysicalProperty{ExpectedCnt: 2})
	return []memo.Implementation{impl.NewMaxOneRowImpl(physicalMaxOneRow)}, nil
}

// ImplWindow implements LogicalWindow to PhysicalWindow.
type ImplWindow struct {
}

// Match implements ImplementationRule Match interface.
func (*ImplWindow) Match(expr *memo.GroupExpr, prop *property.PhysicalProperty) (matched bool) {
	lw := expr.ExprNode.(*logicalop.LogicalWindow)
	var byItems []property.SortItem
	byItems = append(byItems, lw.PartitionBy...)
	byItems = append(byItems, lw.OrderBy...)
	childProperty := &property.PhysicalProperty{ExpectedCnt: math.MaxFloat64, SortItems: byItems}
	return prop.IsPrefix(childProperty)
}

// OnImplement implements ImplementationRule OnImplement interface.
func (*ImplWindow) OnImplement(expr *memo.GroupExpr, reqProp *property.PhysicalProperty) ([]memo.Implementation, error) {
	lw := expr.ExprNode.(*logicalop.LogicalWindow)
	var byItems []property.SortItem
	byItems = append(byItems, lw.PartitionBy...)
	byItems = append(byItems, lw.OrderBy...)
	physicalWindow := physicalop.PhysicalWindow{
		WindowFuncDescs: lw.WindowFuncDescs,
		PartitionBy:     lw.PartitionBy,
		OrderBy:         lw.OrderBy,
		Frame:           lw.Frame,
	}.Init(
		lw.SCtx(),
		expr.Group.Prop.Stats.ScaleByExpectCnt(reqProp.ExpectedCnt),
		lw.QueryBlockOffset(),
		&property.PhysicalProperty{ExpectedCnt: math.MaxFloat64, SortItems: byItems},
	)
	physicalWindow.SetSchema(expr.Group.Prop.Schema)
	return []memo.Implementation{impl.NewWindowImpl(physicalWindow)}, nil
}
