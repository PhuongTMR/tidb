// Copyright 2016 PingCAP, Inc.
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
	"context"
	gjson "encoding/json"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/docker/go-units"
	"github.com/pingcap/errors"
	"github.com/pingcap/failpoint"
	"github.com/pingcap/tidb/br/pkg/utils"
	"github.com/pingcap/tidb/pkg/bindinfo"
	"github.com/pingcap/tidb/pkg/ddl"
	"github.com/pingcap/tidb/pkg/disttask/framework/proto"
	fstorage "github.com/pingcap/tidb/pkg/disttask/framework/storage"
	"github.com/pingcap/tidb/pkg/disttask/importinto"
	"github.com/pingcap/tidb/pkg/domain"
	"github.com/pingcap/tidb/pkg/domain/infosync"
	"github.com/pingcap/tidb/pkg/executor/importer"
	"github.com/pingcap/tidb/pkg/executor/internal/exec"
	"github.com/pingcap/tidb/pkg/expression"
	"github.com/pingcap/tidb/pkg/infoschema"
	"github.com/pingcap/tidb/pkg/kv"
	"github.com/pingcap/tidb/pkg/meta/autoid"
	"github.com/pingcap/tidb/pkg/meta/metadef"
	"github.com/pingcap/tidb/pkg/meta/model"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/auth"
	"github.com/pingcap/tidb/pkg/parser/charset"
	parserformat "github.com/pingcap/tidb/pkg/parser/format"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/parser/terror"
	"github.com/pingcap/tidb/pkg/parser/tidb"
	field_types "github.com/pingcap/tidb/pkg/parser/types"
	plannercore "github.com/pingcap/tidb/pkg/planner/core"
	"github.com/pingcap/tidb/pkg/planner/core/base"
	"github.com/pingcap/tidb/pkg/planner/core/resolve"
	"github.com/pingcap/tidb/pkg/plugin"
	"github.com/pingcap/tidb/pkg/privilege"
	"github.com/pingcap/tidb/pkg/privilege/privileges"
	"github.com/pingcap/tidb/pkg/sessionctx"
	"github.com/pingcap/tidb/pkg/sessionctx/sessionstates"
	"github.com/pingcap/tidb/pkg/sessionctx/vardef"
	"github.com/pingcap/tidb/pkg/sessionctx/variable"
	"github.com/pingcap/tidb/pkg/sessiontxn"
	"github.com/pingcap/tidb/pkg/store/helper"
	"github.com/pingcap/tidb/pkg/table"
	"github.com/pingcap/tidb/pkg/table/tables"
	"github.com/pingcap/tidb/pkg/tablecodec"
	"github.com/pingcap/tidb/pkg/types"
	"github.com/pingcap/tidb/pkg/util/chunk"
	"github.com/pingcap/tidb/pkg/util/codec"
	"github.com/pingcap/tidb/pkg/util/collate"
	contextutil "github.com/pingcap/tidb/pkg/util/context"
	"github.com/pingcap/tidb/pkg/util/dbterror/exeerrors"
	"github.com/pingcap/tidb/pkg/util/dbterror/plannererrors"
	"github.com/pingcap/tidb/pkg/util/format"
	"github.com/pingcap/tidb/pkg/util/hack"
	"github.com/pingcap/tidb/pkg/util/hint"
	"github.com/pingcap/tidb/pkg/util/logutil"
	"github.com/pingcap/tidb/pkg/util/memory"
	"github.com/pingcap/tidb/pkg/util/sem"
	"github.com/pingcap/tidb/pkg/util/set"
	"github.com/pingcap/tidb/pkg/util/sqlexec"
	"github.com/pingcap/tidb/pkg/util/stringutil"
	"github.com/tikv/pd/client/errs"
	pdHttp "github.com/tikv/pd/client/http"
	"go.uber.org/zap"
)

// ShowExec represents a show executor.
type ShowExec struct {
	exec.BaseExecutor

	Tp                ast.ShowStmtType // Databases/Tables/Columns/....
	DBName            ast.CIStr
	Table             *resolve.TableNameW  // Used for showing columns.
	Partition         ast.CIStr            // Used for showing partition
	Column            *ast.ColumnName      // Used for `desc table column`.
	IndexName         ast.CIStr            // Used for show table regions.
	ResourceGroupName ast.CIStr            // Used for showing resource group
	Flag              int                  // Some flag parsed from sql, such as FULL.
	Roles             []*auth.RoleIdentity // Used for show grants.
	User              *auth.UserIdentity   // Used by show grants, show create user.
	Extractor         base.ShowPredicateExtractor

	is infoschema.InfoSchema

	CountWarningsOrErrors bool // Used for showing count(*) warnings | errors

	result *chunk.Chunk
	cursor int

	Full        bool
	IfNotExists bool // Used for `show create database if not exists`
	GlobalScope bool // GlobalScope is used by show variables
	Extended    bool // Used for `show extended columns from ...`

	ImportJobID       *int64
	DistributionJobID *int64
}

type showTableRegionRowItem struct {
	regionMeta
	schedulingConstraints string
	schedulingState       string
}

// Next implements the Executor Next interface.
func (e *ShowExec) Next(ctx context.Context, req *chunk.Chunk) error {
	req.GrowAndReset(e.MaxChunkSize())
	if e.result == nil {
		e.result = exec.NewFirstChunk(e)
		err := e.fetchAll(ctx)
		if err != nil {
			return errors.Trace(err)
		}
		iter := chunk.NewIterator4Chunk(e.result)
		for colIdx := range e.Schema().Len() {
			retType := e.Schema().Columns[colIdx].RetType
			if !types.IsTypeVarchar(retType.GetType()) {
				continue
			}
			for row := iter.Begin(); row != iter.End(); row = iter.Next() {
				if valLen := len(row.GetString(colIdx)); retType.GetFlen() < valLen {
					retType.SetFlen(valLen)
				}
			}
		}
	}
	if e.cursor >= e.result.NumRows() {
		return nil
	}
	numCurBatch := min(req.Capacity(), e.result.NumRows()-e.cursor)
	req.Append(e.result, e.cursor, e.cursor+numCurBatch)
	e.cursor += numCurBatch
	return nil
}

func (e *ShowExec) fetchAll(ctx context.Context) error {
	// Temporary disables select limit to avoid miss the result.
	// Because some of below fetch show result stmt functions will generate
	// a SQL stmt and then execute the new SQL stmt to do the fetch result task
	// for the up-level show stmt.
	// Here, the problem is the new SQL stmt will be influenced by SelectLimit value
	// and return a limited result set back to up level show stmt. This, in fact, may
	// cause a filter effect on result set that may exclude the qualified record outside of
	// result set.
	// Finally, when above result set, may not include qualified record, is returned to up
	// level show stmt's selection, which really applies the filter operation on returned
	// result set, it may return empty result to client.
	oldSelectLimit := e.Ctx().GetSessionVars().SelectLimit
	e.Ctx().GetSessionVars().SelectLimit = math.MaxUint64
	defer func() {
		// Restore session Var SelectLimit value.
		e.Ctx().GetSessionVars().SelectLimit = oldSelectLimit
	}()

	switch e.Tp {
	case ast.ShowCharset:
		return e.fetchShowCharset()
	case ast.ShowCollation:
		return e.fetchShowCollation()
	case ast.ShowColumns:
		return e.fetchShowColumns(ctx)
	case ast.ShowConfig:
		return e.fetchShowClusterConfigs()
	case ast.ShowCreateTable:
		return e.fetchShowCreateTable()
	case ast.ShowCreateSequence:
		return e.fetchShowCreateSequence()
	case ast.ShowCreateUser:
		return e.fetchShowCreateUser(ctx)
	case ast.ShowCreateView:
		return e.fetchShowCreateView()
	case ast.ShowCreateDatabase:
		return e.fetchShowCreateDatabase()
	case ast.ShowCreatePlacementPolicy:
		return e.fetchShowCreatePlacementPolicy()
	case ast.ShowCreateResourceGroup:
		return e.fetchShowCreateResourceGroup()
	case ast.ShowDatabases:
		return e.fetchShowDatabases()
	case ast.ShowEngines:
		return e.fetchShowEngines(ctx)
	case ast.ShowGrants:
		return e.fetchShowGrants(ctx)
	case ast.ShowIndex:
		return e.fetchShowIndex()
	case ast.ShowProcedureStatus:
		return e.fetchShowProcedureStatus()
	case ast.ShowStatus:
		return e.fetchShowStatus()
	case ast.ShowTables:
		return e.fetchShowTables(ctx)
	case ast.ShowOpenTables:
		return e.fetchShowOpenTables()
	case ast.ShowTableStatus:
		return e.fetchShowTableStatus(ctx)
	case ast.ShowTriggers:
		return e.fetchShowTriggers()
	case ast.ShowVariables:
		return e.fetchShowVariables(ctx)
	case ast.ShowWarnings:
		return e.fetchShowWarnings(false)
	case ast.ShowErrors:
		return e.fetchShowWarnings(true)
	case ast.ShowProcessList:
		return e.fetchShowProcessList()
	case ast.ShowEvents:
		// empty result
	case ast.ShowStatsExtended:
		return e.fetchShowStatsExtended(ctx)
	case ast.ShowStatsMeta:
		return e.fetchShowStatsMeta(ctx)
	case ast.ShowStatsHistograms:
		return e.fetchShowStatsHistogram(ctx)
	case ast.ShowStatsBuckets:
		return e.fetchShowStatsBuckets(ctx)
	case ast.ShowStatsTopN:
		return e.fetchShowStatsTopN(ctx)
	case ast.ShowStatsHealthy:
		e.fetchShowStatsHealthy(ctx)
		return nil
	case ast.ShowStatsLocked:
		return e.fetchShowStatsLocked(ctx)
	case ast.ShowHistogramsInFlight:
		e.fetchShowHistogramsInFlight()
		return nil
	case ast.ShowColumnStatsUsage:
		return e.fetchShowColumnStatsUsage(ctx)
	case ast.ShowPlugins:
		return e.fetchShowPlugins()
	case ast.ShowProfiles:
		// empty result
	case ast.ShowMasterStatus, ast.ShowBinlogStatus:
		return e.fetchShowMasterStatus()
	case ast.ShowPrivileges:
		return e.fetchShowPrivileges()
	case ast.ShowBindings:
		return e.fetchShowBind()
	case ast.ShowBindingCacheStatus:
		return e.fetchShowBindingCacheStatus(ctx)
	case ast.ShowAnalyzeStatus:
		return e.fetchShowAnalyzeStatus(ctx)
	case ast.ShowRegions:
		return e.fetchShowTableRegions(ctx)
	case ast.ShowDistributions:
		return e.fetchShowDistributions(ctx)
	case ast.ShowBuiltins:
		return e.fetchShowBuiltins()
	case ast.ShowBackups:
		return e.fetchShowBRIE(ast.BRIEKindBackup)
	case ast.ShowRestores:
		return e.fetchShowBRIE(ast.BRIEKindRestore)
	case ast.ShowPlacementLabels:
		return e.fetchShowPlacementLabels(ctx)
	case ast.ShowPlacement:
		return e.fetchShowPlacement(ctx)
	case ast.ShowPlacementForDatabase:
		return e.fetchShowPlacementForDB(ctx)
	case ast.ShowPlacementForTable:
		return e.fetchShowPlacementForTable(ctx)
	case ast.ShowPlacementForPartition:
		return e.fetchShowPlacementForPartition(ctx)
	case ast.ShowSessionStates:
		return e.fetchShowSessionStates(ctx)
	case ast.ShowImportJobs:
		return e.fetchShowImportJobs(ctx)
	case ast.ShowDistributionJobs:
		return e.fetchShowDistributionJobs(ctx)
	}
	return nil
}

// visibleChecker checks if a stmt is visible for a certain user.
type visibleChecker struct {
	defaultDB string
	ctx       sessionctx.Context
	is        infoschema.InfoSchema
	manager   privilege.Manager
	ok        bool
}

func (v *visibleChecker) Enter(in ast.Node) (out ast.Node, skipChildren bool) {
	if x, ok := in.(*ast.TableName); ok {
		schema := x.Schema.L
		if schema == "" {
			schema = v.defaultDB
		}
		if !v.is.TableExists(ast.NewCIStr(schema), x.Name) {
			return in, true
		}
		activeRoles := v.ctx.GetSessionVars().ActiveRoles
		if v.manager != nil && !v.manager.RequestVerification(activeRoles, schema, x.Name.L, "", mysql.SelectPriv) {
			v.ok = false
		}
		return in, true
	}
	return in, false
}

func (*visibleChecker) Leave(in ast.Node) (out ast.Node, ok bool) {
	return in, true
}

func (e *ShowExec) fetchShowBind() error {
	var bindings []*bindinfo.Binding
	if !e.GlobalScope {
		handle := e.Ctx().Value(bindinfo.SessionBindInfoKeyType).(bindinfo.SessionBindingHandle)
		bindings = handle.GetAllSessionBindings()
	} else {
		bindings = domain.GetDomain(e.Ctx()).BindingHandle().GetAllBindings()
	}
	// Remove the invalid bindings.
	parser := parser.New()
	// For the different origin_sql, sort the bindings according to their max update time.
	sort.Slice(bindings, func(i int, j int) bool {
		cmpResult := bindings[i].UpdateTime.Compare(bindings[j].UpdateTime)
		if cmpResult == 0 {
			// Because the create time must be different, the result of sorting is stable.
			cmpResult = bindings[i].CreateTime.Compare(bindings[j].CreateTime)
		}
		return cmpResult > 0
	})
	for _, hint := range bindings {
		stmt, err := parser.ParseOneStmt(hint.BindSQL, hint.Charset, hint.Collation)
		if err != nil {
			return err
		}
		checker := visibleChecker{
			defaultDB: hint.Db,
			ctx:       e.Ctx(),
			is:        e.is,
			manager:   privilege.GetPrivilegeManager(e.Ctx()),
			ok:        true,
		}
		stmt.Accept(&checker)
		if !checker.ok {
			continue
		}
		e.appendRow([]any{
			hint.OriginalSQL,
			hint.BindSQL,
			hint.Db,
			hint.Status,
			hint.CreateTime,
			hint.UpdateTime,
			hint.Charset,
			hint.Collation,
			hint.Source,
			hint.SQLDigest,
			hint.PlanDigest,
		})
	}
	return nil
}

func (e *ShowExec) fetchShowBindingCacheStatus(ctx context.Context) error {
	exec := e.Ctx().GetRestrictedSQLExecutor()
	ctx = kv.WithInternalSourceType(ctx, kv.InternalTxnBindInfo)

	rows, _, err := exec.ExecRestrictedSQL(ctx, nil,
		fmt.Sprintf("SELECT count(*) FROM mysql.bind_info where status = '%s' or status = '%s';",
			bindinfo.StatusEnabled, bindinfo.StatusUsing))
	if err != nil {
		return errors.Trace(err)
	}

	handle := domain.GetDomain(e.Ctx()).BindingHandle()

	bindings := handle.GetAllBindings()
	numBindings := 0
	for _, binding := range bindings {
		if binding.IsBindingEnabled() {
			numBindings++
		}
	}

	memUsage := handle.GetMemUsage()
	memCapacity := handle.GetMemCapacity()
	e.appendRow([]any{
		numBindings,
		rows[0].GetInt64(0),
		memory.FormatBytes(memUsage),
		memory.FormatBytes(memCapacity),
	})
	return nil
}

func (e *ShowExec) fetchShowEngines(ctx context.Context) error {
	ctx = kv.WithInternalSourceType(ctx, kv.InternalTxnMeta)
	exec := e.Ctx().GetRestrictedSQLExecutor()

	rows, _, err := exec.ExecRestrictedSQL(ctx, nil, `SELECT * FROM information_schema.engines`)
	if err != nil {
		return errors.Trace(err)
	}

	e.result.AppendRows(rows)
	return nil
}

// moveInfoSchemaToFront moves information_schema to the first, and the others are sorted in the origin ascending order.
func moveInfoSchemaToFront(dbs []string) {
	if len(dbs) > 0 && strings.EqualFold(dbs[0], metadef.InformationSchemaName.O) {
		return
	}

	i := sort.SearchStrings(dbs, metadef.InformationSchemaName.O)
	if i < len(dbs) && strings.EqualFold(dbs[i], metadef.InformationSchemaName.O) {
		copy(dbs[1:i+1], dbs[0:i])
		dbs[0] = metadef.InformationSchemaName.O
	}
}

func (e *ShowExec) fetchShowDatabases() error {
	dbs := infoschema.AllSchemaNames(e.is)
	checker := privilege.GetPrivilegeManager(e.Ctx())
	slices.Sort(dbs)
	var (
		fieldPatternsLike collate.WildcardPattern
		fieldFilter       string
	)

	if e.Extractor != nil {
		fieldFilter = e.Extractor.Field()
		fieldPatternsLike = e.Extractor.FieldPatternLike()
	}
	// let information_schema be the first database
	moveInfoSchemaToFront(dbs)
	for _, d := range dbs {
		if checker != nil && !checker.DBIsVisible(e.Ctx().GetSessionVars().ActiveRoles, d) {
			continue
		} else if fieldFilter != "" && strings.ToLower(d) != fieldFilter {
			continue
		} else if fieldPatternsLike != nil && !fieldPatternsLike.DoMatch(strings.ToLower(d)) {
			continue
		}
		e.appendRow([]any{
			d,
		})
	}
	return nil
}

func (e *ShowExec) fetchShowProcessList() error {
	sm := e.Ctx().GetSessionManager()
	if sm == nil {
		return nil
	}

	loginUser, activeRoles := e.Ctx().GetSessionVars().User, e.Ctx().GetSessionVars().ActiveRoles
	var hasProcessPriv bool
	if pm := privilege.GetPrivilegeManager(e.Ctx()); pm != nil {
		if pm.RequestVerification(activeRoles, "", "", "", mysql.ProcessPriv) {
			hasProcessPriv = true
		}
	}

	pl := sm.ShowProcessList()
	for _, pi := range pl {
		// If you have the PROCESS privilege, you can see all threads.
		// Otherwise, you can see only your own threads.
		if !hasProcessPriv && pi.User != loginUser.Username {
			continue
		}
		row := pi.ToRowForShow(e.Full)
		e.appendRow(row)
	}
	return nil
}

func (*ShowExec) fetchShowOpenTables() error {
	// TiDB has no concept like mysql's "table cache" and "open table"
	// For simplicity, we just return an empty result with the same structure as MySQL's SHOW OPEN TABLES
	return nil
}

// showInfo represents the result of `SHOW TABLES`.
type showInfo struct {
	Name ast.CIStr
	// only used for show full tables
	TableType string
}

// getTableType returns the type of the table.
func (e *ShowExec) getTableType(tb *model.TableInfo) string {
	switch {
	case tb.IsView():
		return "VIEW"
	case tb.IsSequence():
		return "SEQUENCE"
	case metadef.IsMemDB(e.DBName.L):
		return "SYSTEM VIEW"
	default:
		return "BASE TABLE"
	}
}

// fetchShowInfoByName fetches the show info for `SHOW <FULL> TABLES like 'xxx'`
func (e *ShowExec) fetchShowInfoByName(ctx context.Context, name string) ([]*showInfo, error) {
	tb, err := e.is.TableByName(ctx, e.DBName, ast.NewCIStr(name))
	if err != nil {
		// do nothing if table not exists
		if infoschema.ErrTableNotExists.Equal(err) {
			return nil, nil
		}
		return nil, errors.Trace(err)
	}
	if tb.Meta().TempTableType == model.TempTableLocal {
		return nil, nil
	}
	return []*showInfo{{Name: tb.Meta().Name, TableType: e.getTableType(tb.Meta())}}, nil
}

// fetchShowSimpleTables fetches the table info for `SHOW TABLE`.
func (e *ShowExec) fetchShowSimpleTables(ctx context.Context) ([]*showInfo, error) {
	tb, err := e.is.SchemaSimpleTableInfos(ctx, e.DBName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	showInfos := make([]*showInfo, 0, len(tb))
	for _, v := range tb {
		// TODO: consider add type info to TableNameInfo
		showInfos = append(showInfos, &showInfo{Name: v.Name})
	}
	return showInfos, nil
}

// fetchShowFullTables fetches the table info for `SHOW FULL TABLES`.
func (e *ShowExec) fetchShowFullTables(ctx context.Context) ([]*showInfo, error) {
	tb, err := e.is.SchemaTableInfos(ctx, e.DBName)
	if err != nil {
		return nil, errors.Trace(err)
	}
	showInfos := make([]*showInfo, 0, len(tb))
	for _, v := range tb {
		showInfos = append(showInfos, &showInfo{Name: v.Name, TableType: e.getTableType(v)})
	}
	return showInfos, nil
}

func (e *ShowExec) fetchShowTables(ctx context.Context) error {
	checker := privilege.GetPrivilegeManager(e.Ctx())
	if checker != nil && e.Ctx().GetSessionVars().User != nil {
		if !checker.DBIsVisible(e.Ctx().GetSessionVars().ActiveRoles, e.DBName.O) {
			return e.dbAccessDenied()
		}
	}
	if !e.is.SchemaExists(e.DBName) {
		return exeerrors.ErrBadDB.GenWithStackByArgs(e.DBName)
	}
	var (
		tableNames = make([]string, 0)
		showInfos  []*showInfo
		err        error
	)
	activeRoles := e.Ctx().GetSessionVars().ActiveRoles
	var (
		tableTypes        = make(map[string]string)
		fieldPatternsLike collate.WildcardPattern
		fieldFilter       string
	)

	if e.Extractor != nil {
		fieldFilter = e.Extractor.Field()
		fieldPatternsLike = e.Extractor.FieldPatternLike()
	}

	if fieldFilter != "" {
		showInfos, err = e.fetchShowInfoByName(ctx, fieldFilter)
	} else if e.Full {
		showInfos, err = e.fetchShowFullTables(ctx)
	} else {
		showInfos, err = e.fetchShowSimpleTables(ctx)
	}
	if err != nil {
		return errors.Trace(err)
	}
	for _, v := range showInfos {
		// Test with mysql.AllPrivMask means any privilege would be OK.
		// TODO: Should consider column privileges, which also make a table visible.
		if checker != nil && !checker.RequestVerification(activeRoles, e.DBName.O, v.Name.O, "", mysql.AllPrivMask&(^mysql.CreateTMPTablePriv)) {
			continue
		} else if fieldFilter != "" && v.Name.L != fieldFilter {
			continue
		} else if fieldPatternsLike != nil && !fieldPatternsLike.DoMatch(v.Name.L) {
			continue
		}
		tableNames = append(tableNames, v.Name.O)
		if e.Full {
			tableTypes[v.Name.O] = v.TableType
		}
	}
	slices.Sort(tableNames)
	for _, v := range tableNames {
		if e.Full {
			e.appendRow([]any{v, tableTypes[v]})
		} else {
			e.appendRow([]any{v})
		}
	}
	return nil
}

func (e *ShowExec) fetchShowTableStatus(ctx context.Context) error {
	checker := privilege.GetPrivilegeManager(e.Ctx())
	if checker != nil && e.Ctx().GetSessionVars().User != nil {
		if !checker.DBIsVisible(e.Ctx().GetSessionVars().ActiveRoles, e.DBName.O) {
			return e.dbAccessDenied()
		}
	}
	if !e.is.SchemaExists(e.DBName) {
		return exeerrors.ErrBadDB.GenWithStackByArgs(e.DBName)
	}

	exec := e.Ctx().GetRestrictedSQLExecutor()
	ctx = kv.WithInternalSourceType(ctx, kv.InternalTxnStats)

	var snapshot uint64
	txn, err := e.Ctx().Txn(false)
	if err != nil {
		return errors.Trace(err)
	}
	if txn.Valid() {
		snapshot = txn.StartTS()
	}
	if e.Ctx().GetSessionVars().SnapshotTS != 0 {
		snapshot = e.Ctx().GetSessionVars().SnapshotTS
	}

	rows, _, err := exec.ExecRestrictedSQL(ctx, []sqlexec.OptionFuncAlias{sqlexec.ExecOptionWithSnapshot(snapshot), sqlexec.ExecOptionUseCurSession},
		`SELECT table_name, engine, version, row_format, table_rows,
		avg_row_length, data_length, max_data_length, index_length,
		data_free, auto_increment, create_time, update_time, check_time,
		table_collation, IFNULL(checksum,''), create_options, table_comment
		FROM information_schema.tables
		WHERE lower(table_schema)=%? ORDER BY table_name`, e.DBName.L)
	if err != nil {
		return errors.Trace(err)
	}
	var (
		fieldPatternsLike collate.WildcardPattern
		fieldFilter       string
	)

	if e.Extractor != nil {
		fieldFilter = e.Extractor.Field()
		fieldPatternsLike = e.Extractor.FieldPatternLike()
	}
	activeRoles := e.Ctx().GetSessionVars().ActiveRoles
	for _, row := range rows {
		tableName := row.GetString(0)
		if checker != nil && !checker.RequestVerification(activeRoles, e.DBName.O, tableName, "", mysql.AllPrivMask) {
			continue
		} else if fieldFilter != "" && strings.ToLower(tableName) != fieldFilter {
			continue
		} else if fieldPatternsLike != nil && !fieldPatternsLike.DoMatch(strings.ToLower(tableName)) {
			continue
		}
		e.result.AppendRow(row)
	}
	return nil
}

func (e *ShowExec) fetchShowColumns(ctx context.Context) error {
	tb, err := e.getTable()
	if err != nil {
		return errors.Trace(err)
	}
	// we will fill the column type information later, so clone a new table here.
	tb, err = table.TableFromMeta(tb.Allocators(e.Ctx().GetTableCtx()), tb.Meta().Clone())
	if err != nil {
		return errors.Trace(err)
	}
	var (
		fieldPatternsLike collate.WildcardPattern
		fieldFilter       string
	)

	if e.Extractor != nil {
		fieldFilter = e.Extractor.Field()
		fieldPatternsLike = e.Extractor.FieldPatternLike()
	}

	checker := privilege.GetPrivilegeManager(e.Ctx())
	activeRoles := e.Ctx().GetSessionVars().ActiveRoles
	if checker != nil && e.Ctx().GetSessionVars().User != nil && !checker.RequestVerification(activeRoles, e.DBName.O, tb.Meta().Name.O, "", mysql.InsertPriv|mysql.SelectPriv|mysql.UpdatePriv|mysql.ReferencesPriv) {
		return e.tableAccessDenied("SELECT", tb.Meta().Name.O)
	}

	var cols []*table.Column
	// The optional EXTENDED keyword causes the output to include information about hidden columns that MySQL uses internally and are not accessible by users.
	// See https://dev.mysql.com/doc/refman/8.0/en/show-columns.html
	if e.Extended {
		cols = tb.Cols()
	} else {
		cols = tb.VisibleCols()
	}
	if err := tryFillViewColumnType(ctx, e.Ctx(), e.is, e.DBName, tb.Meta()); err != nil {
		return err
	}
	for _, col := range cols {
		if fieldFilter != "" && col.Name.L != fieldFilter {
			continue
		} else if fieldPatternsLike != nil && !fieldPatternsLike.DoMatch(col.Name.L) {
			continue
		}
		desc := table.NewColDesc(col)
		var columnDefault any
		if desc.DefaultValue != nil {
			// SHOW COLUMNS result expects string value
			defaultValStr := fmt.Sprintf("%v", desc.DefaultValue)
			// If column is timestamp, and default value is not current_timestamp, should convert the default value to the current session time zone.
			if col.GetType() == mysql.TypeTimestamp && defaultValStr != types.ZeroDatetimeStr && !strings.HasPrefix(strings.ToUpper(defaultValStr), strings.ToUpper(ast.CurrentTimestamp)) {
				timeValue, err := table.GetColDefaultValue(e.Ctx().GetExprCtx(), col.ToInfo())
				if err != nil {
					return errors.Trace(err)
				}
				defaultValStr = timeValue.GetMysqlTime().String()
			}
			if col.GetType() == mysql.TypeBit {
				defaultValBinaryLiteral := types.BinaryLiteral(defaultValStr)
				columnDefault = defaultValBinaryLiteral.ToBitLiteralString(true)
			} else {
				columnDefault = defaultValStr
			}
		}

		// The FULL keyword causes the output to include the column collation and comments,
		// as well as the privileges you have for each column.
		if e.Full {
			e.appendRow([]any{
				desc.Field,
				desc.Type,
				desc.Collation,
				desc.Null,
				desc.Key,
				columnDefault,
				desc.Extra,
				desc.Privileges,
				desc.Comment,
			})
		} else {
			e.appendRow([]any{
				desc.Field,
				desc.Type,
				desc.Null,
				desc.Key,
				columnDefault,
				desc.Extra,
			})
		}
	}
	return nil
}

func (e *ShowExec) fetchShowIndex() error {
	do := domain.GetDomain(e.Ctx())
	h := do.StatsHandle()

	tb, err := e.getTable()
	if err != nil {
		return errors.Trace(err)
	}

	statsTbl := h.GetTableStats(tb.Meta())

	checker := privilege.GetPrivilegeManager(e.Ctx())
	activeRoles := e.Ctx().GetSessionVars().ActiveRoles
	if checker != nil && e.Ctx().GetSessionVars().User != nil && !checker.RequestVerification(activeRoles, e.DBName.O, tb.Meta().Name.O, "", mysql.AllPrivMask) {
		return e.tableAccessDenied("SELECT", tb.Meta().Name.O)
	}

	if tb.Meta().PKIsHandle {
		var pkCol *table.Column
		for _, col := range tb.Cols() {
			if mysql.HasPriKeyFlag(col.GetFlag()) {
				pkCol = col
				break
			}
		}
		colStats := statsTbl.GetCol(pkCol.ID)
		var ndv int64
		if colStats != nil {
			ndv = colStats.NDV
		}
		e.appendRow([]any{
			tb.Meta().Name.O, // Table
			0,                // Non_unique
			"PRIMARY",        // Key_name
			1,                // Seq_in_index
			pkCol.Name.O,     // Column_name
			"A",              // Collation
			ndv,              // Cardinality
			nil,              // Sub_part
			nil,              // Packed
			"",               // Null
			"BTREE",          // Index_type
			"",               // Comment
			"",               // Index_comment
			"YES",            // Index_visible
			nil,              // Expression
			"YES",            // Clustered
			"NO",             // Global_index
		})
	}
	for _, idx := range tb.Indices() {
		idxInfo := idx.Meta()
		if idxInfo.State != model.StatePublic {
			continue
		}
		isClustered := "NO"
		if tb.Meta().IsCommonHandle && idxInfo.Primary {
			isClustered = "YES"
		}

		isGlobalIndex := "NO"
		if idxInfo.Global {
			isGlobalIndex = "YES"
		}

		for i, col := range idxInfo.Columns {
			nonUniq := 1
			if idx.Meta().Unique {
				nonUniq = 0
			}

			var subPart any
			if col.Length != types.UnspecifiedLength {
				subPart = col.Length
			}

			tblCol := tb.Meta().Columns[col.Offset]
			nullVal := "YES"
			if mysql.HasNotNullFlag(tblCol.GetFlag()) {
				nullVal = ""
			}

			visible := "YES"
			if idx.Meta().Invisible {
				visible = "NO"
			}

			colName := col.Name.O
			var expression any
			if tblCol.Hidden {
				colName = "NULL"
				expression = tblCol.GeneratedExprString
			}

			colStats := statsTbl.GetCol(tblCol.ID)
			var ndv int64
			if colStats != nil {
				ndv = colStats.NDV
			}

			e.appendRow([]any{
				tb.Meta().Name.O,       // Table
				nonUniq,                // Non_unique
				idx.Meta().Name.O,      // Key_name
				i + 1,                  // Seq_in_index
				colName,                // Column_name
				"A",                    // Collation
				ndv,                    // Cardinality
				subPart,                // Sub_part
				nil,                    // Packed
				nullVal,                // Null
				idx.Meta().Tp.String(), // Index_type
				"",                     // Comment
				idx.Meta().Comment,     // Index_comment
				visible,                // Index_visible
				expression,             // Expression
				isClustered,            // Clustered
				isGlobalIndex,          // Global_index
			})
		}
	}
	return nil
}

// fetchShowCharset gets all charset information and fill them into e.rows.
// See http://dev.mysql.com/doc/refman/5.7/en/show-character-set.html
func (e *ShowExec) fetchShowCharset() error {
	descs := charset.GetSupportedCharsets()
	sessVars := e.Ctx().GetSessionVars()
	for _, desc := range descs {
		defaultCollation := desc.DefaultCollation
		if desc.Name == charset.CharsetUTF8MB4 {
			var err error
			defaultCollation, err = sessVars.GetSessionOrGlobalSystemVar(context.Background(), vardef.DefaultCollationForUTF8MB4)
			if err != nil {
				return err
			}
		}
		e.appendRow([]any{
			desc.Name,
			desc.Desc,
			defaultCollation,
			desc.Maxlen,
		})
	}
	return nil
}

func (e *ShowExec) fetchShowMasterStatus() error {
	tso := e.Ctx().GetSessionVars().TxnCtx.StartTS
	e.appendRow([]any{"tidb-binlog", tso, "", "", ""})
	return nil
}

func (e *ShowExec) fetchShowVariables(ctx context.Context) (err error) {
	var (
		value       string
		sessionVars = e.Ctx().GetSessionVars()
	)
	var (
		fieldPatternsLike collate.WildcardPattern
		fieldFilter       string
	)

	if e.Extractor != nil {
		fieldFilter = e.Extractor.Field()
		fieldPatternsLike = e.Extractor.FieldPatternLike()
	}
	if e.GlobalScope {
		// Collect global scope variables,
		// 1. Exclude the variables of ScopeSession in variable.SysVars;
		// 2. If the variable is ScopeNone, it's a read-only variable, return the default value of it,
		// 		otherwise, fetch the value from table `mysql.Global_Variables`.
		for _, v := range variable.GetSysVars() {
			if v.Scope != vardef.ScopeSession {
				if v.IsNoop && !vardef.EnableNoopVariables.Load() {
					continue
				}
				if fieldFilter != "" && v.Name != fieldFilter {
					continue
				} else if fieldPatternsLike != nil && !fieldPatternsLike.DoMatch(v.Name) {
					continue
				}
				if infoschema.SysVarHiddenForSem(e.Ctx(), v.Name) {
					continue
				}
				value, err = sessionVars.GetGlobalSystemVar(ctx, v.Name)
				if err != nil {
					return errors.Trace(err)
				}
				e.appendRow([]any{v.Name, value})
			}
		}
		return nil
	}

	// Collect session scope variables,
	// If it is a session only variable, use the default value defined in code,
	//   otherwise, fetch the value from table `mysql.Global_Variables`.
	for _, v := range variable.GetSysVars() {
		if v.IsNoop && !vardef.EnableNoopVariables.Load() {
			continue
		}
		if fieldFilter != "" && v.Name != fieldFilter {
			continue
		} else if fieldPatternsLike != nil && !fieldPatternsLike.DoMatch(v.Name) {
			continue
		}
		if infoschema.SysVarHiddenForSem(e.Ctx(), v.Name) {
			continue
		}
		value, err = sessionVars.GetSessionOrGlobalSystemVar(context.Background(), v.Name)
		if err != nil {
			return errors.Trace(err)
		}
		e.appendRow([]any{v.Name, value})
	}
	return nil
}

func (e *ShowExec) fetchShowStatus() error {
	sessionVars := e.Ctx().GetSessionVars()
	statusVars, err := variable.GetStatusVars(sessionVars)
	if err != nil {
		return errors.Trace(err)
	}
	checker := privilege.GetPrivilegeManager(e.Ctx())
	for status, v := range statusVars {
		if e.GlobalScope && v.Scope == vardef.ScopeSession {
			continue
		}
		// Skip invisible status vars if permission fails.
		if sem.IsEnabled() && sem.IsInvisibleStatusVar(status) {
			if checker == nil || !checker.RequestDynamicVerification(sessionVars.ActiveRoles, "RESTRICTED_STATUS_ADMIN", false) {
				continue
			}
		}
		switch v.Value.(type) {
		case []any, nil:
			v.Value = fmt.Sprintf("%v", v.Value)
		}
		value, err := types.ToString(v.Value)
		if err != nil {
			return errors.Trace(err)
		}
		e.appendRow([]any{status, value})
	}
	return nil
}

func getDefaultCollate(charsetName string) string {
	ch, err := charset.GetCharsetInfo(charsetName)
	if err != nil {
		// The charset is invalid, return server default.
		return mysql.DefaultCollationName
	}
	return ch.DefaultCollation
}

// ConstructResultOfShowCreateTable constructs the result for show create table.
func ConstructResultOfShowCreateTable(ctx sessionctx.Context, tableInfo *model.TableInfo, allocators autoid.Allocators, buf *bytes.Buffer) (err error) {
	return constructResultOfShowCreateTable(ctx, nil, tableInfo, allocators, buf)
}

func constructResultOfShowCreateTable(ctx sessionctx.Context, dbName *ast.CIStr, tableInfo *model.TableInfo, allocators autoid.Allocators, buf *bytes.Buffer) (err error) {
	if tableInfo.IsView() {
		fetchShowCreateTable4View(ctx, tableInfo, buf)
		return nil
	}
	if tableInfo.IsSequence() {
		ConstructResultOfShowCreateSequence(ctx, tableInfo, buf)
		return nil
	}

	tblCharset := tableInfo.Charset
	if len(tblCharset) == 0 {
		tblCharset = mysql.DefaultCharset
	}
	tblCollate := tableInfo.Collate
	// Set default collate if collate is not specified.
	if len(tblCollate) == 0 {
		tblCollate = getDefaultCollate(tblCharset)
	}

	sqlMode := ctx.GetSessionVars().SQLMode
	tableName := stringutil.Escape(tableInfo.Name.O, sqlMode)
	switch tableInfo.TempTableType {
	case model.TempTableGlobal:
		fmt.Fprintf(buf, "CREATE GLOBAL TEMPORARY TABLE %s (\n", tableName)
	case model.TempTableLocal:
		fmt.Fprintf(buf, "CREATE TEMPORARY TABLE %s (\n", tableName)
	default:
		fmt.Fprintf(buf, "CREATE TABLE %s (\n", tableName)
	}
	var pkCol *model.ColumnInfo
	var hasAutoIncID bool
	needAddComma := false
	for i, col := range tableInfo.Cols() {
		if col == nil || col.Hidden {
			continue
		}
		if needAddComma {
			buf.WriteString(",\n")
		}
		fmt.Fprintf(buf, "  %s %s", stringutil.Escape(col.Name.O, sqlMode), col.GetTypeDesc())
		if field_types.HasCharset(&col.FieldType) {
			if col.GetCharset() != tblCharset {
				fmt.Fprintf(buf, " CHARACTER SET %s", col.GetCharset())
			}
			if col.GetCollate() != tblCollate {
				fmt.Fprintf(buf, " COLLATE %s", col.GetCollate())
			} else {
				defcol, err := charset.GetDefaultCollation(col.GetCharset())
				if err == nil && defcol != col.GetCollate() {
					fmt.Fprintf(buf, " COLLATE %s", col.GetCollate())
				}
			}
		}
		if col.IsGenerated() {
			// It's a generated column.
			fmt.Fprintf(buf, " GENERATED ALWAYS AS (%s)", col.GeneratedExprString)
			if col.GeneratedStored {
				buf.WriteString(" STORED")
			} else {
				buf.WriteString(" VIRTUAL")
			}
		}
		if mysql.HasAutoIncrementFlag(col.GetFlag()) {
			hasAutoIncID = true
			buf.WriteString(" NOT NULL AUTO_INCREMENT")
		} else {
			if mysql.HasNotNullFlag(col.GetFlag()) {
				buf.WriteString(" NOT NULL")
			}
			// default values are not shown for generated columns in MySQL
			if !mysql.HasNoDefaultValueFlag(col.GetFlag()) && !col.IsGenerated() {
				defaultValue := col.GetDefaultValue()
				switch defaultValue {
				case nil:
					if !mysql.HasNotNullFlag(col.GetFlag()) {
						if col.GetType() == mysql.TypeTimestamp {
							buf.WriteString(" NULL")
						}
						buf.WriteString(" DEFAULT NULL")
					}
				case "CURRENT_TIMESTAMP":
					buf.WriteString(" DEFAULT ")
					buf.WriteString(defaultValue.(string))
					if col.GetDecimal() > 0 {
						fmt.Fprintf(buf, "(%d)", col.GetDecimal())
					}
				case "CURRENT_DATE":
					buf.WriteString(" DEFAULT (")
					buf.WriteString(defaultValue.(string))
					if col.GetDecimal() > 0 {
						fmt.Fprintf(buf, "(%d)", col.GetDecimal())
					}
					buf.WriteString(")")
				default:
					defaultValStr := fmt.Sprintf("%v", defaultValue)
					// If column is timestamp, and default value is not current_timestamp, should convert the default value to the current session time zone.
					if defaultValStr != types.ZeroDatetimeStr && col.GetType() == mysql.TypeTimestamp {
						timeValue, err := table.GetColDefaultValue(ctx.GetExprCtx(), col)
						if err != nil {
							return errors.Trace(err)
						}
						defaultValStr = timeValue.GetMysqlTime().String()
					}

					if col.DefaultIsExpr {
						fmt.Fprintf(buf, " DEFAULT (%s)", defaultValStr)
					} else {
						if col.GetType() == mysql.TypeBit {
							defaultValBinaryLiteral := types.BinaryLiteral(defaultValStr)
							fmt.Fprintf(buf, " DEFAULT %s", defaultValBinaryLiteral.ToBitLiteralString(true))
						} else {
							fmt.Fprintf(buf, " DEFAULT '%s'", format.OutputFormat(defaultValStr))
						}
					}
				}
			}
			if mysql.HasOnUpdateNowFlag(col.GetFlag()) {
				buf.WriteString(" ON UPDATE CURRENT_TIMESTAMP")
				buf.WriteString(table.OptionalFsp(&col.FieldType))
			}
		}
		if ddl.IsAutoRandomColumnID(tableInfo, col.ID) {
			s, r := tableInfo.AutoRandomBits, tableInfo.AutoRandomRangeBits
			if r == 0 || r == autoid.AutoRandomRangeBitsDefault {
				fmt.Fprintf(buf, " /*T![auto_rand] AUTO_RANDOM(%d) */", s)
			} else {
				fmt.Fprintf(buf, " /*T![auto_rand] AUTO_RANDOM(%d, %d) */", s, r)
			}
		}
		if len(col.Comment) > 0 {
			fmt.Fprintf(buf, " COMMENT '%s'", format.OutputFormat(col.Comment))
		}
		if i != len(tableInfo.Cols())-1 {
			needAddComma = true
		}
		if tableInfo.PKIsHandle && mysql.HasPriKeyFlag(col.GetFlag()) {
			pkCol = col
		}
	}

	if pkCol != nil {
		// If PKIsHandle, pk info is not in tb.Indices(). We should handle it here.
		buf.WriteString(",\n")
		fmt.Fprintf(buf, "  PRIMARY KEY (%s)", stringutil.Escape(pkCol.Name.O, sqlMode))
		buf.WriteString(" /*T![clustered_index] CLUSTERED */")
	}

	publicIndices := make([]*model.IndexInfo, 0, len(tableInfo.Indices))
	for _, idx := range tableInfo.Indices {
		if idx.State == model.StatePublic {
			publicIndices = append(publicIndices, idx)
		}
	}

	// consider hypo-indexes
	hypoIndexes := ctx.GetSessionVars().HypoIndexes
	if hypoIndexes != nil && dbName != nil {
		schemaName := dbName.L
		tblName := tableInfo.Name.L
		if hypoIndexes[schemaName] != nil && hypoIndexes[schemaName][tblName] != nil {
			hypoIndexList := make([]*model.IndexInfo, 0, len(hypoIndexes[schemaName][tblName]))
			for _, index := range hypoIndexes[schemaName][tblName] {
				hypoIndexList = append(hypoIndexList, index)
			}
			sort.Slice(hypoIndexList, func(i, j int) bool { // to make the result stable
				return hypoIndexList[i].Name.O < hypoIndexList[j].Name.O
			})
			publicIndices = append(publicIndices, hypoIndexList...)
		}
	}
	if len(publicIndices) > 0 {
		buf.WriteString(",\n")
	}

	for i, idxInfo := range publicIndices {
		if idxInfo.Primary {
			buf.WriteString("  PRIMARY KEY ")
		} else if idxInfo.Unique {
			fmt.Fprintf(buf, "  UNIQUE KEY %s ", stringutil.Escape(idxInfo.Name.O, sqlMode))
		} else if idxInfo.VectorInfo != nil {
			fmt.Fprintf(buf, "  VECTOR INDEX %s", stringutil.Escape(idxInfo.Name.O, sqlMode))
		} else if idxInfo.FullTextInfo != nil {
			fmt.Fprintf(buf, "  FULLTEXT INDEX %s", stringutil.Escape(idxInfo.Name.O, sqlMode))
		} else if idxInfo.InvertedInfo != nil {
			fmt.Fprintf(buf, "  COLUMNAR INDEX %s", stringutil.Escape(idxInfo.Name.O, sqlMode))
		} else {
			fmt.Fprintf(buf, "  KEY %s ", stringutil.Escape(idxInfo.Name.O, sqlMode))
		}

		cols := make([]string, 0, len(idxInfo.Columns))
		var colInfo string
		for _, c := range idxInfo.Columns {
			if tableInfo.Columns[c.Offset].Hidden {
				colInfo = fmt.Sprintf("(%s)", tableInfo.Columns[c.Offset].GeneratedExprString)
			} else {
				colInfo = stringutil.Escape(c.Name.O, sqlMode)
				if c.Length != types.UnspecifiedLength {
					colInfo = fmt.Sprintf("%s(%s)", colInfo, strconv.Itoa(c.Length))
				}
			}
			cols = append(cols, colInfo)
		}
		if idxInfo.VectorInfo != nil {
			funcName := model.IndexableDistanceMetricToFnName[idxInfo.VectorInfo.DistanceMetric]
			fmt.Fprintf(buf, "((%s(%s)))", strings.ToUpper(funcName), strings.Join(cols, ","))
		} else {
			fmt.Fprintf(buf, "(%s)", strings.Join(cols, ","))
		}

		if idxInfo.InvertedInfo != nil {
			fmt.Fprintf(buf, " USING INVERTED")
		}
		if idxInfo.FullTextInfo != nil {
			fmt.Fprintf(buf, " WITH PARSER %s", idxInfo.FullTextInfo.ParserType.SQLName())
		}
		if idxInfo.Invisible {
			fmt.Fprintf(buf, ` /*!80000 INVISIBLE */`)
		}
		if idxInfo.Comment != "" {
			fmt.Fprintf(buf, ` COMMENT '%s'`, format.OutputFormat(idxInfo.Comment))
		}
		if idxInfo.Tp == ast.IndexTypeHypo {
			fmt.Fprintf(buf, ` /* HYPO INDEX */`)
		}
		if idxInfo.Primary {
			if tableInfo.HasClusteredIndex() {
				buf.WriteString(" /*T![clustered_index] CLUSTERED */")
			} else {
				buf.WriteString(" /*T![clustered_index] NONCLUSTERED */")
			}
		}
		if idxInfo.Global {
			buf.WriteString(" /*T![global_index] GLOBAL */")
		}
		if i != len(publicIndices)-1 {
			buf.WriteString(",\n")
		}
	}

	// Foreign Keys are supported by data dictionary even though
	// they are not enforced by DDL. This is still helpful to applications.
	for _, fk := range tableInfo.ForeignKeys {
		fmt.Fprintf(buf, ",\n  CONSTRAINT %s FOREIGN KEY ", stringutil.Escape(fk.Name.O, sqlMode))
		colNames := make([]string, 0, len(fk.Cols))
		for _, col := range fk.Cols {
			colNames = append(colNames, stringutil.Escape(col.O, sqlMode))
		}
		fmt.Fprintf(buf, "(%s)", strings.Join(colNames, ","))
		if fk.RefSchema.L != "" && dbName != nil && fk.RefSchema.L != dbName.L {
			fmt.Fprintf(buf, " REFERENCES %s.%s ", stringutil.Escape(fk.RefSchema.O, sqlMode), stringutil.Escape(fk.RefTable.O, sqlMode))
		} else {
			fmt.Fprintf(buf, " REFERENCES %s ", stringutil.Escape(fk.RefTable.O, sqlMode))
		}
		refColNames := make([]string, 0, len(fk.Cols))
		for _, refCol := range fk.RefCols {
			refColNames = append(refColNames, stringutil.Escape(refCol.O, sqlMode))
		}
		fmt.Fprintf(buf, "(%s)", strings.Join(refColNames, ","))
		if ast.ReferOptionType(fk.OnDelete) != 0 {
			fmt.Fprintf(buf, " ON DELETE %s", ast.ReferOptionType(fk.OnDelete).String())
		}
		if ast.ReferOptionType(fk.OnUpdate) != 0 {
			fmt.Fprintf(buf, " ON UPDATE %s", ast.ReferOptionType(fk.OnUpdate).String())
		}
		if fk.Version < model.FKVersion1 {
			buf.WriteString(" /* FOREIGN KEY INVALID */")
		}
	}
	// add check constraints info
	publicConstraints := make([]*model.ConstraintInfo, 0, len(tableInfo.Indices))
	for _, constr := range tableInfo.Constraints {
		if constr.State == model.StatePublic {
			publicConstraints = append(publicConstraints, constr)
		}
	}
	if len(publicConstraints) > 0 {
		buf.WriteString(",\n")
	}
	for i, constrInfo := range publicConstraints {
		fmt.Fprintf(buf, "  CONSTRAINT %s CHECK ((%s))", stringutil.Escape(constrInfo.Name.O, sqlMode), constrInfo.ExprString)
		if !constrInfo.Enforced {
			buf.WriteString(" /*!80016 NOT ENFORCED */")
		}
		if i != len(publicConstraints)-1 {
			buf.WriteString(",\n")
		}
	}

	buf.WriteString("\n")

	buf.WriteString(") ENGINE=InnoDB")
	// We need to explicitly set the default charset and collation
	// to make it work on MySQL server which has default collate utf8_general_ci.
	if len(tblCollate) == 0 || tblCollate == "binary" {
		// If we can not find default collate for the given charset,
		// or the collate is 'binary'(MySQL-5.7 compatibility, see #15633 for details),
		// do not show the collate part.
		fmt.Fprintf(buf, " DEFAULT CHARSET=%s", tblCharset)
	} else {
		fmt.Fprintf(buf, " DEFAULT CHARSET=%s COLLATE=%s", tblCharset, tblCollate)
	}

	// Displayed if the compression typed is set.
	if len(tableInfo.Compression) != 0 {
		fmt.Fprintf(buf, " COMPRESSION='%s'", tableInfo.Compression)
	}

	incrementAllocator := allocators.Get(autoid.AutoIncrementType)
	if hasAutoIncID && incrementAllocator != nil {
		autoIncID, err := incrementAllocator.NextGlobalAutoID()
		if err != nil {
			return errors.Trace(err)
		}

		// It's compatible with MySQL.
		if autoIncID > 1 {
			fmt.Fprintf(buf, " AUTO_INCREMENT=%d", autoIncID)
		}
	}

	if tableInfo.AutoIDCache != 0 {
		fmt.Fprintf(buf, " /*T![auto_id_cache] AUTO_ID_CACHE=%d */", tableInfo.AutoIDCache)
	}

	randomAllocator := allocators.Get(autoid.AutoRandomType)
	if randomAllocator != nil {
		autoRandID, err := randomAllocator.NextGlobalAutoID()
		if err != nil {
			return errors.Trace(err)
		}

		if autoRandID > 1 {
			fmt.Fprintf(buf, " /*T![auto_rand_base] AUTO_RANDOM_BASE=%d */", autoRandID)
		}
	}

	if tableInfo.ShardRowIDBits > 0 {
		fmt.Fprintf(buf, " /*T! SHARD_ROW_ID_BITS=%d ", tableInfo.ShardRowIDBits)
		if tableInfo.PreSplitRegions > 0 {
			fmt.Fprintf(buf, "PRE_SPLIT_REGIONS=%d ", tableInfo.PreSplitRegions)
		}
		buf.WriteString("*/")
	}

	if tableInfo.AutoRandomBits > 0 && tableInfo.PreSplitRegions > 0 {
		fmt.Fprintf(buf, " /*T! PRE_SPLIT_REGIONS=%d */", tableInfo.PreSplitRegions)
	}

	if len(tableInfo.Comment) > 0 {
		fmt.Fprintf(buf, " COMMENT='%s'", format.OutputFormat(tableInfo.Comment))
	}

	if tableInfo.TempTableType == model.TempTableGlobal {
		fmt.Fprintf(buf, " ON COMMIT DELETE ROWS")
	}

	if tableInfo.PlacementPolicyRef != nil {
		fmt.Fprintf(buf, " /*T![placement] PLACEMENT POLICY=%s */", stringutil.Escape(tableInfo.PlacementPolicyRef.Name.String(), sqlMode))
	}

	if tableInfo.TableCacheStatusType == model.TableCacheStatusEnable {
		// This is not meant to be understand by other components, so it's not written as /*T![cached] */
		// For all external components, cached table is just a normal table.
		fmt.Fprintf(buf, " /* CACHED ON */")
	}

	// add partition info here.
	ddl.AppendPartitionInfo(tableInfo.Partition, buf, sqlMode)

	if tableInfo.TTLInfo != nil {
		restoreFlags := parserformat.RestoreStringSingleQuotes | parserformat.RestoreNameBackQuotes | parserformat.RestoreTiDBSpecialComment
		restoreCtx := parserformat.NewRestoreCtx(restoreFlags, buf)

		restoreCtx.WritePlain(" ")
		err = restoreCtx.WriteWithSpecialComments(tidb.FeatureIDTTL, func() error {
			columnName := ast.ColumnName{Name: tableInfo.TTLInfo.ColumnName}
			timeUnit := ast.TimeUnitExpr{Unit: ast.TimeUnitType(tableInfo.TTLInfo.IntervalTimeUnit)}
			restoreCtx.WriteKeyWord("TTL")
			restoreCtx.WritePlain("=")
			restoreCtx.WriteName(columnName.String())
			restoreCtx.WritePlainf(" + INTERVAL %s ", tableInfo.TTLInfo.IntervalExprStr)
			return timeUnit.Restore(restoreCtx)
		})

		if err != nil {
			return err
		}

		restoreCtx.WritePlain(" ")
		err = restoreCtx.WriteWithSpecialComments(tidb.FeatureIDTTL, func() error {
			restoreCtx.WriteKeyWord("TTL_ENABLE")
			restoreCtx.WritePlain("=")
			if tableInfo.TTLInfo.Enable {
				restoreCtx.WriteString("ON")
			} else {
				restoreCtx.WriteString("OFF")
			}
			return nil
		})

		if err != nil {
			return err
		}

		restoreCtx.WritePlain(" ")
		err = restoreCtx.WriteWithSpecialComments(tidb.FeatureIDTTL, func() error {
			restoreCtx.WriteKeyWord("TTL_JOB_INTERVAL")
			restoreCtx.WritePlain("=")
			if len(tableInfo.TTLInfo.JobInterval) == 0 {
				// This only happens when the table is created from 6.5 in which the `tidb_job_interval` is not introduced yet.
				// We use `OldDefaultTTLJobInterval` as the return value to ensure a consistent behavior for the
				// upgrades: v6.5 -> v8.5(or previous version) -> newer version than v8.5.
				restoreCtx.WriteString(model.OldDefaultTTLJobInterval)
			} else {
				restoreCtx.WriteString(tableInfo.TTLInfo.JobInterval)
			}
			return nil
		})

		if err != nil {
			return err
		}
	}
	return nil
}

// ConstructResultOfShowCreateSequence constructs the result for show create sequence.
func ConstructResultOfShowCreateSequence(ctx sessionctx.Context, tableInfo *model.TableInfo, buf *bytes.Buffer) {
	sqlMode := ctx.GetSessionVars().SQLMode
	fmt.Fprintf(buf, "CREATE SEQUENCE %s ", stringutil.Escape(tableInfo.Name.O, sqlMode))
	sequenceInfo := tableInfo.Sequence
	fmt.Fprintf(buf, "start with %d ", sequenceInfo.Start)
	fmt.Fprintf(buf, "minvalue %d ", sequenceInfo.MinValue)
	fmt.Fprintf(buf, "maxvalue %d ", sequenceInfo.MaxValue)
	fmt.Fprintf(buf, "increment by %d ", sequenceInfo.Increment)
	if sequenceInfo.Cache {
		fmt.Fprintf(buf, "cache %d ", sequenceInfo.CacheValue)
	} else {
		buf.WriteString("nocache ")
	}
	if sequenceInfo.Cycle {
		buf.WriteString("cycle ")
	} else {
		buf.WriteString("nocycle ")
	}
	buf.WriteString("ENGINE=InnoDB")
	if len(sequenceInfo.Comment) > 0 {
		fmt.Fprintf(buf, " COMMENT='%s'", format.OutputFormat(sequenceInfo.Comment))
	}
}

func (e *ShowExec) fetchShowCreateSequence() error {
	tbl, err := e.getTable()
	if err != nil {
		return errors.Trace(err)
	}
	tableInfo := tbl.Meta()
	if !tableInfo.IsSequence() {
		return exeerrors.ErrWrongObject.GenWithStackByArgs(e.DBName.O, tableInfo.Name.O, "SEQUENCE")
	}
	var buf bytes.Buffer
	ConstructResultOfShowCreateSequence(e.Ctx(), tableInfo, &buf)
	e.appendRow([]any{tableInfo.Name.O, buf.String()})
	return nil
}

// TestShowClusterConfigKey is the key used to store TestShowClusterConfigFunc.
var TestShowClusterConfigKey stringutil.StringerStr = "TestShowClusterConfigKey"

// TestShowClusterConfigFunc is used to test 'show config ...'.
type TestShowClusterConfigFunc func() ([][]types.Datum, error)

func (e *ShowExec) fetchShowClusterConfigs() error {
	emptySet := set.NewStringSet()
	var confItems [][]types.Datum
	var err error
	if f := e.Ctx().Value(TestShowClusterConfigKey); f != nil {
		confItems, err = f.(TestShowClusterConfigFunc)()
	} else {
		confItems, err = fetchClusterConfig(e.Ctx(), emptySet, emptySet)
	}
	if err != nil {
		return err
	}
	for _, items := range confItems {
		row := make([]any, 0, 4)
		for _, item := range items {
			row = append(row, item.GetString())
		}
		e.appendRow(row)
	}
	return nil
}

func (e *ShowExec) fetchShowCreateTable() error {
	tb, err := e.getTable()
	if err != nil {
		return errors.Trace(err)
	}

	tableInfo := tb.Meta()
	var buf bytes.Buffer
	// TODO: let the result more like MySQL.
	if err = constructResultOfShowCreateTable(e.Ctx(), &e.DBName, tableInfo, tb.Allocators(e.Ctx().GetTableCtx()), &buf); err != nil {
		return err
	}
	if tableInfo.IsView() {
		e.appendRow([]any{tableInfo.Name.O, buf.String(), tableInfo.Charset, tableInfo.Collate})
		return nil
	}

	e.appendRow([]any{tableInfo.Name.O, buf.String()})
	return nil
}

func (e *ShowExec) fetchShowCreateView() error {
	db, ok := e.is.SchemaByName(e.DBName)
	if !ok {
		return infoschema.ErrDatabaseNotExists.GenWithStackByArgs(e.DBName.O)
	}

	tb, err := e.getTable()
	if err != nil {
		return errors.Trace(err)
	}

	if !tb.Meta().IsView() {
		return exeerrors.ErrWrongObject.GenWithStackByArgs(db.Name.O, tb.Meta().Name.O, "VIEW")
	}

	var buf bytes.Buffer
	fetchShowCreateTable4View(e.Ctx(), tb.Meta(), &buf)
	e.appendRow([]any{tb.Meta().Name.O, buf.String(), tb.Meta().Charset, tb.Meta().Collate})
	return nil
}

func fetchShowCreateTable4View(ctx sessionctx.Context, tb *model.TableInfo, buf *bytes.Buffer) {
	sqlMode := ctx.GetSessionVars().SQLMode
	fmt.Fprintf(buf, "CREATE ALGORITHM=%s ", tb.View.Algorithm.String())
	if tb.View.Definer.AuthUsername == "" || tb.View.Definer.AuthHostname == "" {
		fmt.Fprintf(buf, "DEFINER=%s@%s ", stringutil.Escape(tb.View.Definer.Username, sqlMode), stringutil.Escape(tb.View.Definer.Hostname, sqlMode))
	} else {
		fmt.Fprintf(buf, "DEFINER=%s@%s ", stringutil.Escape(tb.View.Definer.AuthUsername, sqlMode), stringutil.Escape(tb.View.Definer.AuthHostname, sqlMode))
	}
	fmt.Fprintf(buf, "SQL SECURITY %s ", tb.View.Security.String())
	fmt.Fprintf(buf, "VIEW %s (", stringutil.Escape(tb.Name.O, sqlMode))
	for i, col := range tb.Columns {
		fmt.Fprintf(buf, "%s", stringutil.Escape(col.Name.O, sqlMode))
		if i < len(tb.Columns)-1 {
			fmt.Fprintf(buf, ", ")
		}
	}
	fmt.Fprintf(buf, ") AS %s", tb.View.SelectStmt)
}

// ConstructResultOfShowCreateDatabase constructs the result for show create database.
func ConstructResultOfShowCreateDatabase(ctx sessionctx.Context, dbInfo *model.DBInfo, ifNotExists bool, buf *bytes.Buffer) (err error) {
	sqlMode := ctx.GetSessionVars().SQLMode
	var ifNotExistsStr string
	if ifNotExists {
		ifNotExistsStr = "IF NOT EXISTS "
	}
	fmt.Fprintf(buf, "CREATE DATABASE %s%s", ifNotExistsStr, stringutil.Escape(dbInfo.Name.O, sqlMode))
	if dbInfo.Charset != "" {
		fmt.Fprintf(buf, " /*!40100 DEFAULT CHARACTER SET %s ", dbInfo.Charset)
		defaultCollate, err := charset.GetDefaultCollation(dbInfo.Charset)
		if err != nil {
			return errors.Trace(err)
		}
		if dbInfo.Collate != "" && dbInfo.Collate != defaultCollate {
			fmt.Fprintf(buf, "COLLATE %s ", dbInfo.Collate)
		}
		fmt.Fprint(buf, "*/")
	} else if dbInfo.Collate != "" {
		collInfo, err := collate.GetCollationByName(dbInfo.Collate)
		if err != nil {
			return errors.Trace(err)
		}
		fmt.Fprintf(buf, " /*!40100 DEFAULT CHARACTER SET %s ", collInfo.CharsetName)
		if !collInfo.IsDefault {
			fmt.Fprintf(buf, "COLLATE %s ", dbInfo.Collate)
		}
		fmt.Fprint(buf, "*/")
	}
	// MySQL 5.7 always show the charset info but TiDB may ignore it, which makes a slight difference. We keep this
	// behavior unchanged because it is trivial enough.
	if dbInfo.PlacementPolicyRef != nil {
		// add placement ref info here
		fmt.Fprintf(buf, " /*T![placement] PLACEMENT POLICY=%s */", stringutil.Escape(dbInfo.PlacementPolicyRef.Name.O, sqlMode))
	}
	return nil
}

// ConstructResultOfShowCreatePlacementPolicy constructs the result for show create placement policy.
func ConstructResultOfShowCreatePlacementPolicy(policyInfo *model.PolicyInfo) string {
	return fmt.Sprintf("CREATE PLACEMENT POLICY `%s` %s", policyInfo.Name.O, policyInfo.PlacementSettings.String())
}

// constructResultOfShowCreateResourceGroup constructs the result for show create resource group.
func constructResultOfShowCreateResourceGroup(resourceGroup *model.ResourceGroupInfo) string {
	return fmt.Sprintf("CREATE RESOURCE GROUP `%s` %s", resourceGroup.Name.O, resourceGroup.ResourceGroupSettings.String())
}

// fetchShowCreateDatabase composes show create database result.
func (e *ShowExec) fetchShowCreateDatabase() error {
	checker := privilege.GetPrivilegeManager(e.Ctx())
	if checker != nil && e.Ctx().GetSessionVars().User != nil {
		if !checker.DBIsVisible(e.Ctx().GetSessionVars().ActiveRoles, e.DBName.String()) {
			return e.dbAccessDenied()
		}
	}
	dbInfo, ok := e.is.SchemaByName(e.DBName)
	if !ok {
		return infoschema.ErrDatabaseNotExists.GenWithStackByArgs(e.DBName.O)
	}

	var buf bytes.Buffer
	err := ConstructResultOfShowCreateDatabase(e.Ctx(), dbInfo, e.IfNotExists, &buf)
	if err != nil {
		return err
	}
	e.appendRow([]any{dbInfo.Name.O, buf.String()})
	return nil
}

// fetchShowCreatePlacementPolicy composes show create policy result.
func (e *ShowExec) fetchShowCreatePlacementPolicy() error {
	policy, found := e.is.PolicyByName(e.DBName)
	if !found {
		return infoschema.ErrPlacementPolicyNotExists.GenWithStackByArgs(e.DBName.O)
	}
	showCreate := ConstructResultOfShowCreatePlacementPolicy(policy)
	e.appendRow([]any{e.DBName.O, showCreate})
	return nil
}

// fetchShowCreateResourceGroup composes show create resource group result.
func (e *ShowExec) fetchShowCreateResourceGroup() error {
	group, found := e.is.ResourceGroupByName(e.ResourceGroupName)
	if !found {
		return infoschema.ErrResourceGroupNotExists.GenWithStackByArgs(e.ResourceGroupName.O)
	}
	showCreate := constructResultOfShowCreateResourceGroup(group)
	e.appendRow([]any{e.ResourceGroupName.O, showCreate})
	return nil
}

// isUTF8MB4AndDefaultCollation returns if the cs is utf8mb4 and the co is DefaultCollationForUTF8MB4.
func isUTF8MB4AndDefaultCollation(sessVars *variable.SessionVars, cs, co string) (isUTF8MB4 bool, isDefault bool, err error) {
	if cs != charset.CharsetUTF8MB4 {
		return false, false, nil
	}
	defaultCollation, err := sessVars.GetSessionOrGlobalSystemVar(context.Background(), vardef.DefaultCollationForUTF8MB4)
	if err != nil {
		return false, false, err
	}
	if co == defaultCollation {
		return true, true, nil
	}
	return true, false, nil
}

func (e *ShowExec) fetchShowCollation() error {
	var (
		fieldPatternsLike collate.WildcardPattern
		fieldFilter       string
	)
	if e.Extractor != nil {
		fieldPatternsLike = e.Extractor.FieldPatternLike()
		fieldFilter = e.Extractor.Field()
	}

	sessVars := e.Ctx().GetSessionVars()
	collations := collate.GetSupportedCollations()
	for _, v := range collations {
		isDefault := ""
		isUTF8MB4, isDefaultCollation, err := isUTF8MB4AndDefaultCollation(sessVars, v.CharsetName, v.Name)
		if err != nil {
			return err
		}
		if isUTF8MB4 && isDefaultCollation {
			isDefault = "Yes"
		} else if !isUTF8MB4 && v.IsDefault {
			isDefault = "Yes"
		}
		if fieldFilter != "" && strings.ToLower(v.Name) != fieldFilter {
			continue
		} else if fieldPatternsLike != nil && !fieldPatternsLike.DoMatch(v.Name) {
			continue
		}
		e.appendRow([]any{
			v.Name,
			v.CharsetName,
			v.ID,
			isDefault,
			"Yes",
			v.Sortlen,
			v.PadAttribute,
		})
	}
	return nil
}

// fetchShowCreateUser composes 'show create user' result.
func (e *ShowExec) fetchShowCreateUser(ctx context.Context) error {
	checker := privilege.GetPrivilegeManager(e.Ctx())
	if checker == nil {
		return errors.New("miss privilege checker")
	}
	ctx = kv.WithInternalSourceType(ctx, kv.InternalTxnPrivilege)

	userName, hostName := e.User.Username, e.User.Hostname
	sessVars := e.Ctx().GetSessionVars()
	if e.User.CurrentUser {
		userName = sessVars.User.AuthUsername
		hostName = sessVars.User.AuthHostname
	} else {
		// Show create user requires the SELECT privilege on mysql.user.
		// Ref https://dev.mysql.com/doc/refman/5.7/en/show-create-user.html
		activeRoles := sessVars.ActiveRoles
		if !checker.RequestVerification(activeRoles, mysql.SystemDB, mysql.UserTable, "", mysql.SelectPriv) {
			return e.tableAccessDenied("SELECT", mysql.UserTable)
		}
	}

	exec := e.Ctx().GetRestrictedSQLExecutor()

	rows, _, err := exec.ExecRestrictedSQL(ctx, nil,
		`SELECT plugin, Account_locked, user_attributes->>'$.metadata', Token_issuer,
        Password_reuse_history, Password_reuse_time, Password_expired, Password_lifetime,
        user_attributes->>'$.Password_locking.failed_login_attempts',
        user_attributes->>'$.Password_locking.password_lock_time_days', authentication_string,
        Max_user_connections
		FROM %n.%n WHERE User=%? AND Host=%?`,
		mysql.SystemDB, mysql.UserTable, userName, strings.ToLower(hostName))
	if err != nil {
		return errors.Trace(err)
	}

	if len(rows) == 0 {
		// FIXME: the error returned is not escaped safely
		return exeerrors.ErrCannotUser.GenWithStackByArgs("SHOW CREATE USER",
			fmt.Sprintf("'%s'@'%s'", e.User.Username, e.User.Hostname))
	}

	authPlugin, err := e.Ctx().GetSessionVars().GlobalVarsAccessor.GetGlobalSysVar(vardef.DefaultAuthPlugin)
	if err != nil {
		return errors.Trace(err)
	}
	if len(rows) == 1 && rows[0].GetString(0) != "" {
		authPlugin = rows[0].GetString(0)
	}

	accountLockedRaw := rows[0].GetString(1)
	accountLocked := "LOCK"
	if accountLockedRaw[len(accountLockedRaw)-1:] == "N" {
		accountLocked = "UNLOCK"
	}

	userAttributes := rows[0].GetString(2)
	if len(userAttributes) > 0 {
		userAttributes = fmt.Sprintf(" ATTRIBUTE '%s'", userAttributes)
	}

	tokenIssuer := rows[0].GetString(3)
	if len(tokenIssuer) > 0 {
		tokenIssuer = " token_issuer " + tokenIssuer
	}

	var passwordHistory string
	if rows[0].IsNull(4) {
		passwordHistory = "DEFAULT"
	} else {
		passwordHistory = strconv.FormatUint(rows[0].GetUint64(4), 10)
	}

	var passwordReuseInterval string
	if rows[0].IsNull(5) {
		passwordReuseInterval = "DEFAULT"
	} else {
		passwordReuseInterval = strconv.FormatUint(rows[0].GetUint64(5), 10) + " DAY"
	}

	passwordExpired := rows[0].GetEnum(6).String()
	passwordLifetime := int64(-1)
	if !rows[0].IsNull(7) {
		passwordLifetime = rows[0].GetInt64(7)
	}
	passwordExpiredStr := "PASSWORD EXPIRE DEFAULT"
	if passwordExpired == "Y" {
		passwordExpiredStr = "PASSWORD EXPIRE"
	} else if passwordLifetime == 0 {
		passwordExpiredStr = "PASSWORD EXPIRE NEVER"
	} else if passwordLifetime > 0 {
		passwordExpiredStr = fmt.Sprintf("PASSWORD EXPIRE INTERVAL %d DAY", passwordLifetime)
	}

	failedLoginAttempts := rows[0].GetString(8)
	if len(failedLoginAttempts) > 0 {
		failedLoginAttempts = " FAILED_LOGIN_ATTEMPTS " + failedLoginAttempts
	}

	passwordLockTimeDays := rows[0].GetString(9)
	if len(passwordLockTimeDays) > 0 {
		if passwordLockTimeDays == "-1" {
			passwordLockTimeDays = " PASSWORD_LOCK_TIME UNBOUNDED"
		} else {
			passwordLockTimeDays = " PASSWORD_LOCK_TIME " + passwordLockTimeDays
		}
	}
	authData := rows[0].GetString(10)

	maxUserConnections := rows[0].GetInt64(11)
	maxUserConnectionsStr := ""
	if maxUserConnections > 0 {
		maxUserConnectionsStr = fmt.Sprintf(" WITH MAX_USER_CONNECTIONS %d", maxUserConnections)
	}

	rows, _, err = exec.ExecRestrictedSQL(ctx, nil, `SELECT Priv FROM %n.%n WHERE User=%? AND Host=%?`, mysql.SystemDB, mysql.GlobalPrivTable, userName, hostName)
	if err != nil {
		return errors.Trace(err)
	}

	require := "NONE"
	if len(rows) == 1 {
		privData := rows[0].GetString(0)
		var privValue privileges.GlobalPrivValue
		err = gjson.Unmarshal(hack.Slice(privData), &privValue)
		if err != nil {
			return errors.Trace(err)
		}
		require = privValue.RequireStr()
	}

	authStr := ""
	if !(authPlugin == mysql.AuthSocket && authData == "") {
		authStr = fmt.Sprintf(" AS '%s'", authData)
	}

	// FIXME: the returned string is not escaped safely
	showStr := fmt.Sprintf("CREATE USER '%s'@'%s' IDENTIFIED WITH '%s'%s REQUIRE %s%s%s %s ACCOUNT %s PASSWORD HISTORY %s PASSWORD REUSE INTERVAL %s%s%s%s",
		e.User.Username, e.User.Hostname, authPlugin, authStr, require, tokenIssuer, maxUserConnectionsStr, passwordExpiredStr, accountLocked, passwordHistory, passwordReuseInterval, failedLoginAttempts, passwordLockTimeDays, userAttributes)
	e.appendRow([]any{showStr})
	return nil
}

func (e *ShowExec) fetchShowGrants(ctx context.Context) error {
	vars := e.Ctx().GetSessionVars()
	checker := privilege.GetPrivilegeManager(e.Ctx())
	if checker == nil {
		return errors.New("miss privilege checker")
	}
	if e.User == nil || e.User.CurrentUser {
		// The input is a "SHOW GRANTS" statement with no users *or* SHOW GRANTS FOR CURRENT_USER()
		// In these cases we include the active roles for showing privileges.
		e.User = &auth.UserIdentity{Username: vars.User.AuthUsername, Hostname: vars.User.AuthHostname}
		if len(e.Roles) == 0 {
			e.Roles = vars.ActiveRoles
		}
	} else {
		userName := vars.User.AuthUsername
		hostName := vars.User.AuthHostname
		// Show grant user requires the SELECT privilege on mysql schema.
		// Ref https://dev.mysql.com/doc/refman/8.0/en/show-grants.html
		if userName != e.User.Username || hostName != e.User.Hostname {
			if !checker.RequestVerification(vars.ActiveRoles, mysql.SystemDB, "", "", mysql.SelectPriv) {
				return exeerrors.ErrDBaccessDenied.GenWithStackByArgs(userName, hostName, mysql.SystemDB)
			}
		}
	}
	// This is for the syntax SHOW GRANTS FOR x USING role
	for _, r := range e.Roles {
		if r.Hostname == "" {
			r.Hostname = "%"
		}
		if !checker.FindEdge(ctx, r, e.User) {
			return exeerrors.ErrRoleNotGranted.GenWithStackByArgs(r.String(), e.User.String())
		}
	}
	gs, err := checker.ShowGrants(ctx, e.Ctx(), e.User, e.Roles)
	if err != nil {
		return errors.Trace(err)
	}
	for _, g := range gs {
		e.appendRow([]any{g})
	}
	return nil
}

func (e *ShowExec) fetchShowPrivileges() error {
	e.appendRow([]any{"Alter", "Tables", "To alter the table"})
	e.appendRow([]any{"Alter routine", "Functions,Procedures", "To alter or drop stored functions/procedures"})
	e.appendRow([]any{"Config", "Server Admin", "To use SHOW CONFIG and SET CONFIG statements"})
	e.appendRow([]any{"Create", "Databases,Tables,Indexes", "To create new databases and tables"})
	e.appendRow([]any{"Create routine", "Databases", "To use CREATE FUNCTION/PROCEDURE"})
	e.appendRow([]any{"Create role", "Server Admin", "To create new roles"})
	e.appendRow([]any{"Create temporary tables", "Databases", "To use CREATE TEMPORARY TABLE"})
	e.appendRow([]any{"Create view", "Tables", "To create new views"})
	e.appendRow([]any{"Create user", "Server Admin", "To create new users"})
	e.appendRow([]any{"Delete", "Tables", "To delete existing rows"})
	e.appendRow([]any{"Drop", "Databases,Tables", "To drop databases, tables, and views"})
	e.appendRow([]any{"Drop role", "Server Admin", "To drop roles"})
	e.appendRow([]any{"Event", "Server Admin", "To create, alter, drop and execute events"})
	e.appendRow([]any{"Execute", "Functions,Procedures", "To execute stored routines"})
	e.appendRow([]any{"File", "File access on server", "To read and write files on the server"})
	e.appendRow([]any{"Grant option", "Databases,Tables,Functions,Procedures", "To give to other users those privileges you possess"})
	e.appendRow([]any{"Index", "Tables", "To create or drop indexes"})
	e.appendRow([]any{"Insert", "Tables", "To insert data into tables"})
	e.appendRow([]any{"Lock tables", "Databases", "To use LOCK TABLES (together with SELECT privilege)"})
	e.appendRow([]any{"Process", "Server Admin", "To view the plain text of currently executing queries"})
	e.appendRow([]any{"Proxy", "Server Admin", "To make proxy user possible"})
	e.appendRow([]any{"References", "Databases,Tables", "To have references on tables"})
	e.appendRow([]any{"Reload", "Server Admin", "To reload or refresh tables, logs and privileges"})
	e.appendRow([]any{"Replication client", "Server Admin", "To ask where the slave or master servers are"})
	e.appendRow([]any{"Replication slave", "Server Admin", "To read binary log events from the master"})
	e.appendRow([]any{"Select", "Tables", "To retrieve rows from table"})
	e.appendRow([]any{"Show databases", "Server Admin", "To see all databases with SHOW DATABASES"})
	e.appendRow([]any{"Show view", "Tables", "To see views with SHOW CREATE VIEW"})
	e.appendRow([]any{"Shutdown", "Server Admin", "To shut down the server"})
	e.appendRow([]any{"Super", "Server Admin", "To use KILL thread, SET GLOBAL, CHANGE MASTER, etc."})
	e.appendRow([]any{"Trigger", "Tables", "To use triggers"})
	e.appendRow([]any{"Create tablespace", "Server Admin", "To create/alter/drop tablespaces"})
	e.appendRow([]any{"Update", "Tables", "To update existing rows"})
	e.appendRow([]any{"Usage", "Server Admin", "No privileges - allow connect only"})

	for _, priv := range privileges.GetDynamicPrivileges() {
		e.appendRow([]any{priv, "Server Admin", ""})
	}
	return nil
}

func (*ShowExec) fetchShowTriggers() error {
	return nil
}

func (*ShowExec) fetchShowProcedureStatus() error {
	return nil
}

func (e *ShowExec) fetchShowPlugins() error {
	tiPlugins := plugin.GetAll()
	for _, ps := range tiPlugins {
		for _, p := range ps {
			e.appendRow([]any{p.Name, p.StateValue(), p.Kind.String(), p.Path, p.License, strconv.Itoa(int(p.Version))})
		}
	}
	return nil
}

func (e *ShowExec) fetchShowWarnings(errOnly bool) error {
	stmtCtx := e.Ctx().GetSessionVars().StmtCtx
	if e.CountWarningsOrErrors {
		errCount, warnCount := stmtCtx.NumErrorWarnings()
		if errOnly {
			e.appendRow([]any{int64(errCount)})
		} else {
			e.appendRow([]any{int64(warnCount)})
		}
		return nil
	}
	for _, w := range stmtCtx.GetWarnings() {
		if errOnly && w.Level != contextutil.WarnLevelError {
			continue
		}
		warn := errors.Cause(w.Err)
		switch x := warn.(type) {
		case *terror.Error:
			sqlErr := terror.ToSQLError(x)
			e.appendRow([]any{w.Level, int64(sqlErr.Code), sqlErr.Message})
		default:
			var err string
			if warn != nil {
				err = warn.Error()
			}
			e.appendRow([]any{w.Level, int64(mysql.ErrUnknown), err})
		}
	}
	return nil
}

func (e *ShowExec) getTable() (table.Table, error) {
	if e.Table == nil {
		return nil, errors.New("table not found")
	}
	tb, ok := e.is.TableByID(context.Background(), e.Table.TableInfo.ID)
	if !ok {
		return nil, errors.Errorf("table %s not found", e.Table.Name)
	}
	return tb, nil
}

func (e *ShowExec) dbAccessDenied() error {
	user := e.Ctx().GetSessionVars().User
	u := user.Username
	h := user.Hostname
	if len(user.AuthUsername) > 0 && len(user.AuthHostname) > 0 {
		u = user.AuthUsername
		h = user.AuthHostname
	}
	return exeerrors.ErrDBaccessDenied.GenWithStackByArgs(u, h, e.DBName)
}

func (e *ShowExec) tableAccessDenied(access string, table string) error {
	user := e.Ctx().GetSessionVars().User
	u := user.Username
	h := user.Hostname
	if len(user.AuthUsername) > 0 && len(user.AuthHostname) > 0 {
		u = user.AuthUsername
		h = user.AuthHostname
	}
	return exeerrors.ErrTableaccessDenied.GenWithStackByArgs(access, u, h, table)
}

func (e *ShowExec) appendRow(row []any) {
	for i, col := range row {
		switch x := col.(type) {
		case nil:
			e.result.AppendNull(i)
		case int:
			e.result.AppendInt64(i, int64(x))
		case int64:
			e.result.AppendInt64(i, x)
		case uint64:
			e.result.AppendUint64(i, x)
		case float64:
			e.result.AppendFloat64(i, x)
		case float32:
			e.result.AppendFloat32(i, x)
		case string:
			e.result.AppendString(i, x)
		case []byte:
			e.result.AppendBytes(i, x)
		case types.BinaryLiteral:
			e.result.AppendBytes(i, x)
		case *types.MyDecimal:
			e.result.AppendMyDecimal(i, x)
		case types.Time:
			e.result.AppendTime(i, x)
		case types.BinaryJSON:
			e.result.AppendJSON(i, x)
		case types.VectorFloat32:
			e.result.AppendVectorFloat32(i, x)
		case types.Duration:
			e.result.AppendDuration(i, x)
		case types.Enum:
			e.result.AppendEnum(i, x)
		case types.Set:
			e.result.AppendSet(i, x)
		default:
			e.result.AppendNull(i)
		}
	}
}

func (e *ShowExec) fetchShowDistributions(ctx context.Context) error {
	tb, err := e.getTable()
	if err != nil {
		return errors.Trace(err)
	}
	physicalIDs := []int64{}
	partitionNames := make([]string, 0)
	if pi := tb.Meta().GetPartitionInfo(); pi != nil {
		for _, name := range e.Table.PartitionNames {
			pid, err := tables.FindPartitionByName(tb.Meta(), name.L)
			if err != nil {
				return err
			}
			physicalIDs = append(physicalIDs, pid)
			partitionNames = append(partitionNames, name.L)
		}
		if len(physicalIDs) == 0 {
			for _, p := range pi.Definitions {
				physicalIDs = append(physicalIDs, p.ID)
				partitionNames = append(partitionNames, p.Name.L)
			}
		}
	} else {
		if len(e.Table.PartitionNames) != 0 {
			return plannererrors.ErrPartitionClauseOnNonpartitioned
		}
		physicalIDs = append(physicalIDs, tb.Meta().ID)
		partitionNames = append(partitionNames, tb.Meta().Name.L)
	}
	distributions := make([]*pdHttp.RegionDistribution, 0)
	var resp *pdHttp.RegionDistributions
	for idx, pid := range physicalIDs {
		startKey := codec.EncodeBytes([]byte{}, tablecodec.GenTablePrefix(pid))
		endKey := codec.EncodeBytes([]byte{}, tablecodec.GenTablePrefix(pid+1))
		// todo： support engine type
		resp, err = infosync.GetRegionDistributionByKeyRange(ctx, startKey, endKey, "")
		if err != nil {
			return err
		}
		e.fillDistributionsToChunk(partitionNames[idx], resp.RegionDistributions)
		distributions = append(distributions, resp.RegionDistributions...)
	}
	return nil
}

func (e *ShowExec) fetchShowTableRegions(ctx context.Context) error {
	store := e.Ctx().GetStore()
	tikvStore, ok := store.(helper.Storage)
	if !ok {
		return nil
	}
	splitStore, ok := store.(kv.SplittableStore)
	if !ok {
		return nil
	}

	tb, err := e.getTable()
	if err != nil {
		return errors.Trace(err)
	}

	physicalIDs := []int64{}
	hasGlobalIndex := false
	if pi := tb.Meta().GetPartitionInfo(); pi != nil {
		for _, name := range e.Table.PartitionNames {
			pid, err := tables.FindPartitionByName(tb.Meta(), name.L)
			if err != nil {
				return err
			}
			physicalIDs = append(physicalIDs, pid)
		}
		if len(physicalIDs) == 0 {
			for _, p := range pi.Definitions {
				physicalIDs = append(physicalIDs, p.ID)
			}
		}
		// when table has global index, show the logical table region.
		for _, index := range tb.Meta().Indices {
			if index.Global {
				hasGlobalIndex = true
				break
			}
		}
	} else {
		if len(e.Table.PartitionNames) != 0 {
			return plannererrors.ErrPartitionClauseOnNonpartitioned
		}
		physicalIDs = append(physicalIDs, tb.Meta().ID)
	}

	// Get table regions from pd, not from regionCache, because the region cache maybe outdated.
	var regions []regionMeta
	if len(e.IndexName.L) != 0 {
		// show table * index * region
		indexInfo := tb.Meta().FindIndexByName(e.IndexName.L)
		if indexInfo == nil {
			return plannererrors.ErrKeyDoesNotExist.GenWithStackByArgs(e.IndexName, tb.Meta().Name)
		}
		if indexInfo.Global {
			regions, err = getTableIndexRegions(indexInfo, []int64{tb.Meta().ID}, tikvStore, splitStore)
		} else {
			regions, err = getTableIndexRegions(indexInfo, physicalIDs, tikvStore, splitStore)
		}
	} else {
		// show table * region
		if hasGlobalIndex {
			physicalIDs = append([]int64{tb.Meta().ID}, physicalIDs...)
		}
		regions, err = getTableRegions(tb, physicalIDs, tikvStore, splitStore)
	}
	if err != nil {
		return err
	}

	regionRowItem, err := e.fetchSchedulingInfo(ctx, regions, tb.Meta())
	if err != nil {
		return err
	}

	e.fillRegionsToChunk(regionRowItem)
	return nil
}

func (e *ShowExec) fetchSchedulingInfo(ctx context.Context, regions []regionMeta, tbInfo *model.TableInfo) ([]showTableRegionRowItem, error) {
	scheduleState := make(map[int64]infosync.PlacementScheduleState)
	schedulingConstraints := make(map[int64]*model.PlacementSettings)
	regionRowItem := make([]showTableRegionRowItem, 0)
	tblPlacement, err := e.getTablePlacement(tbInfo)
	if err != nil {
		return nil, err
	}

	if tbInfo.GetPartitionInfo() != nil {
		// partitioned table
		for _, part := range tbInfo.GetPartitionInfo().Definitions {
			_, err = fetchScheduleState(ctx, scheduleState, part.ID)
			if err != nil {
				return nil, err
			}
			placement, err := e.getPolicyPlacement(part.PlacementPolicyRef)
			if err != nil {
				return nil, err
			}
			if placement == nil {
				schedulingConstraints[part.ID] = tblPlacement
			} else {
				schedulingConstraints[part.ID] = placement
			}
		}
	} else {
		// un-partitioned table or index
		schedulingConstraints[tbInfo.ID] = tblPlacement
		_, err = fetchScheduleState(ctx, scheduleState, tbInfo.ID)
		if err != nil {
			return nil, err
		}
	}
	var constraintStr string
	var scheduleStateStr string
	for i := range regions {
		if constraint, ok := schedulingConstraints[regions[i].physicalID]; ok && constraint != nil {
			constraintStr = constraint.String()
			scheduleStateStr = scheduleState[regions[i].physicalID].String()
		} else {
			constraintStr = ""
			scheduleStateStr = ""
		}
		regionRowItem = append(regionRowItem, showTableRegionRowItem{
			regionMeta:            regions[i],
			schedulingConstraints: constraintStr,
			schedulingState:       scheduleStateStr,
		})
	}
	return regionRowItem, nil
}

func getTableRegions(tb table.Table, physicalIDs []int64, tikvStore helper.Storage, splitStore kv.SplittableStore) ([]regionMeta, error) {
	regions := make([]regionMeta, 0, len(physicalIDs))
	uniqueRegionMap := make(map[uint64]struct{})
	for _, id := range physicalIDs {
		rs, err := getPhysicalTableRegions(id, tb.Meta(), tikvStore, splitStore, uniqueRegionMap)
		if err != nil {
			return nil, err
		}
		regions = append(regions, rs...)
	}
	return regions, nil
}

func getTableIndexRegions(indexInfo *model.IndexInfo, physicalIDs []int64, tikvStore helper.Storage, splitStore kv.SplittableStore) ([]regionMeta, error) {
	regions := make([]regionMeta, 0, len(physicalIDs))
	uniqueRegionMap := make(map[uint64]struct{})
	for _, id := range physicalIDs {
		rs, err := getPhysicalIndexRegions(id, indexInfo, tikvStore, splitStore, uniqueRegionMap)
		if err != nil {
			return nil, err
		}
		regions = append(regions, rs...)
	}
	return regions, nil
}

func (e *ShowExec) fillDistributionsToChunk(partitionName string, distributions []*pdHttp.RegionDistribution) {
	for _, dis := range distributions {
		e.result.AppendString(0, partitionName)
		e.result.AppendUint64(1, dis.StoreID)
		e.result.AppendString(2, dis.EngineType)
		e.result.AppendInt64(3, int64(dis.RegionLeaderCount))
		e.result.AppendInt64(4, int64(dis.RegionPeerCount))
		e.result.AppendUint64(5, dis.RegionWriteBytes)
		e.result.AppendUint64(6, dis.RegionWriteKeys)
		e.result.AppendUint64(7, dis.RegionWriteQuery)
		e.result.AppendUint64(8, dis.RegionLeaderReadBytes)
		e.result.AppendUint64(9, dis.RegionLeaderReadKeys)
		e.result.AppendUint64(10, dis.RegionLeaderReadQuery)
		e.result.AppendUint64(11, dis.RegionPeerReadBytes)
		e.result.AppendUint64(12, dis.RegionPeerReadKeys)
		e.result.AppendUint64(13, dis.RegionPeerReadQuery)
	}
}

func (e *ShowExec) fillRegionsToChunk(regions []showTableRegionRowItem) {
	for i := range regions {
		e.result.AppendUint64(0, regions[i].region.Id)
		e.result.AppendString(1, regions[i].start)
		e.result.AppendString(2, regions[i].end)
		e.result.AppendUint64(3, regions[i].leaderID)
		e.result.AppendUint64(4, regions[i].storeID)

		peers := ""
		for i, peer := range regions[i].region.Peers {
			if i > 0 {
				peers += ", "
			}
			peers += strconv.FormatUint(peer.Id, 10)
		}
		e.result.AppendString(5, peers)
		if regions[i].scattering {
			e.result.AppendInt64(6, 1)
		} else {
			e.result.AppendInt64(6, 0)
		}

		e.result.AppendUint64(7, regions[i].writtenBytes)
		e.result.AppendUint64(8, regions[i].readBytes)
		e.result.AppendInt64(9, regions[i].approximateSize)
		e.result.AppendInt64(10, regions[i].approximateKeys)
		e.result.AppendString(11, regions[i].schedulingConstraints)
		e.result.AppendString(12, regions[i].schedulingState)
	}
}

func (e *ShowExec) fetchShowBuiltins() error {
	for _, f := range expression.GetBuiltinList() {
		e.appendRow([]any{f})
	}
	return nil
}

func (e *ShowExec) fetchShowSessionStates(ctx context.Context) error {
	sessionStates := &sessionstates.SessionStates{}
	err := e.Ctx().EncodeStates(ctx, sessionStates)
	if err != nil {
		return err
	}
	stateBytes, err := gjson.Marshal(sessionStates)
	if err != nil {
		return errors.Trace(err)
	}
	stateJSON := types.BinaryJSON{}
	if err = stateJSON.UnmarshalJSON(stateBytes); err != nil {
		return err
	}
	// session token
	var token *sessionstates.SessionToken
	// In testing, user may be nil.
	if user := e.Ctx().GetSessionVars().User; user != nil {
		// The token may be leaked without secure transport, but the cloud can ensure security in some situations,
		// so we don't enforce secure connections.
		if token, err = sessionstates.CreateSessionToken(user.Username); err != nil {
			// Some users deploy TiProxy after the cluster is running and configuring signing certs will restart TiDB.
			// The users may don't need connection migration, e.g. they only want traffic replay, which requires session states
			// but not session tokens. So we don't return errors, just log it.
			logutil.Logger(ctx).Warn("create session token failed", zap.Error(err))
		}
	}
	if token != nil {
		tokenBytes, err := gjson.Marshal(token)
		if err != nil {
			return errors.Trace(err)
		}
		tokenJSON := types.BinaryJSON{}
		if err = tokenJSON.UnmarshalJSON(tokenBytes); err != nil {
			return err
		}
		e.appendRow([]any{stateJSON, tokenJSON})
	} else {
		e.appendRow([]any{stateJSON, nil})
	}
	return nil
}

// FillOneImportJobInfo is exported for testing.
func FillOneImportJobInfo(result *chunk.Chunk, info *importer.JobInfo, runInfo *importinto.RuntimeInfo) {
	fullTableName := utils.EncloseDBAndTable(info.TableSchema, info.TableName)
	result.AppendInt64(0, info.ID)
	result.AppendNull(1) // group key, which will be filled in next PR
	result.AppendString(2, info.Parameters.FileLocation)
	result.AppendString(3, fullTableName)
	result.AppendInt64(4, info.TableID)
	result.AppendString(5, info.Step)
	result.AppendString(6, info.Status)
	result.AppendString(7, units.BytesSize(float64(info.SourceFileSize)))

	if runInfo != nil {
		// running import job
		result.AppendUint64(8, uint64(runInfo.ImportRows))
	} else if info.Status == importer.JobStatusFinished {
		// successful import job
		result.AppendUint64(8, uint64(info.Summary.ImportedRows))
	} else {
		// failed import job
		result.AppendNull(8)
	}

	result.AppendString(9, info.ErrorMessage)
	result.AppendTime(10, info.CreateTime)
	if info.StartTime.IsZero() {
		result.AppendNull(11)
	} else {
		result.AppendTime(11, info.StartTime)
	}
	if info.EndTime.IsZero() {
		result.AppendNull(12)
	} else {
		result.AppendTime(12, info.EndTime)
	}
	result.AppendString(13, info.CreatedBy)

	// For finished job, only keep the update time same as end time
	// and fill other fields with null.
	if runInfo == nil {
		if info.EndTime.IsZero() {
			result.AppendNull(14)
		} else {
			result.AppendTime(14, info.EndTime)
		}
		for i := 15; i < 21; i++ {
			result.AppendNull(i)
		}
		return
	}

	result.AppendTime(14, runInfo.UpdateTime)
	result.AppendString(15, proto.Step2Str(proto.ImportInto, runInfo.Step))
	result.AppendString(16, runInfo.ProcessedSize())
	result.AppendString(17, runInfo.TotalSize())
	result.AppendString(18, runInfo.Percent())
	speed, eta := runInfo.SpeedAndETA()
	result.AppendString(19, speed)
	result.AppendString(20, eta)
}

func handleImportJobInfo(
	ctx context.Context, location *time.Location,
	info *importer.JobInfo, result *chunk.Chunk,
) error {
	var (
		runInfo *importinto.RuntimeInfo
		err     error
	)

	if info.Status == importer.JobStatusRunning {
		// need to get more info from distributed framework for running jobs
		runInfo, err = importinto.GetRuntimeInfoForJob(ctx, location, info.ID)
		if err != nil {
			return err
		}
		// UpdateTime is updated when the job is switching to the next step
		runInfo.StartTime = info.UpdateTime
		if runInfo.Status == proto.TaskStateAwaitingResolution {
			info.Status = string(runInfo.Status)
			info.ErrorMessage = runInfo.ErrorMsg
		}

		failpoint.Inject("mockUpdateTime", func(val failpoint.Value) {
			if v, ok := val.(int); ok {
				ti := time.Now()
				runInfo.StartTime = types.NewTime(types.FromGoTime(ti), mysql.TypeTimestamp, 0)
				runInfo.UpdateTime = types.NewTime(types.FromGoTime(ti.Add(time.Duration(v)*time.Second)), mysql.TypeTimestamp, 0)
			}
		})
	}
	FillOneImportJobInfo(result, info, runInfo)
	return nil
}

const balanceRangeScheduler = "balance-range-scheduler"

func (e *ShowExec) fetchShowDistributionJobs(ctx context.Context) error {
	config, err := infosync.GetSchedulerConfig(ctx, balanceRangeScheduler)
	if err != nil {
		return err
	}
	configs, ok := config.([]any)
	if !ok {
		// it means that no any jobs
		return nil
	}
	jobs := make([]map[string]any, 0, len(configs))
	for _, cfg := range configs {
		job, ok := cfg.(map[string]any)
		if !ok {
			return errs.ErrClientProtoUnmarshal.FastGenByArgs(cfg)
		}
		jobs = append(jobs, job)
	}
	if e.DistributionJobID != nil {
		for _, job := range jobs {
			jobID, ok := job["job-id"].(float64)
			if ok && *e.DistributionJobID == int64(jobID) {
				if err := fillDistributionJobToChunk(ctx, job, e.result); err != nil {
					return err
				}
				break
			}
		}
	} else {
		for _, job := range jobs {
			if err := fillDistributionJobToChunk(ctx, job, e.result); err != nil {
				return err
			}
		}
	}
	return nil
}

// fillDistributionJobToChunk fills the distribution job to the chunk
func fillDistributionJobToChunk(ctx context.Context, job map[string]any, result *chunk.Chunk) error {
	// alias is {db_name}.{table_name}.{partition_name}
	alias := strings.Split(job["alias"].(string), ".")
	logutil.Logger(ctx).Info("fillDistributionJobToChunk", zap.String("alias", job["alias"].(string)))
	if len(alias) != 3 {
		return errs.ErrClientProtoUnmarshal.FastGenByArgs(fmt.Sprintf("alias:%s is invalid", job["alias"].(string)))
	}
	result.AppendUint64(0, uint64(job["job-id"].(float64)))
	result.AppendString(1, alias[0])
	result.AppendString(2, alias[1])
	// partition name maybe empty when the table is not partitioned
	if alias[2] == "" {
		result.AppendNull(3)
	} else {
		result.AppendString(3, alias[2])
	}
	result.AppendString(4, job["engine"].(string))
	result.AppendString(5, job["rule"].(string))
	result.AppendString(6, job["status"].(string))
	timeout := uint64(job["timeout"].(float64))
	result.AppendString(7, time.Duration(timeout).String())
	if create, ok := job["create"]; ok {
		createTime := &time.Time{}
		err := createTime.UnmarshalText([]byte(create.(string)))
		if err != nil {
			return err
		}
		result.AppendTime(8, types.NewTime(types.FromGoTime(*createTime), mysql.TypeDatetime, types.DefaultFsp))
	} else {
		result.AppendNull(8)
	}
	if start, ok := job["start"]; ok {
		startTime := &time.Time{}
		err := startTime.UnmarshalText([]byte(start.(string)))
		if err != nil {
			return err
		}
		result.AppendTime(9, types.NewTime(types.FromGoTime(*startTime), mysql.TypeDatetime, types.DefaultFsp))
	} else {
		result.AppendNull(9)
	}
	if finish, ok := job["finish"]; ok {
		finishedTime := &time.Time{}
		err := finishedTime.UnmarshalText([]byte(finish.(string)))
		if err != nil {
			return err
		}
		result.AppendTime(10, types.NewTime(types.FromGoTime(*finishedTime), mysql.TypeDatetime, types.DefaultFsp))
	} else {
		result.AppendNull(10)
	}
	return nil
}

// fetchShowImportJobs fills the result with the schema:
// {"Job_ID", "Data_Source", "Target_Table", "Table_ID",
// "Phase", "Status", "Source_File_Size", "Imported_Rows",
// "Result_Message", "Create_Time", "Start_Time", "End_Time", "Created_By"}
func (e *ShowExec) fetchShowImportJobs(ctx context.Context) error {
	sctx := e.Ctx()

	var hasSuperPriv bool
	if pm := privilege.GetPrivilegeManager(sctx); pm != nil {
		hasSuperPriv = pm.RequestVerification(sctx.GetSessionVars().ActiveRoles, "", "", "", mysql.SuperPriv)
	}
	// we use sessionCtx from GetTaskManager, user ctx might not have system table privileges.
	taskManager, err := fstorage.GetTaskManager()
	ctx = kv.WithInternalSourceType(ctx, kv.InternalDistTask)
	if err != nil {
		return err
	}

	loc := sctx.GetSessionVars().Location()
	if e.ImportJobID != nil {
		var info *importer.JobInfo
		if err = taskManager.WithNewSession(func(se sessionctx.Context) error {
			exec := se.GetSQLExecutor()
			var err2 error
			info, err2 = importer.GetJob(ctx, exec, *e.ImportJobID, sctx.GetSessionVars().User.String(), hasSuperPriv)
			return err2
		}); err != nil {
			return err
		}
		return handleImportJobInfo(ctx, loc, info, e.result)
	}
	var infos []*importer.JobInfo
	if err = taskManager.WithNewSession(func(se sessionctx.Context) error {
		exec := se.GetSQLExecutor()
		var err2 error
		infos, err2 = importer.GetAllViewableJobs(ctx, exec, sctx.GetSessionVars().User.String(), hasSuperPriv)
		return err2
	}); err != nil {
		return err
	}
	for _, info := range infos {
		if err2 := handleImportJobInfo(ctx, loc, info, e.result); err2 != nil {
			return err2
		}
	}
	// TODO: does not support filtering for now
	return nil
}

// tryFillViewColumnType fill the columns type info of a view.
// Because view's underlying table's column could change or recreate, so view's column type may change over time.
// To avoid this situation we need to generate a logical plan and extract current column types from Schema.
func tryFillViewColumnType(ctx context.Context, sctx sessionctx.Context, is infoschema.InfoSchema, dbName ast.CIStr, tbl *model.TableInfo) error {
	if !tbl.IsView() {
		return nil
	}
	ctx = kv.WithInternalSourceType(context.Background(), kv.InternalTxnOthers)
	// We need to run the build plan process in another session because there may be
	// multiple goroutines running at the same time while session is not goroutine-safe.
	// Take joining system table as an example, `fetchBuildSideRows` and `fetchProbeSideChunks` can be run concurrently.
	return runWithSystemSession(ctx, sctx, func(s sessionctx.Context) error {
		// Retrieve view columns info.
		planBuilder, _ := plannercore.NewPlanBuilder(
			plannercore.PlanBuilderOptNoExecution{}).Init(s.GetPlanCtx(), is, hint.NewQBHintHandler(nil))
		viewLogicalPlan, err := planBuilder.BuildDataSourceFromView(ctx, dbName, tbl, nil, nil)
		if err != nil {
			return err
		}
		viewSchema := viewLogicalPlan.Schema()
		viewOutputNames := viewLogicalPlan.OutputNames()
		for _, col := range tbl.Columns {
			idx := expression.FindFieldNameIdxByColName(viewOutputNames, col.Name.L)
			if idx >= 0 {
				col.FieldType = *viewSchema.Columns[idx].GetType(sctx.GetExprCtx().GetEvalCtx())
			}
			if col.GetType() == mysql.TypeVarString {
				col.SetType(mysql.TypeVarchar)
			}
		}
		return nil
	})
}

func runWithSystemSession(ctx context.Context, sctx sessionctx.Context, fn func(sessionctx.Context) error) error {
	b := exec.NewBaseExecutor(sctx, nil, 0)
	sysCtx, err := b.GetSysSession()
	if err != nil {
		return err
	}
	defer b.ReleaseSysSession(ctx, sysCtx)

	if err = loadSnapshotInfoSchemaIfNeeded(sysCtx, sctx.GetSessionVars().SnapshotTS); err != nil {
		return err
	}
	// `fn` may use KV transaction, so initialize the txn here
	if err = sessiontxn.NewTxn(ctx, sysCtx); err != nil {
		return err
	}
	defer sysCtx.RollbackTxn(ctx)
	if err = ResetContextOfStmt(sysCtx, &ast.SelectStmt{}); err != nil {
		return err
	}
	return fn(sysCtx)
}
