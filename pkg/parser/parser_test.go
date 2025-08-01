// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package parser_test

import (
	"bytes"
	"fmt"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
	. "github.com/pingcap/tidb/pkg/parser/format"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/parser/opcode"
	"github.com/pingcap/tidb/pkg/parser/terror"
	"github.com/pingcap/tidb/pkg/parser/test_driver"
	"github.com/stretchr/testify/require"
)

func TestSimple(t *testing.T) {
	p := parser.New()

	reservedKws := []string{
		"add", "all", "alter", "analyze", "and", "as", "asc", "between", "bigint",
		"binary", "blob", "both", "by", "call", "cascade", "case", "change", "character", "check", "collate",
		"column", "constraint", "convert", "create", "cross", "current_date", "current_time",
		"current_timestamp", "current_user", "database", "databases", "day_hour", "day_microsecond",
		"day_minute", "day_second", "decimal", "default", "delete", "desc", "describe",
		"distinct", "distinctRow", "div", "double", "drop", "dual", "else", "enclosed", "escaped",
		"exists", "explain", "false", "float", "fetch", "for", "force", "foreign", "from",
		"fulltext", "grant", "group", "having", "hour_microsecond", "hour_minute",
		"hour_second", "if", "ignore", "in", "index", "infile", "inner", "insert", "int", "into", "integer",
		"interval", "is", "join", "key", "keys", "kill", "leading", "left", "like", "ilike", "limit", "lines", "load",
		"localtime", "localtimestamp", "lock", "longblob", "longtext", "mediumblob", "maxvalue", "mediumint", "mediumtext",
		"minute_microsecond", "minute_second", "mod", "not", "no_write_to_binlog", "null", "numeric",
		"on", "option", "optionally", "or", "order", "outer", "partition", "precision", "primary", "procedure", "range", "read", "real", "recursive",
		"references", "regexp", "rename", "repeat", "replace", "revoke", "restrict", "right", "rlike",
		"schema", "schemas", "second_microsecond", "select", "set", "show", "smallint",
		"starting", "table", "terminated", "then", "tinyblob", "tinyint", "tinytext", "to",
		"trailing", "true", "union", "unique", "unlock", "unsigned",
		"update", "use", "using", "utc_date", "values", "varbinary", "varchar",
		"when", "where", "write", "xor", "year_month", "zerofill",
		"generated", "virtual", "stored", "usage",
		"delayed", "high_priority", "low_priority",
		"cumeDist", "denseRank", "firstValue", "lag", "lastValue", "lead", "nthValue", "ntile",
		"over", "percentRank", "rank", "row", "rows", "rowNumber", "window", "linear",
		"match", "until", "placement", "tablesample", "failedLoginAttempts", "passwordLockTime",
		// TODO: support the following keywords
		// "with",
	}
	for _, kw := range reservedKws {
		src := fmt.Sprintf("SELECT * FROM db.%s;", kw)
		_, err := p.ParseOneStmt(src, "", "")

		require.NoErrorf(t, err, "source %s", src)

		src = fmt.Sprintf("SELECT * FROM %s.desc", kw)
		_, err = p.ParseOneStmt(src, "", "")
		require.NoErrorf(t, err, "source %s", src)

		src = fmt.Sprintf("SELECT t.%s FROM t", kw)
		_, err = p.ParseOneStmt(src, "", "")
		require.NoErrorf(t, err, "source %s", src)
	}

	// Testcase for unreserved keywords
	unreservedKws := []string{
		"add_columnar_replica_on_demand", "auto_increment", "after", "begin", "bit", "bool", "boolean", "charset", "columns", "commit",
		"date", "datediff", "datetime", "deallocate", "do", "from_days", "end", "engine", "engines", "execute", "extended", "first", "file", "full",
		"local", "names", "offset", "password", "prepare", "quick", "rollback", "savepoint", "session", "signed",
		"start", "global", "tables", "tablespace", "target", "text", "time", "timestamp", "tidb", "transaction", "truncate", "unknown",
		"value", "warnings", "year", "now", "substr", "subpartition", "subpartitions", "substring", "mode", "any", "some", "user", "identified",
		"collation", "comment", "avg_row_length", "checksum", "compression", "connection", "key_block_size",
		"max_rows", "min_rows", "national", "quarter", "escape", "grants", "status", "fields", "triggers", "language",
		"delay_key_write", "isolation", "partitions", "repeatable", "committed", "uncommitted", "only", "serializable", "level",
		"curtime", "variables", "dayname", "version", "btree", "hash", "row_format", "dynamic", "fixed", "compressed",
		"compact", "redundant", "1 sql_no_cache", "1 sql_cache", "action", "round",
		"enable", "disable", "reverse", "space", "privileges", "get_lock", "release_lock", "sleep", "no", "greatest", "least",
		"binlog", "hex", "unhex", "function", "indexes", "from_unixtime", "processlist", "events", "less", "than", "timediff",
		"ln", "log", "log2", "log10", "timestampdiff", "pi", "proxy", "quote", "none", "super", "shared", "exclusive",
		"always", "stats", "stats_meta", "stats_histogram", "stats_buckets", "stats_healthy", "tidb_version", "replication", "slave", "client",
		"max_connections_per_hour", "max_queries_per_hour", "max_updates_per_hour", "max_user_connections", "event", "reload", "routine", "temporary",
		"following", "preceding", "unbounded", "respect", "nulls", "current", "last", "against", "expansion",
		"chain", "error", "general", "nvarchar", "pack_keys", "p", "shard_row_id_bits", "pre_split_regions",
		"constraints", "role", "replicas", "policy", "s3", "strict", "running", "stop", "preserve", "placement", "attributes", "attribute", "resource",
		"burstable", "calibrate", "rollup",
	}
	for _, kw := range unreservedKws {
		src := fmt.Sprintf("SELECT %s FROM tbl;", kw)
		_, err := p.ParseOneStmt(src, "", "")
		require.NoErrorf(t, err, "source %s", src)
	}

	// Testcase for prepared statement
	src := "SELECT id+?, id+? from t;"
	_, err := p.ParseOneStmt(src, "", "")
	require.NoError(t, err)

	// Testcase for -- Comment and unary -- operator
	src = "CREATE TABLE foo (a SMALLINT UNSIGNED, b INT UNSIGNED); -- foo\nSelect --1 from foo;"
	stmts, _, err := p.Parse(src, "", "")
	require.NoError(t, err)
	require.Len(t, stmts, 2)

	// Testcase for /*! xx */
	// See http://dev.mysql.com/doc/refman/5.7/en/comments.html
	// Fix: https://github.com/pingcap/tidb/issues/971
	src = "/*!40101 SET character_set_client = utf8 */;"
	stmts, _, err = p.Parse(src, "", "")
	require.NoError(t, err)
	require.Len(t, stmts, 1)
	stmt := stmts[0]
	_, ok := stmt.(*ast.SetStmt)
	require.True(t, ok)

	// for issue #2017
	src = "insert into blobtable (a) values ('/*! truncated */');"
	stmt, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	is, ok := stmt.(*ast.InsertStmt)
	require.True(t, ok)
	require.Len(t, is.Lists, 1)
	require.Len(t, is.Lists[0], 1)
	require.Equal(t, "/*! truncated */", is.Lists[0][0].(ast.ValueExpr).GetDatumString())

	// Testcase for CONVERT(expr,type)
	src = "SELECT CONVERT('111', SIGNED);"
	st, err := p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	ss, ok := st.(*ast.SelectStmt)
	require.True(t, ok)
	require.Len(t, ss.Fields.Fields, 1)
	cv, ok := ss.Fields.Fields[0].Expr.(*ast.FuncCastExpr)
	require.True(t, ok)
	require.Equal(t, ast.CastConvertFunction, cv.FunctionType)

	// for query start with comment
	srcs := []string{
		"/* some comments */ SELECT CONVERT('111', SIGNED) ;",
		"/* some comments */ /*comment*/ SELECT CONVERT('111', SIGNED) ;",
		"SELECT /*comment*/ CONVERT('111', SIGNED) ;",
		"SELECT CONVERT('111', /*comment*/ SIGNED) ;",
		"SELECT CONVERT('111', SIGNED) /*comment*/;",
	}
	for _, src := range srcs {
		st, err = p.ParseOneStmt(src, "", "")
		require.NoError(t, err)
		_, ok = st.(*ast.SelectStmt)
		require.True(t, ok)
	}

	// for issue #961
	src = "create table t (c int key);"
	st, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	cs, ok := st.(*ast.CreateTableStmt)
	require.True(t, ok)
	require.Len(t, cs.Cols, 1)
	require.Len(t, cs.Cols[0].Options, 1)
	require.Equal(t, ast.ColumnOptionPrimaryKey, cs.Cols[0].Options[0].Tp)

	// for issue #4497
	src = "create table t1(a NVARCHAR(100));"
	_, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)

	// for issue 2803
	src = "use quote;"
	_, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)

	// issue #4354
	src = "select b'';"
	_, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)

	src = "select B'';"
	_, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)

	// src = "select 0b'';"
	// _, err = p.ParseOneStmt(src, "", "")
	// require.Error(t, err)

	// for #4909, support numericType `signed` filedOpt.
	src = "CREATE TABLE t(_sms smallint signed, _smu smallint unsigned);"
	_, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)

	// for #7371, support NATIONAL CHARACTER
	// reference link: https://dev.mysql.com/doc/refman/5.7/en/charset-national.html
	src = "CREATE TABLE t(c1 NATIONAL CHARACTER(10));"
	_, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)

	src = `CREATE TABLE t(a tinyint signed,
		b smallint signed,
		c mediumint signed,
		d int signed,
		e int1 signed,
		f int2 signed,
		g int3 signed,
		h int4 signed,
		i int8 signed,
		j integer signed,
		k bigint signed,
		l bool signed,
		m boolean signed
		);`

	st, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	ct, ok := st.(*ast.CreateTableStmt)
	require.True(t, ok)
	for _, col := range ct.Cols {
		require.Equal(t, uint(0), col.Tp.GetFlag()&mysql.UnsignedFlag)
	}

	// for issue #4006
	src = `insert into tb(v) (select v from tb);`
	_, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)

	// for issue #34642
	src = `SELECT a as c having c = a;`
	_, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)

	// for issue #9823
	src = "SELECT 9223372036854775807;"
	st, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	sel, ok := st.(*ast.SelectStmt)
	require.True(t, ok)
	expr := sel.Fields.Fields[0]
	vExpr := expr.Expr.(*test_driver.ValueExpr)
	require.Equal(t, test_driver.KindInt64, vExpr.Kind())
	src = "SELECT 9223372036854775808;"
	st, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	sel, ok = st.(*ast.SelectStmt)
	require.True(t, ok)
	expr = sel.Fields.Fields[0]
	vExpr = expr.Expr.(*test_driver.ValueExpr)
	require.Equal(t, test_driver.KindUint64, vExpr.Kind())

	src = `select 99e+r10 from t1;`
	st, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	sel, ok = st.(*ast.SelectStmt)
	require.True(t, ok)
	bExpr, ok := sel.Fields.Fields[0].Expr.(*ast.BinaryOperationExpr)
	require.True(t, ok)
	require.Equal(t, opcode.Plus, bExpr.Op)
	require.Equal(t, "99e", bExpr.L.(*ast.ColumnNameExpr).Name.Name.O)
	require.Equal(t, "r10", bExpr.R.(*ast.ColumnNameExpr).Name.Name.O)

	src = `select t./*123*/*,@c3:=0 from t order by t.c1;`
	st, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	sel, ok = st.(*ast.SelectStmt)
	require.True(t, ok)
	require.Equal(t, "t", sel.Fields.Fields[0].WildCard.Table.O)
	varExpr, ok := sel.Fields.Fields[1].Expr.(*ast.VariableExpr)
	require.True(t, ok)
	require.Equal(t, "c3", varExpr.Name)

	src = `select t.1e from test.t;`
	st, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	sel, ok = st.(*ast.SelectStmt)
	require.True(t, ok)
	colExpr, ok := sel.Fields.Fields[0].Expr.(*ast.ColumnNameExpr)
	require.True(t, ok)
	require.Equal(t, "t", colExpr.Name.Table.O)
	require.Equal(t, "1e", colExpr.Name.Name.O)
	tName := sel.From.TableRefs.Left.(*ast.TableSource).Source.(*ast.TableName)
	require.Equal(t, "test", tName.Schema.O)
	require.Equal(t, "t", tName.Name.O)

	src = "select t. `a` > 10 from t;"
	st, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	bExpr, ok = st.(*ast.SelectStmt).Fields.Fields[0].Expr.(*ast.BinaryOperationExpr)
	require.True(t, ok)
	require.Equal(t, opcode.GT, bExpr.Op)
	require.Equal(t, "a", bExpr.L.(*ast.ColumnNameExpr).Name.Name.O)
	require.Equal(t, "t", bExpr.L.(*ast.ColumnNameExpr).Name.Table.O)
	require.Equal(t, int64(10), bExpr.R.(ast.ValueExpr).GetValue().(int64))

	p.SetSQLMode(mysql.ModeANSIQuotes)
	src = `select t."dot"=10 from t;`
	st, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	bExpr, ok = st.(*ast.SelectStmt).Fields.Fields[0].Expr.(*ast.BinaryOperationExpr)
	require.True(t, ok)
	require.Equal(t, opcode.EQ, bExpr.Op)
	require.Equal(t, "dot", bExpr.L.(*ast.ColumnNameExpr).Name.Name.O)
	require.Equal(t, "t", bExpr.L.(*ast.ColumnNameExpr).Name.Table.O)
	require.Equal(t, int64(10), bExpr.R.(ast.ValueExpr).GetValue().(int64))
}

func TestSpecialComments(t *testing.T) {
	p := parser.New()

	// 1. Make sure /*! ... */ respects the same SQL mode.
	_, err := p.ParseOneStmt(`SELECT /*! '\' */;`, "", "")
	require.Error(t, err)

	p.SetSQLMode(mysql.ModeNoBackslashEscapes)
	st, err := p.ParseOneStmt(`SELECT /*! '\' */;`, "", "")
	require.NoError(t, err)
	require.IsType(t, &ast.SelectStmt{}, st)

	// 2. Make sure multiple statements inside /*! ... */ will not crash
	// (this is issue #330)
	stmts, _, err := p.Parse("/*! SET x = 1; SELECT 2 */", "", "")
	require.NoError(t, err)
	require.Len(t, stmts, 2)
	require.IsType(t, &ast.SetStmt{}, stmts[0])
	require.Equal(t, "/*! SET x = 1;", stmts[0].Text())
	require.IsType(t, &ast.SelectStmt{}, stmts[1])
	require.Equal(t, " SELECT 2 */", stmts[1].Text())
	// ^ not sure if correct approach; having multiple statements in MySQL is a syntax error.

	// 3. Make sure invalid text won't cause infinite loop
	// (this is issue #336)
	st, err = p.ParseOneStmt("SELECT /*+ 😅 */ SLEEP(1);", "", "")
	require.NoError(t, err)
	sel, ok := st.(*ast.SelectStmt)
	require.True(t, ok)
	require.Len(t, sel.TableHints, 0)
}

type testCase struct {
	src     string
	ok      bool
	restore string
}

type testErrMsgCase struct {
	src string
	err error
}

func RunTest(t *testing.T, table []testCase, enableWindowFunc bool) {
	p := parser.New()
	p.EnableWindowFunc(enableWindowFunc)
	for _, tbl := range table {
		_, _, err := p.Parse(tbl.src, "", "")
		if !tbl.ok {
			require.Errorf(t, err, "source %v", tbl.src, errors.Trace(err))
			continue
		}
		require.NoErrorf(t, err, "source %v", tbl.src, errors.Trace(err))
		// restore correctness test
		if tbl.ok {
			RunRestoreTest(t, tbl.src, tbl.restore, enableWindowFunc)
		}
	}
}

func RunRestoreTest(t *testing.T, sourceSQLs, expectSQLs string, enableWindowFunc bool) {
	var sb strings.Builder
	p := parser.New()
	p.EnableWindowFunc(enableWindowFunc)
	comment := fmt.Sprintf("source %v", sourceSQLs)
	stmts, _, err := p.Parse(sourceSQLs, "", "")
	require.NoErrorf(t, err, "source %v", sourceSQLs)
	restoreSQLs := ""
	for _, stmt := range stmts {
		sb.Reset()
		err = stmt.Restore(NewRestoreCtx(DefaultRestoreFlags, &sb))
		require.NoError(t, err, comment)
		restoreSQL := sb.String()
		comment = fmt.Sprintf("source %v; restore %v", sourceSQLs, restoreSQL)
		restoreStmt, err := p.ParseOneStmt(restoreSQL, "", "")
		require.NoError(t, err, comment)
		CleanNodeText(stmt)
		CleanNodeText(restoreStmt)
		require.Equal(t, stmt, restoreStmt, comment)
		if restoreSQLs != "" {
			restoreSQLs += "; "
		}
		restoreSQLs += restoreSQL
	}
	require.Equalf(t, expectSQLs, restoreSQLs, "restore %v; expect %v", restoreSQLs, expectSQLs)
}

func RunTestInRealAsFloatMode(t *testing.T, table []testCase, enableWindowFunc bool) {
	p := parser.New()
	p.EnableWindowFunc(enableWindowFunc)
	p.SetSQLMode(mysql.ModeRealAsFloat)
	for _, tbl := range table {
		_, _, err := p.Parse(tbl.src, "", "")
		comment := fmt.Sprintf("source %v", tbl.src)
		if !tbl.ok {
			require.Error(t, err, comment)
			continue
		}
		require.NoError(t, err, comment)
		// restore correctness test
		if tbl.ok {
			RunRestoreTestInRealAsFloatMode(t, tbl.src, tbl.restore, enableWindowFunc)
		}
	}
}

func RunRestoreTestInRealAsFloatMode(t *testing.T, sourceSQLs, expectSQLs string, enableWindowFunc bool) {
	var sb strings.Builder
	p := parser.New()
	p.EnableWindowFunc(enableWindowFunc)
	p.SetSQLMode(mysql.ModeRealAsFloat)
	comment := fmt.Sprintf("source %v", sourceSQLs)
	stmts, _, err := p.Parse(sourceSQLs, "", "")
	require.NoError(t, err, comment)
	restoreSQLs := ""
	for _, stmt := range stmts {
		sb.Reset()
		err = stmt.Restore(NewRestoreCtx(DefaultRestoreFlags, &sb))
		require.NoError(t, err, comment)
		restoreSQL := sb.String()
		comment = fmt.Sprintf("source %v; restore %v", sourceSQLs, restoreSQL)
		restoreStmt, err := p.ParseOneStmt(restoreSQL, "", "")
		require.NoError(t, err, comment)
		CleanNodeText(stmt)
		CleanNodeText(restoreStmt)
		require.Equal(t, stmt, restoreStmt, comment)
		if restoreSQLs != "" {
			restoreSQLs += "; "
		}
		restoreSQLs += restoreSQL
	}
	require.Equal(t, expectSQLs, restoreSQLs, "restore %v; expect %v", restoreSQLs, expectSQLs)
}

func RunErrMsgTest(t *testing.T, table []testErrMsgCase) {
	p := parser.New()
	for _, tbl := range table {
		_, _, err := p.Parse(tbl.src, "", "")
		comment := fmt.Sprintf("source %v", tbl.src)
		if tbl.err != nil {
			require.True(t, terror.ErrorEqual(err, tbl.err), comment)
		} else {
			require.NoError(t, err, comment)
		}
	}
}

func TestRecommendIndex(t *testing.T) {
	table := []testCase{
		{"recommend index run", true, "RECOMMEND INDEX RUN"},
		{"recommend index run with A = 1", true, "RECOMMEND INDEX RUN WITH A = 1"},
		{"recommend index run with A = 1, B = 2", true, "RECOMMEND INDEX RUN WITH A = 1, B = 2"},
		{"recommend index run for 'select * from t where a=1'", true,
			"RECOMMEND INDEX RUN FOR 'select * from t where a=1'"},
		{"recommend index run for 'select * from t where a=1' with A = 1", true,
			"RECOMMEND INDEX RUN FOR 'select * from t where a=1' WITH A = 1"},
		{"recommend index run for 'select * from t where a=1' with A = 1, B = 2", true,
			"RECOMMEND INDEX RUN FOR 'select * from t where a=1' WITH A = 1, B = 2"},
		{"recommend index show option", true, "RECOMMEND INDEX SHOW OPTION"},
		{"recommend index apply 1", true, "RECOMMEND INDEX APPLY 1"},
		{"recommend index ignore 1", true, "RECOMMEND INDEX IGNORE 1"},
		{"recommend index set A = 1", true, "RECOMMEND INDEX SET A = 1"},
		{"recommend index set A = 1, B = 2", true, "RECOMMEND INDEX SET A = 1, B = 2"},
		{"recommend index set A = 1, B = 2, C = 3", true, "RECOMMEND INDEX SET A = 1, B = 2, C = 3"},
	}
	RunTest(t, table, false)
}

func TestAdminStmt(t *testing.T) {
	table := []testCase{
		{"admin show ddl;", true, "ADMIN SHOW DDL"},
		{"admin show ddl jobs;", true, "ADMIN SHOW DDL JOBS"},
		{"admin show ddl jobs where id > 0;", true, "ADMIN SHOW DDL JOBS WHERE `id`>0"},
		{"admin show ddl jobs 20 where id=0;", true, "ADMIN SHOW DDL JOBS 20 WHERE `id`=0"},
		{"admin show ddl jobs -1;", false, ""},
		{"admin show ddl job queries 1", true, "ADMIN SHOW DDL JOB QUERIES 1"},
		{"admin show ddl job queries 1, 2, 3, 4", true, "ADMIN SHOW DDL JOB QUERIES 1, 2, 3, 4"},
		{"admin show ddl job queries limit 5", true, "ADMIN SHOW DDL JOB QUERIES LIMIT 0, 5"},
		{"admin show ddl job queries limit 5, 10", true, "ADMIN SHOW DDL JOB QUERIES LIMIT 5, 10"},
		{"admin show ddl job queries limit 3 offset 2", true, "ADMIN SHOW DDL JOB QUERIES LIMIT 2, 3"},
		{"admin show ddl job queries limit 22 offset 0", true, "ADMIN SHOW DDL JOB QUERIES LIMIT 0, 22"},
		{"admin show t1 next_row_id", true, "ADMIN SHOW `t1` NEXT_ROW_ID"},
		{"admin create workload snapshot;", true, "ADMIN CREATE WORKLOAD SNAPSHOT"},
		{"admin check table t1, t2;", true, "ADMIN CHECK TABLE `t1`, `t2`"},
		{"admin check index tableName idxName;", true, "ADMIN CHECK INDEX `tableName` idxName"},
		{"admin check index tableName idxName (1, 2), (4, 5);", true, "ADMIN CHECK INDEX `tableName` idxName (1,2), (4,5)"},
		{"admin checksum table t1, t2;", true, "ADMIN CHECKSUM TABLE `t1`, `t2`"},
		{"admin cancel ddl jobs 1", true, "ADMIN CANCEL DDL JOBS 1"},
		{"admin cancel ddl jobs 1, 2", true, "ADMIN CANCEL DDL JOBS 1, 2"},
		{"admin pause ddl jobs 1, 3", true, "ADMIN PAUSE DDL JOBS 1, 3"},
		{"admin pause ddl jobs 5", true, "ADMIN PAUSE DDL JOBS 5"},
		{"admin pause ddl jobs", false, "ADMIN PAUSE DDL JOBS"},
		{"admin pause ddl jobs str_not_num", false, "ADMIN PAUSE DDL JOBS str_not_num"},
		{"admin resume ddl jobs 1, 2", true, "ADMIN RESUME DDL JOBS 1, 2"},
		{"admin resume ddl jobs 3", true, "ADMIN RESUME DDL JOBS 3"},
		{"admin resume ddl jobs", false, "ADMIN RESUME DDL JOBS"},
		{"admin resume ddl jobs str_not_num", false, "ADMIN RESUME DDL JOBS str_not_num"},
		{"admin recover index t1 idx_a", true, "ADMIN RECOVER INDEX `t1` idx_a"},
		{"admin cleanup index t1 idx_a", true, "ADMIN CLEANUP INDEX `t1` idx_a"},
		{"admin show slow top 3", true, "ADMIN SHOW SLOW TOP 3"},
		{"admin show slow top internal 7", true, "ADMIN SHOW SLOW TOP INTERNAL 7"},
		{"admin show slow top all 9", true, "ADMIN SHOW SLOW TOP ALL 9"},
		{"admin show slow recent 11", true, "ADMIN SHOW SLOW RECENT 11"},
		{"admin reload expr_pushdown_blacklist", true, "ADMIN RELOAD EXPR_PUSHDOWN_BLACKLIST"},
		{"admin plugins disable audit, whitelist", true, "ADMIN PLUGINS DISABLE audit, whitelist"},
		{"admin plugins enable audit, whitelist", true, "ADMIN PLUGINS ENABLE audit, whitelist"},
		{"admin flush bindings", true, "ADMIN FLUSH BINDINGS"},
		{"admin capture bindings", true, "ADMIN CAPTURE BINDINGS"},
		{"admin evolve bindings", true, "ADMIN EVOLVE BINDINGS"},
		{"admin reload bindings", true, "ADMIN RELOAD BINDINGS"},
		// This case would be removed once TiDB PR to remove ADMIN RELOAD STATISTICS is merged.
		{"admin reload statistics", true, "ADMIN RELOAD STATS_EXTENDED"},
		{"admin reload stats_extended", true, "ADMIN RELOAD STATS_EXTENDED"},
		// Test for 'admin flush plan_cache'
		{"admin flush instance plan_cache", true, "ADMIN FLUSH INSTANCE PLAN_CACHE"},
		{"admin flush session plan_cache", true, "ADMIN FLUSH SESSION PLAN_CACHE"},
		// We do not support the global level. We will check it in the later.
		{"admin flush global plan_cache", true, "ADMIN FLUSH GLOBAL PLAN_CACHE"},
		// for BDR
		{"admin set bdr role primary", true, "ADMIN SET BDR ROLE PRIMARY"},
		{"admin set bdr role secondary", true, "ADMIN SET BDR ROLE SECONDARY"},
		{"admin unset bdr role", true, "ADMIN UNSET BDR ROLE"},
		{"admin show bdr role", true, "ADMIN SHOW BDR ROLE"},
		// for alter job
		{"admin alter ddl jobs 1 thread = 2", true, "ADMIN ALTER DDL JOBS 1 thread = 2"},
		{"admin alter ddl jobs 1 thread = ", false, ""},
		{"admin alter ddl jobs 1 thread", false, ""},
		{"admin alter ddl jobs 1 batch_size = 3", true, "ADMIN ALTER DDL JOBS 1 batch_size = 3"},
		{"admin alter ddl jobs 1 batch_size = ", false, ""},
		{"admin alter ddl jobs 1 batch_size", false, ""},
		{"admin alter ddl jobs 1 max_write_speed = 4", true, "ADMIN ALTER DDL JOBS 1 max_write_speed = 4"},
		{"admin alter ddl jobs 1 max_write_speed = _UTF8MB4'4MiB'", true, "ADMIN ALTER DDL JOBS 1 max_write_speed = _UTF8MB4'4MiB'"},
		{"admin alter ddl jobs 1 max_write_speed = ", false, ""},
		{"admin alter ddl jobs 1 max_write_speed", false, ""},
	}
	RunTest(t, table, false)
}

func TestDMLStmt(t *testing.T) {
	table := []testCase{
		{"", true, ""},
		{";", true, ""},
		{"INSERT INTO foo VALUES (1234)", true, "INSERT INTO `foo` VALUES (1234)"},
		{"INSERT INTO foo VALUES (1234, 5678)", true, "INSERT INTO `foo` VALUES (1234,5678)"},
		{"INSERT INTO t1 (SELECT * FROM t2)", true, "INSERT INTO `t1` (SELECT * FROM `t2`)"},
		{"INSERT INTO t partition (p0) values(1234)", true, "INSERT INTO `t` PARTITION(`p0`) VALUES (1234)"},
		{"REPLACE INTO t partition (p0) values(1234)", true, "REPLACE INTO `t` PARTITION(`p0`) VALUES (1234)"},
		{"INSERT INTO t partition (p0, p1, p2) values(1234)", true, "INSERT INTO `t` PARTITION(`p0`, `p1`, `p2`) VALUES (1234)"},
		{"REPLACE INTO t partition (p0, p1, p2) values(1234)", true, "REPLACE INTO `t` PARTITION(`p0`, `p1`, `p2`) VALUES (1234)"},
		// 15
		{"INSERT INTO foo VALUES (1 || 2)", true, "INSERT INTO `foo` VALUES (1 OR 2)"},
		{"INSERT INTO foo VALUES (1 | 2)", true, "INSERT INTO `foo` VALUES (1|2)"},
		{"INSERT INTO foo VALUES (false || true)", true, "INSERT INTO `foo` VALUES (FALSE OR TRUE)"},
		{"INSERT INTO foo VALUES (bar(5678))", true, "INSERT INTO `foo` VALUES (BAR(5678))"},
		// 20
		{"INSERT INTO foo VALUES ()", true, "INSERT INTO `foo` VALUES ()"},
		{"SELECT * FROM t", true, "SELECT * FROM `t`"},
		{"SELECT * FROM t AS u", true, "SELECT * FROM `t` AS `u`"},
		// 25
		{"SELECT * FROM t, v", true, "SELECT * FROM (`t`) JOIN `v`"},
		{"SELECT * FROM t AS u, v", true, "SELECT * FROM (`t` AS `u`) JOIN `v`"},
		{"SELECT * FROM t, v AS w", true, "SELECT * FROM (`t`) JOIN `v` AS `w`"},
		{"SELECT * FROM t AS u, v AS w", true, "SELECT * FROM (`t` AS `u`) JOIN `v` AS `w`"},
		{"SELECT * FROM foo, bar, foo", true, "SELECT * FROM ((`foo`) JOIN `bar`) JOIN `foo`"},
		// 30
		{"SELECT DISTINCTS * FROM t", false, ""},
		{"SELECT DISTINCT * FROM t", true, "SELECT DISTINCT * FROM `t`"},
		{"SELECT DISTINCTROW * FROM t", true, "SELECT DISTINCT * FROM `t`"},
		{"SELECT ALL * FROM t", true, "SELECT ALL * FROM `t`"},
		{"SELECT DISTINCT ALL * FROM t", false, ""},
		{"SELECT DISTINCTROW ALL * FROM t", false, ""},
		{"INSERT INTO foo (a) VALUES (42)", true, "INSERT INTO `foo` (`a`) VALUES (42)"},
		{"INSERT INTO foo (a,) VALUES (42,)", false, ""},
		// 35
		{"INSERT INTO foo (a,b) VALUES (42,314)", true, "INSERT INTO `foo` (`a`,`b`) VALUES (42,314)"},
		{"INSERT INTO foo (a,b,) VALUES (42,314)", false, ""},
		{"INSERT INTO foo (a,b,) VALUES (42,314,)", false, ""},
		{"INSERT INTO foo () VALUES ()", true, "INSERT INTO `foo` () VALUES ()"},
		{"INSERT INTO foo VALUE ()", true, "INSERT INTO `foo` VALUES ()"},

		// for issue 2402
		{"INSERT INTO tt VALUES (01000001783);", true, "INSERT INTO `tt` VALUES (1000001783)"},
		{"INSERT INTO tt VALUES (default);", true, "INSERT INTO `tt` VALUES (DEFAULT)"},

		{"REPLACE INTO foo VALUES (1 || 2)", true, "REPLACE INTO `foo` VALUES (1 OR 2)"},
		{"REPLACE INTO foo VALUES (1 | 2)", true, "REPLACE INTO `foo` VALUES (1|2)"},
		{"REPLACE INTO foo VALUES (false || true)", true, "REPLACE INTO `foo` VALUES (FALSE OR TRUE)"},
		{"REPLACE INTO foo VALUES (bar(5678))", true, "REPLACE INTO `foo` VALUES (BAR(5678))"},
		{"REPLACE INTO foo VALUES ()", true, "REPLACE INTO `foo` VALUES ()"},
		{"REPLACE INTO foo (a,b) VALUES (42,314)", true, "REPLACE INTO `foo` (`a`,`b`) VALUES (42,314)"},
		{"REPLACE INTO foo (a,b,) VALUES (42,314)", false, ""},
		{"REPLACE INTO foo (a,b,) VALUES (42,314,)", false, ""},
		{"REPLACE INTO foo () VALUES ()", true, "REPLACE INTO `foo` () VALUES ()"},
		{"REPLACE INTO foo VALUE ()", true, "REPLACE INTO `foo` VALUES ()"},
		// 40
		{`SELECT stuff.id
			FROM stuff
			WHERE stuff.value >= ALL (SELECT stuff.value
			FROM stuff)`, true, "SELECT `stuff`.`id` FROM `stuff` WHERE `stuff`.`value`>=ALL (SELECT `stuff`.`value` FROM `stuff`)"},
		{"BEGIN", true, "START TRANSACTION"},
		{"START TRANSACTION", true, "START TRANSACTION"},
		// 45
		{"COMMIT", true, "COMMIT"},
		{"COMMIT AND NO CHAIN", true, "COMMIT"},
		{"COMMIT NO RELEASE", true, "COMMIT"},
		{"COMMIT AND NO CHAIN NO RELEASE", true, "COMMIT"},
		{"COMMIT AND NO CHAIN RELEASE", true, "COMMIT RELEASE"},
		{"COMMIT AND CHAIN NO RELEASE", true, "COMMIT AND CHAIN"},
		{"COMMIT AND CHAIN RELEASE", false, ""},
		{"ROLLBACK", true, "ROLLBACK"},
		{"ROLLBACK AND NO CHAIN", true, "ROLLBACK"},
		{"ROLLBACK NO RELEASE", true, "ROLLBACK"},
		{"ROLLBACK AND NO CHAIN NO RELEASE", true, "ROLLBACK"},
		{"ROLLBACK AND NO CHAIN RELEASE", true, "ROLLBACK RELEASE"},
		{"ROLLBACK AND CHAIN NO RELEASE", true, "ROLLBACK AND CHAIN"},
		{"ROLLBACK AND CHAIN RELEASE", false, ""},
		{`BEGIN;
			INSERT INTO foo VALUES (42, 3.14);
			INSERT INTO foo VALUES (-1, 2.78);
		COMMIT;`, true, "START TRANSACTION; INSERT INTO `foo` VALUES (42,3.14); INSERT INTO `foo` VALUES (-1,2.78); COMMIT"},
		{`BEGIN;
			INSERT INTO tmp SELECT * from bar;
			SELECT * from tmp;
		ROLLBACK;`, true, "START TRANSACTION; INSERT INTO `tmp` SELECT * FROM `bar`; SELECT * FROM `tmp`; ROLLBACK"},
		{"SAVEPOINT x", true, "SAVEPOINT x"},
		{"RELEASE SAVEPOINT x", true, "RELEASE SAVEPOINT x"},
		{"ROLLBACK TO x", true, "ROLLBACK TO x"},
		{"ROLLBACK TO X", true, "ROLLBACK TO X"},
		{"ROLLBACK TO SAVEPOINT x", true, "ROLLBACK TO x"},

		// table statement
		{"TABLE t", true, "TABLE `t`"},
		{"(TABLE t)", true, "(TABLE `t`)"},
		{"TABLE t1, t2", false, ""},
		{"TABLE t ORDER BY b", true, "TABLE `t` ORDER BY `b`"},
		{"TABLE t LIMIT 3", true, "TABLE `t` LIMIT 3"},
		{"TABLE t ORDER BY b LIMIT 3", true, "TABLE `t` ORDER BY `b` LIMIT 3"},
		{"TABLE t ORDER BY b LIMIT 3 OFFSET 2", true, "TABLE `t` ORDER BY `b` LIMIT 2,3"},
		{"TABLE t ORDER BY b LIMIT 2,3", true, "TABLE `t` ORDER BY `b` LIMIT 2,3"},
		{"INSERT INTO ta TABLE tb", true, "INSERT INTO `ta` TABLE `tb`"},
		{"INSERT INTO t.a TABLE t.b", true, "INSERT INTO `t`.`a` TABLE `t`.`b`"},
		{"REPLACE INTO ta TABLE tb", true, "REPLACE INTO `ta` TABLE `tb`"},
		{"REPLACE INTO t.a TABLE t.b", true, "REPLACE INTO `t`.`a` TABLE `t`.`b`"},
		{"TABLE t1 INTO OUTFILE 'a.txt'", true, "TABLE `t1` INTO OUTFILE 'a.txt'"},
		{"TABLE t ORDER BY a INTO OUTFILE '/tmp/abc'", true, "TABLE `t` ORDER BY `a` INTO OUTFILE '/tmp/abc'"},
		{"CREATE TABLE t.a TABLE t.b", true, "CREATE TABLE `t`.`a` AS TABLE `t`.`b`"},
		{"CREATE TABLE ta TABLE tb", true, "CREATE TABLE `ta` AS TABLE `tb`"},
		{"CREATE TABLE ta (x INT) TABLE tb", true, "CREATE TABLE `ta` (`x` INT) AS TABLE `tb`"},
		{"CREATE VIEW v AS TABLE t", true, "CREATE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS TABLE `t`"},
		{"CREATE VIEW v AS (TABLE t)", true, "CREATE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (TABLE `t`)"},
		{"SELECT * FROM t1 WHERE a IN (TABLE t2)", true, "SELECT * FROM `t1` WHERE `a` IN (TABLE `t2`)"},

		// values statement
		{"VALUES ROW(1)", true, "VALUES ROW(1)"},
		{"VALUES ROW()", true, "VALUES ROW()"},
		{"VALUES ROW(1, default)", true, "VALUES ROW(1,DEFAULT)"},
		{"VALUES ROW(1), ROW(2,3)", true, "VALUES ROW(1), ROW(2,3)"},
		{"VALUES (1,2)", false, ""},
		{"VALUES ROW(1,-2,3), ROW(5,7,9), ROW(4,6,8)", true, "VALUES ROW(1,-2,3), ROW(5,7,9), ROW(4,6,8)"},
		{"VALUES ROW(1,s,3.1), ROW(5,y,9.9)", true, "VALUES ROW(1,`s`,3.1), ROW(5,`y`,9.9)"},
		{"VALUES ROW(1,-2,3), ROW(5,7,9), ROW(4,6,8) LIMIT 3", true, "VALUES ROW(1,-2,3), ROW(5,7,9), ROW(4,6,8) LIMIT 3"},
		{"VALUES ROW(1,-2,3), ROW(5,7,9), ROW(4,6,8) ORDER BY a", true, "VALUES ROW(1,-2,3), ROW(5,7,9), ROW(4,6,8) ORDER BY `a`"},
		{"VALUES ROW(1,-2,3), ROW(5,7,9), ROW(4,6,8) ORDER BY a LIMIT 2", true, "VALUES ROW(1,-2,3), ROW(5,7,9), ROW(4,6,8) ORDER BY `a` LIMIT 2"},
		{"VALUES ROW(1,-2,3), ROW(5,7,9) INTO OUTFILE 'a.txt'", true, "VALUES ROW(1,-2,3), ROW(5,7,9) INTO OUTFILE 'a.txt'"},
		{"VALUES ROW(1,-2,3), ROW(5,7,9) ORDER BY a INTO OUTFILE '/tmp/abc'", true, "VALUES ROW(1,-2,3), ROW(5,7,9) ORDER BY `a` INTO OUTFILE '/tmp/abc'"},
		{"CREATE TABLE ta VALUES ROW(1)", true, "CREATE TABLE `ta` AS VALUES ROW(1)"},
		{"CREATE TABLE ta AS VALUES ROW(1)", true, "CREATE TABLE `ta` AS VALUES ROW(1)"},
		{"CREATE VIEW a AS VALUES ROW(1)", true, "CREATE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `a` AS VALUES ROW(1)"},

		// qualified select
		{"SELECT a.b.c FROM t", true, "SELECT `a`.`b`.`c` FROM `t`"},
		{"SELECT a.b.*.c FROM t", false, ""},
		{"SELECT a.b.* FROM t", true, "SELECT `a`.`b`.* FROM `t`"},
		{"SELECT a FROM t", true, "SELECT `a` FROM `t`"},
		{"SELECT a.b.c.d FROM t", false, ""},

		// do statement
		{"DO 1", true, "DO 1"},
		{"DO 1, sleep(1)", true, "DO 1, SLEEP(1)"},
		{"DO 1 from t", false, ""},

		// load data
		{"load data local infile '/tmp/t.csv' into table t1 fields terminated by ',' optionally enclosed by '\"' ignore 1 lines", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t1` FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '\"' IGNORE 1 LINES"},
		{"load data infile '/tmp/t.csv' into table t", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t`"},
		{"load data infile '/tmp/t.csv' into table t character set utf8", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` CHARACTER SET utf8"},
		{"load data infile '/tmp/t.csv' into table t fields terminated by 'ab'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS TERMINATED BY 'ab'"},
		{"load data infile '/tmp/t.csv' into table t columns terminated by 'ab'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS TERMINATED BY 'ab'"},
		{"load data infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b'"},
		{"load data infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b' escaped by '*'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b' ESCAPED BY '*'"},
		{"load data infile '/tmp/t.csv' into table t lines starting by 'ab'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` LINES STARTING BY 'ab'"},
		{"load data infile '/tmp/t.csv' into table t lines starting by 'ab' terminated by 'xy'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` LINES STARTING BY 'ab' TERMINATED BY 'xy'"},
		{"load data infile '/tmp/t.csv' into table t fields terminated by 'ab' lines terminated by 'xy'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS TERMINATED BY 'ab' LINES TERMINATED BY 'xy'"},
		{"load data infile '/tmp/t.csv' into table t terminated by 'xy' fields terminated by 'ab'", false, ""},
		{"load data local infile '/tmp/t.csv' into table t", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t`"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab'", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab'"},
		{"load data local infile '/tmp/t.csv' into table t columns terminated by 'ab'", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab'"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b'", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b'"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b' escaped by '*'", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b' ESCAPED BY '*'"},
		{"load data local infile '/tmp/t.csv' into table t character set utf8 fields terminated by 'ab' enclosed by 'b' escaped by '*'", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` CHARACTER SET utf8 FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b' ESCAPED BY '*'"},
		{"load data local infile '/tmp/t.csv' into table t lines starting by 'ab'", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` LINES STARTING BY 'ab'"},
		{"load data local infile '/tmp/t.csv' into table t lines starting by 'ab' terminated by 'xy'", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` LINES STARTING BY 'ab' TERMINATED BY 'xy'"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' lines terminated by 'xy'", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' LINES TERMINATED BY 'xy'"},
		{"load data local infile '/tmp/t.csv' into table t terminated by 'xy' fields terminated by 'ab'", false, ""},
		{"load data infile '/tmp/t.csv' into table t (a,b)", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t columns terminated by 'ab' (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b' (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b' (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b' escaped by '*' (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b' ESCAPED BY '*' (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t character set utf8 fields terminated by 'ab' enclosed by 'b' escaped by '*' (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` CHARACTER SET utf8 FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b' ESCAPED BY '*' (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t lines starting by 'ab' (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` LINES STARTING BY 'ab' (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t lines starting by 'ab' terminated by 'xy' (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` LINES STARTING BY 'ab' TERMINATED BY 'xy' (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t character set utf8 fields terminated by 'ab' lines terminated by 'xy' (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` CHARACTER SET utf8 FIELDS TERMINATED BY 'ab' LINES TERMINATED BY 'xy' (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' lines terminated by 'xy' (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' LINES TERMINATED BY 'xy' (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t (a,b) fields terminated by 'ab'", false, ""},
		{"load data local infile '/tmp/t.csv' into table t ignore 1 lines", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` IGNORE 1 LINES"},
		{"load data local infile '/tmp/t.csv' into table t ignore -1 lines", false, ""},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b' (a,b) ignore 1 lines", false, ""},
		{"load data local infile '/tmp/t.csv' into table t lines starting by 'ab' terminated by 'xy' ignore 1 lines", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` LINES STARTING BY 'ab' TERMINATED BY 'xy' IGNORE 1 LINES"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b' escaped by '*' ignore 1 lines (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b' ESCAPED BY '*' IGNORE 1 LINES (`a`,`b`)"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b' escaped by ''", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b' ESCAPED BY ''"},
		{"load data local infile '~/1.csv' into table `t_ascii` fields terminated by X'6B6B';", true, "LOAD DATA LOCAL INFILE '~/1.csv' IGNORE INTO TABLE `t_ascii` FIELDS TERMINATED BY 'kk'"},
		{"load data local infile '~/1.csv' into table `t_ascii` fields terminated by X'6B6B' enclosed by X'0D';", true, "LOAD DATA LOCAL INFILE '~/1.csv' IGNORE INTO TABLE `t_ascii` FIELDS TERMINATED BY 'kk' ENCLOSED BY '\r'"},
		{"load data local infile '~/1.csv' into table `t_ascii` fields terminated by X'6B6B' enclosed by X'0D0D';", false, ""},
		{"load data local infile '~/1.csv' into table `t_ascii` fields terminated by B'110101101101011';", true, "LOAD DATA LOCAL INFILE '~/1.csv' IGNORE INTO TABLE `t_ascii` FIELDS TERMINATED BY 'kk'"},
		{"load data local infile '~/1.csv' into table `t_ascii` fields terminated by B'110101101101011' enclosed by B'1101';", true, "LOAD DATA LOCAL INFILE '~/1.csv' IGNORE INTO TABLE `t_ascii` FIELDS TERMINATED BY 'kk' ENCLOSED BY '\r'"},
		{"load data local infile '~/1.csv' into table `t_ascii` fields terminated by B'110101101101011' enclosed by B'110100001101';", false, ""},
		{"load data local infile '~/1.csv' into table `t_ascii` lines starting by B'110101101101011' terminated by B'110101101101011';", true, "LOAD DATA LOCAL INFILE '~/1.csv' IGNORE INTO TABLE `t_ascii` LINES STARTING BY 'kk' TERMINATED BY 'kk'"},
		{"load data local infile '~/1.csv' into table `t_ascii` lines starting by X'6B6B' terminated by X'6B6B';", true, "LOAD DATA LOCAL INFILE '~/1.csv' IGNORE INTO TABLE `t_ascii` LINES STARTING BY 'kk' TERMINATED BY 'kk'"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' enclosed by 'b' enclosed by 'b'", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b'"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' escaped by '' enclosed by 'b'", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b' ESCAPED BY ''"},
		{"load data local infile '/tmp/t.csv' into table t fields terminated by 'ab' escaped by '' enclosed by 'b' SET b = CAST(CONV(MID(@var1, 3, LENGTH(@var1)-3), 2, 10) AS UNSIGNED)", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t` FIELDS TERMINATED BY 'ab' ENCLOSED BY 'b' ESCAPED BY '' SET `b`=CAST(CONV(MID(@`var1`, 3, LENGTH(@`var1`)-3), 2, 10) AS UNSIGNED)"},

		{"load data infile '/tmp/t.csv' into table t fields enclosed by ''", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS ENCLOSED BY ''"},
		{"load data infile '/tmp/t.csv' into table t fields enclosed by 'a'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS ENCLOSED BY 'a'"},
		{"load data infile '/tmp/t.csv' into table t fields enclosed by 'aa'", false, ""},
		{"load data infile '/tmp/t.csv' into table t fields escaped by ''", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS ESCAPED BY ''"},
		{"load data infile '/tmp/t.csv' into table t fields escaped by 'a'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS ESCAPED BY 'a'"},
		{"load data infile '/tmp/t.csv' into table t fields escaped by 'aa'", false, ""},

		{"LOAD DATA INFILE 'file.txt' INTO TABLE t1 (column1, @dummy, column2, @dummy, column3)", true, "LOAD DATA INFILE 'file.txt' INTO TABLE `t1` (`column1`,@`dummy`,`column2`,@`dummy`,`column3`)"},
		{"LOAD DATA INFILE 'file.txt' INTO TABLE t1 (column1, @var1) SET column2 = @var1/100", true, "LOAD DATA INFILE 'file.txt' INTO TABLE `t1` (`column1`,@`var1`) SET `column2`=@`var1`/100"},
		{"LOAD DATA INFILE 'file.txt' INTO TABLE t1 (column1, @var1, @var2) SET column2 = @var1/100, column3 = DEFAULT, column4=CURRENT_TIMESTAMP, column5=@var2+1", true, "LOAD DATA INFILE 'file.txt' INTO TABLE `t1` (`column1`,@`var1`,@`var2`) SET `column2`=@`var1`/100, `column3`=DEFAULT, `column4`=CURRENT_TIMESTAMP(), `column5`=@`var2`+1"},

		{"LOAD DATA INFILE '/tmp/t.csv' INTO TABLE t1 FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n';", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t1` FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n'"},
		{"LOAD DATA LOCAL INFILE '/tmp/t.csv' INTO TABLE t1 FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n';", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t1` FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n'"},
		{"LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE t1 FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n';", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' IGNORE INTO TABLE `t1` FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n'"},
		{"LOAD DATA LOCAL INFILE '/tmp/t.csv' REPLACE INTO TABLE t1 FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n';", true, "LOAD DATA LOCAL INFILE '/tmp/t.csv' REPLACE INTO TABLE `t1` FIELDS TERMINATED BY ',' LINES TERMINATED BY '\n'"},

		{"load data infile 's3://bucket-name/t.csv' into table t", true, "LOAD DATA INFILE 's3://bucket-name/t.csv' INTO TABLE `t`"},
		{"load data infile '/tmp/t.csv' into table t fields defined null by 'nil'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS DEFINED NULL BY 'nil'"},
		{"load data infile '/tmp/t.csv' into table t fields defined null by X'00'", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS DEFINED NULL BY '\x00'"},
		{"load data infile '/tmp/t.csv' into table t fields defined null by 'NULL' optionally enclosed ignore 1 lines", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` FIELDS DEFINED NULL BY 'NULL' OPTIONALLY ENCLOSED IGNORE 1 LINES"},
		{"load data infile '/tmp/t.csv' format 'delimited data' into table t (column1, @var1) SET column2 = @var1/100", true, "LOAD DATA INFILE '/tmp/t.csv' FORMAT 'delimited data' INTO TABLE `t` (`column1`,@`var1`) SET `column2`=@`var1`/100"},
		{"load data local infile '/tmp/t.sql' format 'sql file' replace into table t (a,b)", true, "LOAD DATA LOCAL INFILE '/tmp/t.sql' FORMAT 'sql file' REPLACE INTO TABLE `t` (`a`,`b`)"},
		{"load data infile '/tmp/t.parquet' format 'parquet' into table t (column1, @var1) SET column2 = @var1/100", true, "LOAD DATA INFILE '/tmp/t.parquet' FORMAT 'parquet' INTO TABLE `t` (`column1`,@`var1`) SET `column2`=@`var1`/100"},

		{"load data infile '/tmp/t.csv' into table t with detached", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` WITH detached"},
		// we must add "`" to table name, since the offset of restored sql might be different with the source
		{"load data infile '/tmp/t.csv' into table `t` with threads=10", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` WITH threads=10"},
		{"load data infile '/tmp/t.csv' into table `t` with threads=10, detached", true, "LOAD DATA INFILE '/tmp/t.csv' INTO TABLE `t` WITH threads=10, detached"},

		// IMPORT INTO
		{"import into t from '/file.csv'", true, "IMPORT INTO `t` FROM '/file.csv'"},
		{"import into t (a,b) from '/file.csv'", true, "IMPORT INTO `t` (`a`,`b`) FROM '/file.csv'"},
		{"import into t (a,@1) from '/file.csv'", true, "IMPORT INTO `t` (`a`,@`1`) FROM '/file.csv'"},
		{"import into t (a,@1) set b=@1+100 from '/file.csv'", true, "IMPORT INTO `t` (`a`,@`1`) SET `b`=@`1`+100 FROM '/file.csv'"},
		{"import into t from '/file.csv' format 'sql file'", true, "IMPORT INTO `t` FROM '/file.csv' FORMAT 'sql file'"},
		{"import into t from '/file.csv' with detached", true, "IMPORT INTO `t` FROM '/file.csv' WITH detached"},
		{"import into `t` from '/file.csv' with thread=1", true, "IMPORT INTO `t` FROM '/file.csv' WITH thread=1"},
		{"import into `t` from '/file.csv' with detached, thread=1", true, "IMPORT INTO `t` FROM '/file.csv' WITH detached, thread=1"},

		// select for update/share
		{"select * from t for update", true, "SELECT * FROM `t` FOR UPDATE"},
		{"select * from t for share", true, "SELECT * FROM `t` FOR SHARE"},
		{"select * from t for update nowait", true, "SELECT * FROM `t` FOR UPDATE NOWAIT"},
		{"select * from t for update wait 5", true, "SELECT * FROM `t` FOR UPDATE WAIT 5"},
		{"select * from t limit 1 for update wait 11", true, "SELECT * FROM `t` LIMIT 1 FOR UPDATE WAIT 11"},
		{"select * from t for share nowait", true, "SELECT * FROM `t` FOR SHARE NOWAIT"},
		{"select * from t for update skip locked", true, "SELECT * FROM `t` FOR UPDATE SKIP LOCKED"},
		{"select * from t for share skip locked", true, "SELECT * FROM `t` FOR SHARE SKIP LOCKED"},
		{"select * from t lock in share mode", true, "SELECT * FROM `t` FOR SHARE"},
		{"select * from t lock in share mode nowait", false, ""},
		{"select * from t lock in share mode skip locked", false, ""},

		{"select * from t for update of t", true, "SELECT * FROM `t` FOR UPDATE OF `t`"},
		{"select * from t for share of t", true, "SELECT * FROM `t` FOR SHARE OF `t`"},
		{"select * from t for update of t nowait", true, "SELECT * FROM `t` FOR UPDATE OF `t` NOWAIT"},
		{"select * from t for update of t wait 5", true, "SELECT * FROM `t` FOR UPDATE OF `t` WAIT 5"},
		{"select * from t limit 1 for update of t wait 11", true, "SELECT * FROM `t` LIMIT 1 FOR UPDATE OF `t` WAIT 11"},
		{"select * from t for share of t nowait", true, "SELECT * FROM `t` FOR SHARE OF `t` NOWAIT"},
		{"select * from t for update of t skip locked", true, "SELECT * FROM `t` FOR UPDATE OF `t` SKIP LOCKED"},
		{"select * from t for share of t skip locked", true, "SELECT * FROM `t` FOR SHARE OF `t` SKIP LOCKED"},

		// select into outfile
		{"select a, b from t into outfile '/tmp/result.txt'", true, "SELECT `a`,`b` FROM `t` INTO OUTFILE '/tmp/result.txt'"},
		{"select a from t order by a into outfile '/tmp/abc'", true, "SELECT `a` FROM `t` ORDER BY `a` INTO OUTFILE '/tmp/abc'"},
		{"select 1 into outfile '/tmp/1.csv'", true, "SELECT 1 INTO OUTFILE '/tmp/1.csv'"},
		{"select 1 for update into outfile '/tmp/1.csv'", true, "SELECT 1 FOR UPDATE INTO OUTFILE '/tmp/1.csv'"},
		{"select a,b,a+b from t into outfile '/tmp/result.txt' fields terminated BY ','", true, "SELECT `a`,`b`,`a`+`b` FROM `t` INTO OUTFILE '/tmp/result.txt' FIELDS TERMINATED BY ','"},
		{"select a,b,a+b from t into outfile '/tmp/result.txt' fields terminated BY ',' enclosed BY '\"'", true, "SELECT `a`,`b`,`a`+`b` FROM `t` INTO OUTFILE '/tmp/result.txt' FIELDS TERMINATED BY ',' ENCLOSED BY '\"'"},
		{"select a,b,a+b from t into outfile '/tmp/result.txt' fields terminated BY ',' optionally enclosed BY '\"'", true, "SELECT `a`,`b`,`a`+`b` FROM `t` INTO OUTFILE '/tmp/result.txt' FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '\"'"},
		{"select a,b,a+b from t into outfile '/tmp/result.txt' lines terminated BY '\n'", true, "SELECT `a`,`b`,`a`+`b` FROM `t` INTO OUTFILE '/tmp/result.txt' LINES TERMINATED BY '\n'"},
		{"select a,b,a+b from t into outfile '/tmp/result.txt' fields terminated BY ',' optionally enclosed BY '\"' lines terminated BY '\r'", true, "SELECT `a`,`b`,`a`+`b` FROM `t` INTO OUTFILE '/tmp/result.txt' FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '\"' LINES TERMINATED BY '\r'"},
		{"select a,b,a+b from t into outfile '/tmp/result.txt' fields terminated BY ',' enclosed BY '\"' lines terminated BY '\r'", true, "SELECT `a`,`b`,`a`+`b` FROM `t` INTO OUTFILE '/tmp/result.txt' FIELDS TERMINATED BY ',' ENCLOSED BY '\"' LINES TERMINATED BY '\r'"},
		{"select a,b,a+b from t into outfile '/tmp/result.txt' fields terminated BY ',' optionally enclosed BY '\"' lines starting by 'xy' terminated BY '\r'", true, "SELECT `a`,`b`,`a`+`b` FROM `t` INTO OUTFILE '/tmp/result.txt' FIELDS TERMINATED BY ',' OPTIONALLY ENCLOSED BY '\"' LINES STARTING BY 'xy' TERMINATED BY '\r'"},
		{"select a,b,a+b from t into outfile '/tmp/result.txt' fields terminated BY ',' enclosed BY '\"' lines starting by 'xy' terminated BY '\r'", true, "SELECT `a`,`b`,`a`+`b` FROM `t` INTO OUTFILE '/tmp/result.txt' FIELDS TERMINATED BY ',' ENCLOSED BY '\"' LINES STARTING BY 'xy' TERMINATED BY '\r'"},

		// from join
		{"SELECT * from t1, t2, t3", true, "SELECT * FROM ((`t1`) JOIN `t2`) JOIN `t3`"},
		{"select * from t1 join t2 left join t3 on t2.id = t3.id", true, "SELECT * FROM (`t1` JOIN `t2`) LEFT JOIN `t3` ON `t2`.`id`=`t3`.`id`"},
		{"select * from t1 right join t2 on t1.id = t2.id left join t3 on t3.id = t2.id", true, "SELECT * FROM (`t1` RIGHT JOIN `t2` ON `t1`.`id`=`t2`.`id`) LEFT JOIN `t3` ON `t3`.`id`=`t2`.`id`"},
		{"select * from t1 right join t2 on t1.id = t2.id left join t3", false, ""},
		{"select * from t1 join t2 left join t3 using (id)", true, "SELECT * FROM (`t1` JOIN `t2`) LEFT JOIN `t3` USING (`id`)"},
		{"select * from t1 right join t2 using (id) left join t3 using (id)", true, "SELECT * FROM (`t1` RIGHT JOIN `t2` USING (`id`)) LEFT JOIN `t3` USING (`id`)"},
		{"select * from t1 right join t2 using (id) left join t3", false, ""},
		{"select * from t1 natural join t2", true, "SELECT * FROM `t1` NATURAL JOIN `t2`"},
		{"select * from t1 natural right join t2", true, "SELECT * FROM `t1` NATURAL RIGHT JOIN `t2`"},
		{"select * from t1 natural left outer join t2", true, "SELECT * FROM `t1` NATURAL LEFT JOIN `t2`"},
		{"select * from t1 natural inner join t2", false, ""},
		{"select * from t1 natural cross join t2", false, ""},
		{"select * from t3 join t1 join t2 on t1.a=t2.a on t3.b=t2.b", true, "SELECT * FROM `t3` JOIN (`t1` JOIN `t2` ON `t1`.`a`=`t2`.`a`) ON `t3`.`b`=`t2`.`b`"},

		// for straight_join
		{"select * from t1 straight_join t2 on t1.id = t2.id", true, "SELECT * FROM `t1` STRAIGHT_JOIN `t2` ON `t1`.`id`=`t2`.`id`"},
		{"select straight_join * from t1 join t2 on t1.id = t2.id", true, "SELECT STRAIGHT_JOIN * FROM `t1` JOIN `t2` ON `t1`.`id`=`t2`.`id`"},
		{"select straight_join * from t1 left join t2 on t1.id = t2.id", true, "SELECT STRAIGHT_JOIN * FROM `t1` LEFT JOIN `t2` ON `t1`.`id`=`t2`.`id`"},
		{"select straight_join * from t1 right join t2 on t1.id = t2.id", true, "SELECT STRAIGHT_JOIN * FROM `t1` RIGHT JOIN `t2` ON `t1`.`id`=`t2`.`id`"},
		{"select straight_join * from t1 straight_join t2 on t1.id = t2.id", true, "SELECT STRAIGHT_JOIN * FROM `t1` STRAIGHT_JOIN `t2` ON `t1`.`id`=`t2`.`id`"},

		// delete statement
		// single table syntax
		{"DELETE from t1", true, "DELETE FROM `t1`"},
		{"DELETE from t1.*", false, ""},
		{"DELETE LOW_priORITY from t1", true, "DELETE LOW_PRIORITY FROM `t1`"},
		{"DELETE quick from t1", true, "DELETE QUICK FROM `t1`"},
		{"DELETE ignore from t1", true, "DELETE IGNORE FROM `t1`"},
		{"DELETE low_priority quick ignore from t1", true, "DELETE LOW_PRIORITY QUICK IGNORE FROM `t1`"},
		{"DELETE FROM t1 WHERE t1.a > 0 ORDER BY t1.a", true, "DELETE FROM `t1` WHERE `t1`.`a`>0 ORDER BY `t1`.`a`"},
		{"delete from t1 where a=26", true, "DELETE FROM `t1` WHERE `a`=26"},
		{"DELETE from t1 where a=1 limit 1", true, "DELETE FROM `t1` WHERE `a`=1 LIMIT 1"},
		{"DELETE FROM t1 WHERE t1.a > 0 ORDER BY t1.a LIMIT 1", true, "DELETE FROM `t1` WHERE `t1`.`a`>0 ORDER BY `t1`.`a` LIMIT 1"},
		{"DELETE FROM x.y z WHERE z.a > 0", true, "DELETE FROM `x`.`y` AS `z` WHERE `z`.`a`>0"},
		{"DELETE FROM t1 AS w WHERE a > 0", true, "DELETE FROM `t1` AS `w` WHERE `a`>0"},
		{"DELETE from t1 partition (p0,p1)", true, "DELETE FROM `t1` PARTITION(`p0`, `p1`)"},

		// multi table syntax: before from
		{"delete low_priority t1, t2 from t1, t2", true, "DELETE LOW_PRIORITY `t1`,`t2` FROM (`t1`) JOIN `t2`"},
		{"delete quick t1, t2 from t1, t2", true, "DELETE QUICK `t1`,`t2` FROM (`t1`) JOIN `t2`"},
		{"delete ignore t1, t2 from t1, t2", true, "DELETE IGNORE `t1`,`t2` FROM (`t1`) JOIN `t2`"},
		{"delete ignore t1, t2 from t1 partition (p0,p1), t2", true, "DELETE IGNORE `t1`,`t2` FROM (`t1` PARTITION(`p0`, `p1`)) JOIN `t2`"},
		{"delete low_priority quick ignore t1, t2 from t1, t2 where t1.a > 5", true, "DELETE LOW_PRIORITY QUICK IGNORE `t1`,`t2` FROM (`t1`) JOIN `t2` WHERE `t1`.`a`>5"},
		{"delete t1, t2 from t1, t2", true, "DELETE `t1`,`t2` FROM (`t1`) JOIN `t2`"},
		{"delete t1, t2 from t1, t2 where t1.a = 1 and t2.b <> 1", true, "DELETE `t1`,`t2` FROM (`t1`) JOIN `t2` WHERE `t1`.`a`=1 AND `t2`.`b`!=1"},
		{"delete t1 from t1, t2", true, "DELETE `t1` FROM (`t1`) JOIN `t2`"},
		{"delete t2 from t1, t2", true, "DELETE `t2` FROM (`t1`) JOIN `t2`"},
		{"delete t1 from t1", true, "DELETE `t1` FROM `t1`"},
		{"delete t1,t2,t3 from t1, t2, t3", true, "DELETE `t1`,`t2`,`t3` FROM ((`t1`) JOIN `t2`) JOIN `t3`"},
		{"delete t1,t2,t3 from t1, t2, t3 where t3.c < 5 and t1.a = 3", true, "DELETE `t1`,`t2`,`t3` FROM ((`t1`) JOIN `t2`) JOIN `t3` WHERE `t3`.`c`<5 AND `t1`.`a`=3"},
		{"delete t1 from t1, t1 as t2 where t1.b = t2.b and t1.a > t2.a", true, "DELETE `t1` FROM (`t1`) JOIN `t1` AS `t2` WHERE `t1`.`b`=`t2`.`b` AND `t1`.`a`>`t2`.`a`"},
		{"delete t1.*,t2 from t1, t2", true, "DELETE `t1`,`t2` FROM (`t1`) JOIN `t2`"},
		{"delete t.t1.*,t2 from t1, t2", true, "DELETE `t`.`t1`,`t2` FROM (`t1`) JOIN `t2`"},
		{"delete t1.*, t2.* from t1, t2", true, "DELETE `t1`,`t2` FROM (`t1`) JOIN `t2`"},
		{"delete t11.*, t12.* from t11, t12 where t11.a = t12.a and t11.b <> 1", true, "DELETE `t11`,`t12` FROM (`t11`) JOIN `t12` WHERE `t11`.`a`=`t12`.`a` AND `t11`.`b`!=1"},

		// multi table syntax: with using
		{"DELETE quick FROM t1,t2 USING t1,t2", true, "DELETE QUICK FROM `t1`,`t2` USING (`t1`) JOIN `t2`"},
		{"DELETE low_priority ignore FROM t1,t2 USING t1,t2", true, "DELETE LOW_PRIORITY IGNORE FROM `t1`,`t2` USING (`t1`) JOIN `t2`"},
		{"DELETE low_priority quick ignore FROM t1,t2 USING t1,t2", true, "DELETE LOW_PRIORITY QUICK IGNORE FROM `t1`,`t2` USING (`t1`) JOIN `t2`"},
		{"DELETE FROM t1 USING t1 WHERE post='1'", true, "DELETE FROM `t1` USING `t1` WHERE `post`=_UTF8MB4'1'"},
		{"DELETE FROM t1,t2 USING t1,t2", true, "DELETE FROM `t1`,`t2` USING (`t1`) JOIN `t2`"},
		{"DELETE FROM t1,t2,t3 USING t1,t2,t3 where t3.a = 1", true, "DELETE FROM `t1`,`t2`,`t3` USING ((`t1`) JOIN `t2`) JOIN `t3` WHERE `t3`.`a`=1"},
		{"DELETE FROM t2,t3 USING t1,t2,t3 where t1.a = 1", true, "DELETE FROM `t2`,`t3` USING ((`t1`) JOIN `t2`) JOIN `t3` WHERE `t1`.`a`=1"},
		{"DELETE FROM t2.*,t3.* USING t1,t2,t3 where t1.a = 1", true, "DELETE FROM `t2`,`t3` USING ((`t1`) JOIN `t2`) JOIN `t3` WHERE `t1`.`a`=1"},
		{"DELETE FROM t1,t2.*,t3.* USING t1,t2,t3 where t1.a = 1", true, "DELETE FROM `t1`,`t2`,`t3` USING ((`t1`) JOIN `t2`) JOIN `t3` WHERE `t1`.`a`=1"},

		// for delete statement
		{"DELETE t1, t2 FROM t1 INNER JOIN t2 INNER JOIN t3 WHERE t1.id=t2.id AND t2.id=t3.id;", true, "DELETE `t1`,`t2` FROM (`t1` JOIN `t2`) JOIN `t3` WHERE `t1`.`id`=`t2`.`id` AND `t2`.`id`=`t3`.`id`"},
		{"DELETE FROM t1, t2 USING t1 INNER JOIN t2 INNER JOIN t3 WHERE t1.id=t2.id AND t2.id=t3.id;", true, "DELETE FROM `t1`,`t2` USING (`t1` JOIN `t2`) JOIN `t3` WHERE `t1`.`id`=`t2`.`id` AND `t2`.`id`=`t3`.`id`"},
		// for optimizer hint in delete statement
		{"DELETE /*+ TiDB_INLJ(t1, t2) */ t1, t2 from t1, t2 where t1.id=t2.id;", true, "DELETE /*+ TIDB_INLJ(`t1`, `t2`)*/ `t1`,`t2` FROM (`t1`) JOIN `t2` WHERE `t1`.`id`=`t2`.`id`"},
		{"DELETE /*+ TiDB_HJ(t1, t2) */ t1, t2 from t1, t2 where t1.id=t2.id", true, "DELETE /*+ TIDB_HJ(`t1`, `t2`)*/ `t1`,`t2` FROM (`t1`) JOIN `t2` WHERE `t1`.`id`=`t2`.`id`"},
		{"DELETE /*+ TiDB_SMJ(t1, t2) */ t1, t2 from t1, t2 where t1.id=t2.id", true, "DELETE /*+ TIDB_SMJ(`t1`, `t2`)*/ `t1`,`t2` FROM (`t1`) JOIN `t2` WHERE `t1`.`id`=`t2`.`id`"},
		// for "USE INDEX" in delete statement
		{"DELETE FROM t1 USE INDEX(idx_a) WHERE t1.id=1;", true, "DELETE FROM `t1` USE INDEX (`idx_a`) WHERE `t1`.`id`=1"},
		{"DELETE t1, t2 FROM t1 USE INDEX(idx_a) JOIN t2 WHERE t1.id=t2.id;", true, "DELETE `t1`,`t2` FROM `t1` USE INDEX (`idx_a`) JOIN `t2` WHERE `t1`.`id`=`t2`.`id`"},
		{"DELETE t1, t2 FROM t1 USE INDEX(idx_a) JOIN t2 USE INDEX(idx_a) WHERE t1.id=t2.id;", true, "DELETE `t1`,`t2` FROM `t1` USE INDEX (`idx_a`) JOIN `t2` USE INDEX (`idx_a`) WHERE `t1`.`id`=`t2`.`id`"},

		// for fail case
		{"DELETE t1, t2 FROM t1 INNER JOIN t2 INNER JOIN t3 WHERE t1.id=t2.id AND t2.id=t3.id limit 10;", false, ""},
		{"DELETE t1, t2 FROM t1 INNER JOIN t2 INNER JOIN t3 WHERE t1.id=t2.id AND t2.id=t3.id order by t1.id;", false, ""},

		// for on duplicate key update
		{"INSERT INTO t (a,b,c) VALUES (1,2,3),(4,5,6) ON DUPLICATE KEY UPDATE c=VALUES(a)+VALUES(b);", true, "INSERT INTO `t` (`a`,`b`,`c`) VALUES (1,2,3),(4,5,6) ON DUPLICATE KEY UPDATE `c`=VALUES(`a`)+VALUES(`b`)"},
		{"INSERT INTO t (a,b,c) VALUES (1,2,3),(4,5,6) ON DUPLICATE KEY UPDATE c:=VALUES(a)+VALUES(b);", true, "INSERT INTO `t` (`a`,`b`,`c`) VALUES (1,2,3),(4,5,6) ON DUPLICATE KEY UPDATE `c`=VALUES(`a`)+VALUES(`b`)"},
		{"INSERT IGNORE INTO t (a,b,c) VALUES (1,2,3),(4,5,6) ON DUPLICATE KEY UPDATE c=VALUES(a)+VALUES(b);", true, "INSERT IGNORE INTO `t` (`a`,`b`,`c`) VALUES (1,2,3),(4,5,6) ON DUPLICATE KEY UPDATE `c`=VALUES(`a`)+VALUES(`b`)"},
		{"INSERT IGNORE INTO t (a,b,c) VALUES (1,2,3),(4,5,6) ON DUPLICATE KEY UPDATE c:=VALUES(a)+VALUES(b);", true, "INSERT IGNORE INTO `t` (`a`,`b`,`c`) VALUES (1,2,3),(4,5,6) ON DUPLICATE KEY UPDATE `c`=VALUES(`a`)+VALUES(`b`)"},

		// for insert ... set
		{"INSERT INTO t SET a=1,b=2", true, "INSERT INTO `t` SET `a`=1,`b`=2"},
		{"INSERT INTO t (a) SET a=1", false, ""},

		// for update statement
		{"UPDATE LOW_PRIORITY IGNORE t SET id = id + 1 ORDER BY id DESC;", true, "UPDATE LOW_PRIORITY IGNORE `t` SET `id`=`id`+1 ORDER BY `id` DESC"},
		{"UPDATE t SET id = id + 1 ORDER BY id DESC;", true, "UPDATE `t` SET `id`=`id`+1 ORDER BY `id` DESC"},
		{"UPDATE t SET id = id + 1 ORDER BY id DESC limit 3 ;", true, "UPDATE `t` SET `id`=`id`+1 ORDER BY `id` DESC LIMIT 3"},
		{"UPDATE t SET id = id + 1, name = 'jojo';", true, "UPDATE `t` SET `id`=`id`+1, `name`=_UTF8MB4'jojo'"},
		{"UPDATE items,month SET items.price=month.price WHERE items.id=month.id;", true, "UPDATE (`items`) JOIN `month` SET `items`.`price`=`month`.`price` WHERE `items`.`id`=`month`.`id`"},
		{"UPDATE user T0 LEFT OUTER JOIN user_profile T1 ON T1.id = T0.profile_id SET T0.profile_id = 1 WHERE T0.profile_id IN (1);", true, "UPDATE `user` AS `T0` LEFT JOIN `user_profile` AS `T1` ON `T1`.`id`=`T0`.`profile_id` SET `T0`.`profile_id`=1 WHERE `T0`.`profile_id` IN (1)"},
		{"UPDATE t1, t2 set t1.profile_id = 1, t2.profile_id = 1 where ta.a=t.ba", true, "UPDATE (`t1`) JOIN `t2` SET `t1`.`profile_id`=1, `t2`.`profile_id`=1 WHERE `ta`.`a`=`t`.`ba`"},
		// for optimizer hint in update statement
		{"UPDATE /*+ TiDB_INLJ(t1, t2) */ t1, t2 set t1.profile_id = 1, t2.profile_id = 1 where ta.a=t.ba", true, "UPDATE /*+ TIDB_INLJ(`t1`, `t2`)*/ (`t1`) JOIN `t2` SET `t1`.`profile_id`=1, `t2`.`profile_id`=1 WHERE `ta`.`a`=`t`.`ba`"},
		{"UPDATE /*+ TiDB_SMJ(t1, t2) */ t1, t2 set t1.profile_id = 1, t2.profile_id = 1 where ta.a=t.ba", true, "UPDATE /*+ TIDB_SMJ(`t1`, `t2`)*/ (`t1`) JOIN `t2` SET `t1`.`profile_id`=1, `t2`.`profile_id`=1 WHERE `ta`.`a`=`t`.`ba`"},
		{"UPDATE /*+ TiDB_HJ(t1, t2) */ t1, t2 set t1.profile_id = 1, t2.profile_id = 1 where ta.a=t.ba", true, "UPDATE /*+ TIDB_HJ(`t1`, `t2`)*/ (`t1`) JOIN `t2` SET `t1`.`profile_id`=1, `t2`.`profile_id`=1 WHERE `ta`.`a`=`t`.`ba`"},
		// fail case for update statement
		{"UPDATE items,month SET items.price=month.price WHERE items.id=month.id LIMIT 10;", false, ""},
		{"UPDATE items,month SET items.price=month.price WHERE items.id=month.id order by month.id;", false, ""},
		// for "USE INDEX" in delete statement
		{"UPDATE t1 USE INDEX(idx_a) SET t1.price=3.25 WHERE t1.id=1;", true, "UPDATE `t1` USE INDEX (`idx_a`) SET `t1`.`price`=3.25 WHERE `t1`.`id`=1"},
		{"UPDATE t1 USE INDEX(idx_a) JOIN t2 SET t1.price=t2.price WHERE t1.id=t2.id;", true, "UPDATE `t1` USE INDEX (`idx_a`) JOIN `t2` SET `t1`.`price`=`t2`.`price` WHERE `t1`.`id`=`t2`.`id`"},
		{"UPDATE t1 USE INDEX(idx_a) JOIN t2 USE INDEX(idx_a) SET t1.price=t2.price WHERE t1.id=t2.id;", true, "UPDATE `t1` USE INDEX (`idx_a`) JOIN `t2` USE INDEX (`idx_a`) SET `t1`.`price`=`t2`.`price` WHERE `t1`.`id`=`t2`.`id`"},

		// for select with where clause
		{"SELECT * FROM t WHERE 1 = 1", true, "SELECT * FROM `t` WHERE 1=1"},

		// for select with FETCH FIRST syntax
		{"SELECT * FROM t FETCH FIRST 5 ROW ONLY", true, "SELECT * FROM `t` LIMIT 5"},
		{"SELECT * FROM t FETCH NEXT 5 ROW ONLY", true, "SELECT * FROM `t` LIMIT 5"},
		{"SELECT * FROM t FETCH FIRST 5 ROWS ONLY", true, "SELECT * FROM `t` LIMIT 5"},
		{"SELECT * FROM t FETCH NEXT 5 ROWS ONLY", true, "SELECT * FROM `t` LIMIT 5"},
		{"SELECT * FROM t FETCH FIRST ROW ONLY", true, "SELECT * FROM `t` LIMIT 1"},
		{"SELECT * FROM t FETCH NEXT ROW ONLY", true, "SELECT * FROM `t` LIMIT 1"},

		// for dual
		{"select 1 from dual", true, "SELECT 1"},
		{"select 1 from dual limit 1", true, "SELECT 1 LIMIT 1"},
		{"select 1 where exists (select 2)", true, "SELECT 1 FROM DUAL WHERE EXISTS (SELECT 2)"},
		{"select 1 from dual where not exists (select 2)", true, "SELECT 1 FROM DUAL WHERE NOT EXISTS (SELECT 2)"},
		{"select 1 as a from dual order by a", true, "SELECT 1 AS `a` ORDER BY `a`"},
		{"select 1 as a from dual where 1 < any (select 2) order by a", true, "SELECT 1 AS `a` FROM DUAL WHERE 1<ANY (SELECT 2) ORDER BY `a`"},
		{"select 1 order by 1", true, "SELECT 1 ORDER BY 1"},

		// for https://github.com/pingcap/tidb/issues/320
		{`(select 1);`, true, "(SELECT 1)"},

		// https://github.com/pingcap/tidb/issues/14297
		{"select 1 where 1=1", true, "SELECT 1 FROM DUAL WHERE 1=1"},

		// https://github.com/pingcap/tidb/issues/24496
		{"select 1 group by 1", true, "SELECT 1 GROUP BY 1"},
		{"select 1 from dual group by 1", true, "SELECT 1 GROUP BY 1"},

		// for https://github.com/pingcap/parser/issues/963
		{"select min(b) b from (select min(t.b) b from t where t.a = '');", true, "SELECT MIN(`b`) AS `b` FROM (SELECT MIN(`t`.`b`) AS `b` FROM `t` WHERE `t`.`a`=_UTF8MB4'')"},
		{"select min(b) b from (select min(t.b) b from t where t.a = '') as t1;", true, "SELECT MIN(`b`) AS `b` FROM (SELECT MIN(`t`.`b`) AS `b` FROM `t` WHERE `t`.`a`=_UTF8MB4'') AS `t1`"},

		// for https://github.com/pingcap/tidb/issues/1050
		{`SELECT /*!40001 SQL_NO_CACHE */ * FROM test WHERE 1 limit 0, 2000;`, true, "SELECT SQL_NO_CACHE * FROM `test` WHERE 1 LIMIT 0,2000"},

		{`ANALYZE TABLE t`, true, "ANALYZE TABLE `t`"},

		// for comments
		{`/** 20180417 **/ show databases;`, true, "SHOW DATABASES"},
		{`/* 20180417 **/ show databases;`, true, "SHOW DATABASES"},
		{`/** 20180417 */ show databases;`, true, "SHOW DATABASES"},
		{`/** 20180417 ******/ show databases;`, true, "SHOW DATABASES"},
		{`/**/show databases;`, true, "SHOW DATABASES"},
		{`/*+*/show databases;`, true, "SHOW DATABASES"},
		{`select/*+*/1;`, true, "SELECT 1"},
		{`/*T*/show databases;`, true, "SHOW DATABASES"},
		{`/*M*/show databases;`, true, "SHOW DATABASES"},
		{`/*!*/show databases;`, true, "SHOW DATABASES"},
		{`/*T!*/show databases;`, true, "SHOW DATABASES"},
		{`/*M!*/show databases;`, true, "SHOW DATABASES"},

		// for Binlog stmt
		{`BINLOG '
BxSFVw8JAAAA8QAAAPUAAAAAAAQANS41LjQ0LU1hcmlhREItbG9nAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAEzgNAAgAEgAEBAQEEgAA2QAEGggAAAAICAgCAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAA5gm5Mg==
'/*!*/;`, true, `BINLOG '
BxSFVw8JAAAA8QAAAPUAAAAAAAQANS41LjQ0LU1hcmlhREItbG9nAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAEzgNAAgAEgAEBAQEEgAA2QAEGggAAAAICAgCAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA
AAAAAAAAAAAA5gm5Mg==
'`},

		// for partition table dml
		{"select * from t1 partition (p1)", true, "SELECT * FROM `t1` PARTITION(`p1`)"},
		{"select * from t1 partition (p1,p2)", true, "SELECT * FROM `t1` PARTITION(`p1`, `p2`)"},
		{"select * from t1 partition (`p1`, p2, p3)", true, "SELECT * FROM `t1` PARTITION(`p1`, `p2`, `p3`)"},
		{`select * from t1 partition ()`, false, ""},

		// for split table index region syntax
		{"split table t1 index idx1 by ('a'),('b'),('c')", true, "SPLIT TABLE `t1` INDEX `idx1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split table t1 index idx1 by (1)", true, "SPLIT TABLE `t1` INDEX `idx1` BY (1)"},
		{"split table t1 index idx1 by ('abc',123), ('xyz'), ('yz', 1000)", true, "SPLIT TABLE `t1` INDEX `idx1` BY (_UTF8MB4'abc',123),(_UTF8MB4'xyz'),(_UTF8MB4'yz',1000)"},
		{"split table t1 index idx1 by ", false, ""},
		{"split table t1 index idx1 between ('a') and ('z') regions 10", true, "SPLIT TABLE `t1` INDEX `idx1` BETWEEN (_UTF8MB4'a') AND (_UTF8MB4'z') REGIONS 10"},
		{"split table t1 index idx1 between ('a',1) and ('z',2) regions 10", true, "SPLIT TABLE `t1` INDEX `idx1` BETWEEN (_UTF8MB4'a',1) AND (_UTF8MB4'z',2) REGIONS 10"},
		{"split table t1 index idx1 between () and () regions 10", true, "SPLIT TABLE `t1` INDEX `idx1` BETWEEN () AND () REGIONS 10"},
		{"split table t1 index by (1)", false, ""},

		{"split region for table t1 index idx1 by ('a'),('b'),('c')", true, "SPLIT REGION FOR TABLE `t1` INDEX `idx1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split partition table t1 index idx1 by ('a'),('b'),('c')", true, "SPLIT PARTITION TABLE `t1` INDEX `idx1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split region for partition table t1 index idx1 by ('a'),('b'),('c')", true, "SPLIT REGION FOR PARTITION TABLE `t1` INDEX `idx1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split region for table t1 index idx1 between ('a') and ('z') regions 10", true, "SPLIT REGION FOR TABLE `t1` INDEX `idx1` BETWEEN (_UTF8MB4'a') AND (_UTF8MB4'z') REGIONS 10"},
		{"split partition table t1 index idx1 between ('a') and ('z') regions 10", true, "SPLIT PARTITION TABLE `t1` INDEX `idx1` BETWEEN (_UTF8MB4'a') AND (_UTF8MB4'z') REGIONS 10"},
		{"split region for partition table t1 index idx1 between ('a') and ('z') regions 10", true, "SPLIT REGION FOR PARTITION TABLE `t1` INDEX `idx1` BETWEEN (_UTF8MB4'a') AND (_UTF8MB4'z') REGIONS 10"},

		{"split region for table t1 partition (p0,p1) index idx1 by ('a'),('b'),('c')", true, "SPLIT REGION FOR TABLE `t1` PARTITION(`p0`, `p1`) INDEX `idx1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split partition table t1 partition (p0) index idx1 by ('a'),('b'),('c')", true, "SPLIT PARTITION TABLE `t1` PARTITION(`p0`) INDEX `idx1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split region for partition table t1 partition (p0) index idx1 by ('a'),('b'),('c')", true, "SPLIT REGION FOR PARTITION TABLE `t1` PARTITION(`p0`) INDEX `idx1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split region for table t1 partition (p0) index idx1 between ('a') and ('z') regions 10", true, "SPLIT REGION FOR TABLE `t1` PARTITION(`p0`) INDEX `idx1` BETWEEN (_UTF8MB4'a') AND (_UTF8MB4'z') REGIONS 10"},
		{"split partition table t1 partition (p0) index idx1 between ('a') and ('z') regions 10", true, "SPLIT PARTITION TABLE `t1` PARTITION(`p0`) INDEX `idx1` BETWEEN (_UTF8MB4'a') AND (_UTF8MB4'z') REGIONS 10"},
		{"split region for partition table t1 partition (p0) index idx1 between ('a') and ('z') regions 10", true, "SPLIT REGION FOR PARTITION TABLE `t1` PARTITION(`p0`) INDEX `idx1` BETWEEN (_UTF8MB4'a') AND (_UTF8MB4'z') REGIONS 10"},

		// for split table region.
		{"split table t1 by ('a'),('b'),('c')", true, "SPLIT TABLE `t1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split table t1 by (1)", true, "SPLIT TABLE `t1` BY (1)"},
		{"split table t1 by ('abc',123), ('xyz'), ('yz', 1000)", true, "SPLIT TABLE `t1` BY (_UTF8MB4'abc',123),(_UTF8MB4'xyz'),(_UTF8MB4'yz',1000)"},
		{"split table t1 by ", false, ""},
		{"split table t1 between ('a') and ('z') regions 10", true, "SPLIT TABLE `t1` BETWEEN (_UTF8MB4'a') AND (_UTF8MB4'z') REGIONS 10"},
		{"split table t1 between ('a',1) and ('z',2) regions 10", true, "SPLIT TABLE `t1` BETWEEN (_UTF8MB4'a',1) AND (_UTF8MB4'z',2) REGIONS 10"},
		{"split table t1 between () and () regions 10", true, "SPLIT TABLE `t1` BETWEEN () AND () REGIONS 10"},

		{"split region for table t1 by ('a'),('b'),('c')", true, "SPLIT REGION FOR TABLE `t1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split partition table t1 by ('a'),('b'),('c')", true, "SPLIT PARTITION TABLE `t1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split region for partition table t1 by ('a'),('b'),('c')", true, "SPLIT REGION FOR PARTITION TABLE `t1` BY (_UTF8MB4'a'),(_UTF8MB4'b'),(_UTF8MB4'c')"},
		{"split region for table t1 between (1) and (1000) regions 10", true, "SPLIT REGION FOR TABLE `t1` BETWEEN (1) AND (1000) REGIONS 10"},
		{"split partition table t1 between (1) and (1000) regions 10", true, "SPLIT PARTITION TABLE `t1` BETWEEN (1) AND (1000) REGIONS 10"},
		{"split region for partition table t1 between (1) and (1000) regions 10", true, "SPLIT REGION FOR PARTITION TABLE `t1` BETWEEN (1) AND (1000) REGIONS 10"},

		// for show table regions.
		{"show table t1 regions", true, "SHOW TABLE `t1` REGIONS"},
		{"show table t1 regions where a=1", true, "SHOW TABLE `t1` REGIONS WHERE `a`=1"},
		{"show table t1", false, ""},
		{"show table t1 index idx1 regions", true, "SHOW TABLE `t1` INDEX `idx1` REGIONS"},
		{"show table t1 index idx1 regions where a=2", true, "SHOW TABLE `t1` INDEX `idx1` REGIONS WHERE `a`=2"},
		{"show table t1 index idx1", false, ""},

		// for show table partition regions.
		{"show table t1 partition (p0,p1) regions", true, "SHOW TABLE `t1` PARTITION(`p0`, `p1`) REGIONS"},
		{"show table t1 partition (p0) regions where a=1", true, "SHOW TABLE `t1` PARTITION(`p0`) REGIONS WHERE `a`=1"},
		{"show table t1 partition", false, ""},
		{"show table t1 partition (p0) index idx1 regions", true, "SHOW TABLE `t1` PARTITION(`p0`) INDEX `idx1` REGIONS"},
		{"show table t1 partition (p0,p1) index idx1 regions where a=2", true, "SHOW TABLE `t1` PARTITION(`p0`, `p1`) INDEX `idx1` REGIONS WHERE `a`=2"},
		{"show table t1 partition index idx1", false, ""},

		// for show table partition distributions.
		{"show table t1 distributions", true, "SHOW TABLE `t1` DISTRIBUTIONS"},
		{"show table t1 distributions where a=1", true, "SHOW TABLE `t1` DISTRIBUTIONS WHERE `a`=1"},
		{"show table t1 partition (p0,p1) distributions", true, "SHOW TABLE `t1` PARTITION(`p0`, `p1`) DISTRIBUTIONS"},
		{"show table t1 partition (p0,p1) distributions where a=1", true, "SHOW TABLE `t1` PARTITION(`p0`, `p1`) DISTRIBUTIONS WHERE `a`=1"},

		// for distribute table
		{"distribute table t1", false, ""},
		{"distribute table t1 partition(p0)", false, ""},
		{"distribute table t1 partition(p0,p1)", false, ""},
		{"distribute table t1 partition(p0,p1) engine = tikv", false, ""},
		{"distribute table t1 rule = 'leader-scatter' engine = 'tikv'", true, "DISTRIBUTE TABLE `t1` RULE = 'leader-scatter' ENGINE = 'tikv'"},
		{"distribute table t1 rule = \"leader-scatter\" engine = \"tikv\"", true, "DISTRIBUTE TABLE `t1` RULE = 'leader-scatter' ENGINE = 'tikv'"},
		{"distribute table t1 partition(p0,p1) rule = 'learner-scatter' engine = 'tikv'", true, "DISTRIBUTE TABLE `t1` PARTITION(`p0`, `p1`) RULE = 'learner-scatter' ENGINE = 'tikv'"},
		{"distribute table t1 partition(p0) rule = 'peer-scatter' engine = 'tiflash'", true, "DISTRIBUTE TABLE `t1` PARTITION(`p0`) RULE = 'peer-scatter' ENGINE = 'tiflash'"},
		{"distribute table t1 partition(p0) rule = 'peer-scatter' engine = 'tiflash' timeout = '30m'", true, "DISTRIBUTE TABLE `t1` PARTITION(`p0`) RULE = 'peer-scatter' ENGINE = 'tiflash' TIMEOUT = '30m'"},

		// for show distribution job(s)
		{"show distribution jobs 1", false, ""},
		{"show distribution jobs", true, "SHOW DISTRIBUTION JOBS"},
		{"show distribution jobs where id > 0", true, "SHOW DISTRIBUTION JOBS WHERE `id`>0"},
		{"show distribution job 1 where id > 0", false, ""},
		{"show distribution job 1", true, "SHOW DISTRIBUTION JOB 1"},

		// for cancel distribution job JOBID
		{"cancel distribution job", false, ""},
		{"cancel distribution job 1", true, "CANCEL DISTRIBUTION JOB 1"},

		// for show table next_row_id.
		{"show table t1.t1 next_row_id", true, "SHOW TABLE `t1`.`t1` NEXT_ROW_ID"},
		{"show table t1 next_row_id", true, "SHOW TABLE `t1` NEXT_ROW_ID"},
		{"show table next_row_id", false, ""},

		// for transaction mode
		{"begin pessimistic", true, "BEGIN PESSIMISTIC"},
		{"begin optimistic", true, "BEGIN OPTIMISTIC"},

		// for repair table mode.
		{"ADMIN REPAIR TABLE t CREATE TABLE t (a int)", true, "ADMIN REPAIR TABLE `t` CREATE TABLE `t` (`a` INT)"},
		{"ADMIN REPAIR TABLE t CREATE TABLE t (a char(1))", true, "ADMIN REPAIR TABLE `t` CREATE TABLE `t` (`a` CHAR(1))"},
		{"ADMIN REPAIR TABLE t CREATE TABLE t (a char(1), b int)", true, "ADMIN REPAIR TABLE `t` CREATE TABLE `t` (`a` CHAR(1),`b` INT)"},
		{"ADMIN REPAIR TABLE t CREATE TABLE t (c1 TIME(2), c2 DATETIME(2), c3 TIMESTAMP(2));", true, "ADMIN REPAIR TABLE `t` CREATE TABLE `t` (`c1` TIME(2),`c2` DATETIME(2),`c3` TIMESTAMP(2))"},
		{"ADMIN REPAIR TABLE t CREATE TABLE t (a TINYINT UNSIGNED);", true, "ADMIN REPAIR TABLE `t` CREATE TABLE `t` (`a` TINYINT UNSIGNED)"},
		{"ADMIN REPAIR TABLE t CREATE TABLE t (name CHAR(50) CHARACTER SET UTF8)", true, "ADMIN REPAIR TABLE `t` CREATE TABLE `t` (`name` CHAR(50) CHARACTER SET UTF8)"},

		// for alter instance.
		{"ALTER INSTANCE RELOAD TLS", true, "ALTER INSTANCE RELOAD TLS"},
		{"ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR", true, "ALTER INSTANCE RELOAD TLS NO ROLLBACK ON ERROR"},

		// for alter range
		{"ALTER RANGE global PLACEMENT POLICY mypolicy", true, "ALTER RANGE `global` PLACEMENT POLICY = `mypolicy`"},
		{"ALTER RANGE global PLACEMENT POLICY default", true, "ALTER RANGE `global` PLACEMENT POLICY = `default`"},
		{"ALTER RANGE meta PLACEMENT POLICY mypolicy", true, "ALTER RANGE `meta` PLACEMENT POLICY = `mypolicy`"},

		// for create sequence with signed value especially with Two's Complement Min.
		// for issue #17948
		{"CREATE SEQUENCE seq INCREMENT - 9223372036854775807", true, "CREATE SEQUENCE `seq` INCREMENT BY -9223372036854775807"},
		{"CREATE SEQUENCE seq INCREMENT - 9223372036854775808", true, "CREATE SEQUENCE `seq` INCREMENT BY -9223372036854775808"},
		{"CREATE SEQUENCE seq INCREMENT -9223372036854775808", true, "CREATE SEQUENCE `seq` INCREMENT BY -9223372036854775808"},
		{"CREATE SEQUENCE seq INCREMENT -9223372036854775809", false, ""},

		{"select `t`.`1a`.1 from t;", true, "SELECT `t`.`1a`.`1` FROM `t`"},
		{"select * from 1db.1table;", true, "SELECT * FROM `1db`.`1table`"},
		{"select * from t where t. status = 1;", true, "SELECT * FROM `t` WHERE `t`.`status`=1"},

		// for show placement
		{"SHOW PLACEMENT", true, "SHOW PLACEMENT"},
		{"SHOW PLACEMENT LIKE 'POLICY foo%'", true, "SHOW PLACEMENT LIKE _UTF8MB4'POLICY foo%'"},
		{"SHOW PLACEMENT WHERE Target='TABLE test.t1'", true, "SHOW PLACEMENT WHERE `Target`=_UTF8MB4'TABLE test.t1'"},
		{"SHOW PLACEMENT FOR DATABASE db1", true, "SHOW PLACEMENT FOR DATABASE `db1`"},
		{"SHOW PLACEMENT FOR SCHEMA db1", true, "SHOW PLACEMENT FOR DATABASE `db1`"},
		{"SHOW PLACEMENT FOR TABLE tb1", true, "SHOW PLACEMENT FOR TABLE `tb1`"},
		{"SHOW PLACEMENT FOR TABLE db1.tb1", true, "SHOW PLACEMENT FOR TABLE `db1`.`tb1`"},
		{"SHOW PLACEMENT FOR TABLE tb1 PARTITION p1", true, "SHOW PLACEMENT FOR TABLE `tb1` PARTITION `p1`"},
		{"SHOW PLACEMENT FOR TABLE db1.tb1 PARTITION p1", true, "SHOW PLACEMENT FOR TABLE `db1`.`tb1` PARTITION `p1`"},
		{"SHOW PLACEMENT FOR", false, ""},
		{"SHOW PLACEMENT DATABASE db1", false, ""},
		{"SHOW PLACEMENT FOR DB db1", false, ""},
		{"SHOW PLACEMENT FOR DATABASE db1 TABLE tb1", false, ""},
		{"SHOW PLACEMENT FOR PARTITION p1", false, ""},
		{"SHOW PLACEMENT FOR DB LIKE '%'", false, ""},
		{"SHOW PLACEMENT FOR DB db1 LIKE '%'", false, ""},

		// for show placement labels
		{"SHOW PLACEMENT LABELS", true, "SHOW PLACEMENT LABELS"},
		{"SHOW PLACEMENT LABELS LIKE '%zone%'", true, "SHOW PLACEMENT LABELS LIKE _UTF8MB4'%zone%'"},
		{"SHOW PLACEMENT LABELS WHERE label='l123'", true, "SHOW PLACEMENT LABELS WHERE `label`=_UTF8MB4'l123'"},

		// for show/set session_states
		{"SHOW SESSION_STATES", true, "SHOW SESSION_STATES"},
		{"SET SESSION_STATES 'x'", true, "SET SESSION_STATES 'x'"},
		{"SET SESSION_STATES", false, ""},
		{"SET SESSION_STATES 1", false, ""},
		{"SET SESSION_STATES now()", false, ""},

		// for calibrate resource
		{"calibrate resource", true, "CALIBRATE RESOURCE"},
		{"calibrate resource START_TIME '2021-04-15 00:00:00'", true, "CALIBRATE RESOURCE START_TIME _UTF8MB4'2021-04-15 00:00:00'"},
		{"calibrate resource START_TIME '2023-04-01 13:00:00' END_TIME '2023-04-01 16:00:00'", true, "CALIBRATE RESOURCE START_TIME _UTF8MB4'2023-04-01 13:00:00' END_TIME _UTF8MB4'2023-04-01 16:00:00'"},
		{"calibrate resource START_TIME '2023-04-01 13:00:00' DURATION '20m'", true, "CALIBRATE RESOURCE START_TIME _UTF8MB4'2023-04-01 13:00:00' DURATION '20m'"},
		{"calibrate resource START_TIME '2023-04-01 13:00:00' END_TIME '2023-04-01 16:00:00' DURATION '20m'", true, "CALIBRATE RESOURCE START_TIME _UTF8MB4'2023-04-01 13:00:00' END_TIME _UTF8MB4'2023-04-01 16:00:00' DURATION '20m'"},
		{"calibrate resource START_TIME '2023-04-01 13:00:00',END_TIME='2023-04-01 16:00:00'", true, "CALIBRATE RESOURCE START_TIME _UTF8MB4'2023-04-01 13:00:00' END_TIME _UTF8MB4'2023-04-01 16:00:00'"},
		{"calibrate resource START_TIME '2023-04-01 13:00:00',DURATION='20m'", true, "CALIBRATE RESOURCE START_TIME _UTF8MB4'2023-04-01 13:00:00' DURATION '20m'"},
		{"calibrate resource DURATION='20m' START_TIME '2023-04-01 13:00:00'", true, "CALIBRATE RESOURCE DURATION '20m' START_TIME _UTF8MB4'2023-04-01 13:00:00'"},
		{"calibrate resource   START_TIME '2023-04-01 13:00:00' END_TIME='2023-04-01 16:00:00',DURATION '20m'", true, "CALIBRATE RESOURCE START_TIME _UTF8MB4'2023-04-01 13:00:00' END_TIME _UTF8MB4'2023-04-01 16:00:00' DURATION '20m'"},
		{"calibrate resource START_TIME CURRENT_TIMESTAMP() END_TIME current_timestamp()", true, "CALIBRATE RESOURCE START_TIME CURRENT_TIMESTAMP() END_TIME CURRENT_TIMESTAMP()"},
		{"calibrate resource END_TIME now()", true, "CALIBRATE RESOURCE END_TIME NOW()"},
		{"calibrate resource START_TIME now()", true, "CALIBRATE RESOURCE START_TIME NOW()"},
		{"calibrate resource START_TIME NOW() END_TIME now()", true, "CALIBRATE RESOURCE START_TIME NOW() END_TIME NOW()"},
		{"calibrate resource START_TIME CURRENT_TIMESTAMP() - interval 10 minute END_TIME now()", true, "CALIBRATE RESOURCE START_TIME DATE_SUB(CURRENT_TIMESTAMP(), INTERVAL 10 MINUTE) END_TIME NOW()"},
		{"calibrate resource START_TIME now() - 1000 END_TIME current_timestamp()", true, "CALIBRATE RESOURCE START_TIME NOW()-1000 END_TIME CURRENT_TIMESTAMP()"},
		{"calibrate resource START_TIME CURRENT_TIMESTAMP() - interval 20 minute DURATION interval 15 minute", true, "CALIBRATE RESOURCE START_TIME DATE_SUB(CURRENT_TIMESTAMP(), INTERVAL 20 MINUTE) DURATION INTERVAL 15 MINUTE"},
		{"calibrate resource START_TIME CURRENT_TIMESTAMP() - interval 20 minute DURATION '15m'", true, "CALIBRATE RESOURCE START_TIME DATE_SUB(CURRENT_TIMESTAMP(), INTERVAL 20 MINUTE) DURATION '15m'"},
		{"calibrate resource END_TIME now() START_TIME CURRENT_TIMESTAMP() - interval 20 minute", true, "CALIBRATE RESOURCE END_TIME NOW() START_TIME DATE_SUB(CURRENT_TIMESTAMP(), INTERVAL 20 MINUTE)"},
		{"calibrate resource workload", false, ""},
		{"calibrate resource workload tpcc", true, "CALIBRATE RESOURCE WORKLOAD TPCC"},
		{"calibrate resource workload oltp_read_write", true, "CALIBRATE RESOURCE WORKLOAD OLTP_READ_WRITE"},
		{"calibrate resource workload oltp_read_only", true, "CALIBRATE RESOURCE WORKLOAD OLTP_READ_ONLY"},
		{"calibrate resource workload oltp_write_only", true, "CALIBRATE RESOURCE WORKLOAD OLTP_WRITE_ONLY"},
		{"calibrate resource workload = oltp_read_write START_TIME '2023-04-01 13:00:00'", false, ""},

		// for query watch
		{"query watch add SQL DIGEST b13858789fce00208f9a262c99621b7045f4869807cd4e6568008ae7ca19a377 ", true, "QUERY WATCH ADD SQL DIGEST `b13858789fce00208f9a262c99621b7045f4869807cd4e6568008ae7ca19a377`"},
		{"query watch add SQL DIGEST b13858789fce00208f9a262c99621b7045f4869807cd4e6568008ae7ca19a377 ", true, "QUERY WATCH ADD SQL DIGEST `b13858789fce00208f9a262c99621b7045f4869807cd4e6568008ae7ca19a377`"},
		{"query watch add SQL DIGEST 'b13858789fce00208f9a262c99621b7045f4869807cd4e6568008ae7ca19a377' ", true, "QUERY WATCH ADD SQL DIGEST _UTF8MB4'b13858789fce00208f9a262c99621b7045f4869807cd4e6568008ae7ca19a377'"},
		{"query watch add PLAN DIGEST `5e3ddd388f6012e328233dbcdda5d48f404e0536c6c54d9618233210f3d5762a` ", true, "QUERY WATCH ADD PLAN DIGEST `5e3ddd388f6012e328233dbcdda5d48f404e0536c6c54d9618233210f3d5762a`"},
		{"query watch add PLAN DIGEST @digest1 ", true, "QUERY WATCH ADD PLAN DIGEST @`digest1`"},
		{"query watch add SQL TEXT SIMILAR to 'select 1'", true, "QUERY WATCH ADD SQL TEXT SIMILAR TO _UTF8MB4'select 1'"},
		{"query watch add SQL TEXT EXACT to 'select 1'", true, "QUERY WATCH ADD SQL TEXT EXACT TO _UTF8MB4'select 1'"},
		{"query watch add SQL TEXT PLAN to 'select 1'", true, "QUERY WATCH ADD SQL TEXT PLAN TO _UTF8MB4'select 1'"},
		{"query watch add resource group `default` SQL TEXT SIMILAR to 'select 1'", true, "QUERY WATCH ADD RESOURCE GROUP `default` SQL TEXT SIMILAR TO _UTF8MB4'select 1'"},
		{"query watch add resource group @rg SQL TEXT SIMILAR to @sql1", true, "QUERY WATCH ADD RESOURCE GROUP @`rg` SQL TEXT SIMILAR TO @`sql1`"},
		{"query watch add resource group rg1 SQL TEXT SIMILAR to 'select 1'", true, "QUERY WATCH ADD RESOURCE GROUP `rg1` SQL TEXT SIMILAR TO _UTF8MB4'select 1'"},
		{"query watch add SQL TEXT SIMILAR to 'select 1' resource group rg1", true, "QUERY WATCH ADD SQL TEXT SIMILAR TO _UTF8MB4'select 1' RESOURCE GROUP `rg1`"},
		{"query watch add ACTION = KILL SQL TEXT SIMILAR to 'select 1'", true, "QUERY WATCH ADD ACTION = KILL SQL TEXT SIMILAR TO _UTF8MB4'select 1'"},
		{"query watch add ACTION COOLDOWN resource group rg1 SQL TEXT SIMILAR to 'select 1'", true, "QUERY WATCH ADD ACTION = COOLDOWN RESOURCE GROUP `rg1` SQL TEXT SIMILAR TO _UTF8MB4'select 1'"},
		{"query watch add resource group `default` resource group `rg1` SQL TEXT SIMILAR to 'select 1'", false, ""},
		{"query watch add SQL SIMILAR to 'select 1'", false, ""},
		{"query watch add SQL TEXT SIMILAR 'select 1'", false, ""},
		{"query watch remove 1", true, "QUERY WATCH REMOVE 1"},
		{"query watch remove resource group rg1", true, "QUERY WATCH REMOVE RESOURCE GROUP `rg1`"},
		{"query watch remove resource group @rg", true, "QUERY WATCH REMOVE RESOURCE GROUP @`rg`"},
		{"query watch remove", false, ""},

		// for issue 34325, "replace into" with hints
		{"replace /*+ SET_VAR(sql_mode='ALLOW_INVALID_DATES') */ into t values ('2004-04-31');", true, "REPLACE /*+ SET_VAR(sql_mode = 'ALLOW_INVALID_DATES')*/ INTO `t` VALUES (_UTF8MB4'2004-04-31')"},
	}
	RunTest(t, table, false)
}

func TestDBAStmt(t *testing.T) {
	table := []testCase{
		// for SHOW statement
		{"SHOW VARIABLES LIKE 'character_set_results'", true, "SHOW SESSION VARIABLES LIKE _UTF8MB4'character_set_results'"},
		{"SHOW GLOBAL VARIABLES LIKE 'character_set_results'", true, "SHOW GLOBAL VARIABLES LIKE _UTF8MB4'character_set_results'"},
		{"SHOW SESSION VARIABLES LIKE 'character_set_results'", true, "SHOW SESSION VARIABLES LIKE _UTF8MB4'character_set_results'"},
		{"SHOW VARIABLES", true, "SHOW SESSION VARIABLES"},
		{"SHOW GLOBAL VARIABLES", true, "SHOW GLOBAL VARIABLES"},
		{"SHOW GLOBAL VARIABLES WHERE Variable_name = 'autocommit'", true, "SHOW GLOBAL VARIABLES WHERE `Variable_name`=_UTF8MB4'autocommit'"},
		{"SHOW STATUS", true, "SHOW SESSION STATUS"},
		{"SHOW GLOBAL STATUS", true, "SHOW GLOBAL STATUS"},
		{"SHOW SESSION STATUS", true, "SHOW SESSION STATUS"},
		{`SHOW STATUS LIKE 'Up%'`, true, "SHOW SESSION STATUS LIKE _UTF8MB4'Up%'"},
		{`SHOW STATUS WHERE Variable_name`, true, "SHOW SESSION STATUS WHERE `Variable_name`"},
		{`SHOW STATUS WHERE Variable_name LIKE 'Up%'`, true, "SHOW SESSION STATUS WHERE `Variable_name` LIKE _UTF8MB4'Up%'"},
		{`SHOW FULL TABLES FROM icar_qa LIKE play_evolutions`, true, "SHOW FULL TABLES IN `icar_qa` LIKE `play_evolutions`"},
		{`SHOW FULL TABLES WHERE Table_Type != 'VIEW'`, true, "SHOW FULL TABLES WHERE `Table_Type`!=_UTF8MB4'VIEW'"},
		{`SHOW GRANTS`, true, "SHOW GRANTS"},
		{`SHOW GRANTS FOR 'test'@'localhost'`, true, "SHOW GRANTS FOR `test`@`localhost`"},
		{`SHOW GRANTS FOR 'test'@'LOCALHOST'`, true, "SHOW GRANTS FOR `test`@`localhost`"},
		{`SHOW GRANTS FOR current_user()`, true, "SHOW GRANTS FOR CURRENT_USER"},
		{`SHOW GRANTS FOR current_user`, true, "SHOW GRANTS FOR CURRENT_USER"},
		{`SHOW GRANTS FOR 'u1'@'localhost' USING 'r1'`, true, "SHOW GRANTS FOR `u1`@`localhost` USING `r1`@`%`"},
		{`SHOW GRANTS FOR 'u1'@'localhost' USING 'r1', 'r2'`, true, "SHOW GRANTS FOR `u1`@`localhost` USING `r1`@`%`, `r2`@`%`"},
		{`SHOW COLUMNS FROM City;`, true, "SHOW COLUMNS IN `City`"},
		{`SHOW COLUMNS FROM tv189.1_t_1_x;`, true, "SHOW COLUMNS IN `tv189`.`1_t_1_x`"},
		{`SHOW FIELDS FROM City;`, true, "SHOW COLUMNS IN `City`"},
		{`SHOW TRIGGERS LIKE 't'`, true, "SHOW TRIGGERS LIKE _UTF8MB4't'"},
		{`SHOW DATABASES LIKE 'test2'`, true, "SHOW DATABASES LIKE _UTF8MB4'test2'"},
		// PROCEDURE and FUNCTION are currently not supported.
		// And FUNCTION reuse show procedure status process logic.
		{`SHOW PROCEDURE STATUS WHERE Db='test'`, true, "SHOW PROCEDURE STATUS WHERE `Db`=_UTF8MB4'test'"},
		{`SHOW FUNCTION STATUS WHERE Db='test'`, true, "SHOW FUNCTION STATUS WHERE `Db`=_UTF8MB4'test'"},
		{`SHOW INDEX FROM t;`, true, "SHOW INDEX IN `t`"},
		{`SHOW KEYS FROM t;`, true, "SHOW INDEX IN `t`"},
		{`SHOW INDEX IN t;`, true, "SHOW INDEX IN `t`"},
		{`SHOW KEYS IN t;`, true, "SHOW INDEX IN `t`"},
		{`SHOW INDEXES IN t where true;`, true, "SHOW INDEX IN `t` WHERE TRUE"},
		{`SHOW KEYS FROM t FROM test where true;`, true, "SHOW INDEX IN `test`.`t` WHERE TRUE"},
		{`SHOW EVENTS FROM test_db WHERE definer = 'current_user'`, true, "SHOW EVENTS IN `test_db` WHERE `definer`=_UTF8MB4'current_user'"},
		{`SHOW PLUGINS`, true, "SHOW PLUGINS"},
		{`SHOW PROFILES`, true, "SHOW PROFILES"},
		{`SHOW PROFILE`, true, "SHOW PROFILE"},
		{`SHOW PROFILE FOR QUERY 1`, true, "SHOW PROFILE FOR QUERY 1"},
		{`SHOW PROFILE CPU FOR QUERY 2`, true, "SHOW PROFILE CPU FOR QUERY 2"},
		{`SHOW PROFILE CPU FOR QUERY 2 LIMIT 1,1`, true, "SHOW PROFILE CPU FOR QUERY 2 LIMIT 1,1"},
		{`SHOW PROFILE CPU, MEMORY, BLOCK IO, CONTEXT SWITCHES, PAGE FAULTS, IPC, SWAPS, SOURCE FOR QUERY 1 limit 100`, true, "SHOW PROFILE CPU, MEMORY, BLOCK IO, CONTEXT SWITCHES, PAGE FAULTS, IPC, SWAPS, SOURCE FOR QUERY 1 LIMIT 100"},
		{`SHOW MASTER STATUS`, true, "SHOW MASTER STATUS"},
		{`SHOW BINARY LOG STATUS`, true, "SHOW BINARY LOG STATUS"},
		{`SHOW PRIVILEGES`, true, "SHOW PRIVILEGES"},
		// for show character set
		{"show character set;", true, "SHOW CHARSET"},
		{"show charset", true, "SHOW CHARSET"},
		// for show collation
		{"show collation", true, "SHOW COLLATION"},
		{`show collation like 'utf8%'`, true, "SHOW COLLATION LIKE _UTF8MB4'utf8%'"},
		{"show collation where Charset = 'utf8' and Collation = 'utf8_bin'", true, "SHOW COLLATION WHERE `Charset`=_UTF8MB4'utf8' AND `Collation`=_UTF8MB4'utf8_bin'"},
		// for show full columns
		{"show columns in t;", true, "SHOW COLUMNS IN `t`"},
		{"show full columns in t;", true, "SHOW FULL COLUMNS IN `t`"},
		// for show extended columns
		{`SHOW COLUMNS FROM City;`, true, "SHOW COLUMNS IN `City`"},
		{`SHOW EXTENDED COLUMNS FROM City;`, true, "SHOW EXTENDED COLUMNS IN `City`"},
		{`SHOW EXTENDED FIELDS FROM City;`, true, "SHOW EXTENDED COLUMNS IN `City`"},
		// for show extended full columns
		{`SHOW EXTENDED FULL COLUMNS FROM City;`, true, "SHOW EXTENDED FULL COLUMNS IN `City`"},
		{`SHOW EXTENDED FULL FIELDS FROM City;`, true, "SHOW EXTENDED FULL COLUMNS IN `City`"},
		// for show create table
		{"show create table test.t", true, "SHOW CREATE TABLE `test`.`t`"},
		{"show create table t", true, "SHOW CREATE TABLE `t`"},
		// for show create view
		{"show create view test.t", true, "SHOW CREATE VIEW `test`.`t`"},
		{"show create view t", true, "SHOW CREATE VIEW `t`"},
		// for show create database
		{"show create database d1", true, "SHOW CREATE DATABASE `d1`"},
		{"show create database if not exists d1", true, "SHOW CREATE DATABASE IF NOT EXISTS `d1`"},
		// for show create sequence
		{"show create sequence seq", true, "SHOW CREATE SEQUENCE `seq`"},
		{"show create sequence test.seq", true, "SHOW CREATE SEQUENCE `test`.`seq`"},
		// for show stats_extended.
		{"show stats_extended", true, "SHOW STATS_EXTENDED"},
		{"show stats_extended where table_name = 't'", true, "SHOW STATS_EXTENDED WHERE `table_name`=_UTF8MB4't'"},
		// for show stats_meta.
		{"show stats_meta", true, "SHOW STATS_META"},
		{"show stats_meta where table_name = 't'", true, "SHOW STATS_META WHERE `table_name`=_UTF8MB4't'"},
		// for show stats_locked.
		{"show stats_locked", true, "SHOW STATS_LOCKED"},
		{"show stats_locked where table_name = 't'", true, "SHOW STATS_LOCKED WHERE `table_name`=_UTF8MB4't'"},
		// for show stats_histograms
		{"show stats_histograms", true, "SHOW STATS_HISTOGRAMS"},
		{"show stats_histograms where col_name = 'a'", true, "SHOW STATS_HISTOGRAMS WHERE `col_name`=_UTF8MB4'a'"},
		// for show stats_buckets
		{"show stats_buckets", true, "SHOW STATS_BUCKETS"},
		{"show stats_buckets where col_name = 'a'", true, "SHOW STATS_BUCKETS WHERE `col_name`=_UTF8MB4'a'"},
		// for show stats_healthy.
		{"show stats_healthy", true, "SHOW STATS_HEALTHY"},
		{"show stats_healthy where table_name = 't'", true, "SHOW STATS_HEALTHY WHERE `table_name`=_UTF8MB4't'"},
		// for show stats_topn.
		{"show stats_topn", true, "SHOW STATS_TOPN"},
		{"show stats_topn where table_name = 't'", true, "SHOW STATS_TOPN WHERE `table_name`=_UTF8MB4't'"},
		// for show histograms_in_flight.
		{"show histograms_in_flight", true, "SHOW HISTOGRAMS_IN_FLIGHT"},
		// for show column_stats_usage.
		{"show column_stats_usage", true, "SHOW COLUMN_STATS_USAGE"},
		{"show column_stats_usage where table_name = 't'", true, "SHOW COLUMN_STATS_USAGE WHERE `table_name`=_UTF8MB4't'"},
		// for show binding_cache status
		{"show binding_cache status", true, "SHOW BINDING_CACHE STATUS"},
		{"show analyze status", true, "SHOW ANALYZE STATUS"},
		{"show analyze status where table_name = 't'", true, "SHOW ANALYZE STATUS WHERE `table_name`=_UTF8MB4't'"},
		{"show analyze status where table_name like '%'", true, "SHOW ANALYZE STATUS WHERE `table_name` LIKE _UTF8MB4'%'"},
		// for show builtins
		{"show builtins", true, "SHOW BUILTINS"},
		// for show backup & restore
		{"show backups", true, "SHOW BACKUPS"},
		{"show restores like 'r0001'", true, "SHOW RESTORES LIKE _UTF8MB4'r0001'"},
		{"show backups where start_time > now() - interval 10 hour", true, "SHOW BACKUPS WHERE `start_time`>DATE_SUB(NOW(), INTERVAL 10 HOUR)"},
		{"show backup", false, ""},
		{"show restore", false, ""},
		{"show replica status", true, "SHOW REPLICA STATUS"},
		{"show slave status", true, "SHOW REPLICA STATUS"},

		// for load stats
		{"load stats '/tmp/stats.json'", true, "LOAD STATS '/tmp/stats.json'"},
		// for lock stats
		{"lock stats test.t", true, "LOCK STATS `test`.`t`"},
		{"lock stats t, t2", true, "LOCK STATS `t`, `t2`"},
		{"lock stats t partition (p0, p1)", true, "LOCK STATS `t` PARTITION(`p0`, `p1`)"},
		// for unlock stats
		{"unlock stats test.t", true, "UNLOCK STATS `test`.`t`"},
		{"unlock stats t, t2", true, "UNLOCK STATS `t`, `t2`"},
		{"unlock stats t partition (p0, p1)", true, "UNLOCK STATS `t` PARTITION(`p0`, `p1`)"},
		// set
		// user defined
		{"SET @ = 1", true, "SET @``=1"},
		{"SET @' ' = 1", true, "SET @` `=1"},
		{"SET @! = 1", false, ""},
		{"SET @1 = 1", true, "SET @`1`=1"},
		{"SET @a = 1", true, "SET @`a`=1"},
		{"SET @b := 1", true, "SET @`b`=1"},
		{"SET @.c = 1", true, "SET @`.c`=1"},
		{"SET @_d = 1", true, "SET @`_d`=1"},
		{"SET @_e._$. = 1", true, "SET @`_e._$.`=1"},
		{"SET @~f = 1", false, ""},
		{"SET @`g,` = 1", true, "SET @`g,`=1"},
		{"SET", false, ""},
		{"SET @a = 1, @b := 2", true, "SET @`a`=1, @`b`=2"},
		// session system variables
		{"SET SESSION autocommit = 1", true, "SET @@SESSION.`autocommit`=1"},
		{"SET @@session.autocommit = 1", true, "SET @@SESSION.`autocommit`=1"},
		{"SET @@SESSION.autocommit = 1", true, "SET @@SESSION.`autocommit`=1"},
		{"SET @@GLOBAL.GTID_PURGED = '123'", true, "SET @@GLOBAL.`gtid_purged`=_UTF8MB4'123'"},
		{"SET @MYSQLDUMP_TEMP_LOG_BIN = @@SESSION.SQL_LOG_BIN", true, "SET @`MYSQLDUMP_TEMP_LOG_BIN`=@@SESSION.`sql_log_bin`"},
		{"SET LOCAL autocommit = 1", true, "SET @@SESSION.`autocommit`=1"},
		{"SET @@local.autocommit = 1", true, "SET @@SESSION.`autocommit`=1"},
		{"SET @@autocommit = 1", true, "SET @@SESSION.`autocommit`=1"},
		{"SET autocommit = 1", true, "SET @@SESSION.`autocommit`=1"},
		// global system variables
		{"SET GLOBAL autocommit = 1", true, "SET @@GLOBAL.`autocommit`=1"},
		{"SET @@global.autocommit = 1", true, "SET @@GLOBAL.`autocommit`=1"},
		// set through mysql extension assignment syntax
		{"SET autocommit := 1", true, "SET @@SESSION.`autocommit`=1"},
		{"SET @@session.autocommit := 1", true, "SET @@SESSION.`autocommit`=1"},
		{"SET @MYSQLDUMP_TEMP_LOG_BIN := @@SESSION.SQL_LOG_BIN", true, "SET @`MYSQLDUMP_TEMP_LOG_BIN`=@@SESSION.`sql_log_bin`"},
		{"SET LOCAL autocommit := 1", true, "SET @@SESSION.`autocommit`=1"},
		{"SET @@global.autocommit := default", true, "SET @@GLOBAL.`autocommit`=DEFAULT"},
		// set default value
		{"SET @@global.autocommit = default", true, "SET @@GLOBAL.`autocommit`=DEFAULT"},
		{"SET @@session.autocommit = default", true, "SET @@SESSION.`autocommit`=DEFAULT"},
		// set binary value
		{"SET @@character_set_results = binary", true, "SET @@SESSION.`character_set_results`=_UTF8MB4'BINARY'"},
		// SET CHARACTER SET
		{"SET CHARACTER SET utf8mb4;", true, "SET CHARSET 'utf8mb4'"},
		{"SET CHARACTER SET 'utf8mb4';", true, "SET CHARSET 'utf8mb4'"},
		// set password
		{"SET PASSWORD = 'password';", true, "SET PASSWORD='password'"},
		{"SET PASSWORD FOR 'root'@'localhost' = 'password';", true, "SET PASSWORD FOR `root`@`localhost`='password'"},
		// SET TRANSACTION Syntax
		{"SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ", true, "SET @@SESSION.`tx_isolation`=_UTF8MB4'REPEATABLE-READ'"},
		{"SET GLOBAL TRANSACTION ISOLATION LEVEL REPEATABLE READ", true, "SET @@GLOBAL.`tx_isolation`=_UTF8MB4'REPEATABLE-READ'"},
		{"SET SESSION TRANSACTION READ WRITE", true, "SET @@SESSION.`tx_read_only`=_UTF8MB4'0'"},
		{"SET SESSION TRANSACTION READ ONLY", true, "SET @@SESSION.`tx_read_only`=_UTF8MB4'1'"},
		{"SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED", true, "SET @@SESSION.`tx_isolation`=_UTF8MB4'READ-COMMITTED'"},
		{"SET SESSION TRANSACTION ISOLATION LEVEL READ UNCOMMITTED", true, "SET @@SESSION.`tx_isolation`=_UTF8MB4'READ-UNCOMMITTED'"},
		{"SET SESSION TRANSACTION ISOLATION LEVEL SERIALIZABLE", true, "SET @@SESSION.`tx_isolation`=_UTF8MB4'SERIALIZABLE'"},
		{"SET TRANSACTION ISOLATION LEVEL REPEATABLE READ", true, "SET @@SESSION.`tx_isolation_one_shot`=_UTF8MB4'REPEATABLE-READ'"},
		{"SET TRANSACTION READ WRITE", true, "SET @@SESSION.`tx_read_only`=_UTF8MB4'0'"},
		{"SET TRANSACTION READ ONLY", true, "SET @@SESSION.`tx_read_only`=_UTF8MB4'1'"},
		{"SET TRANSACTION ISOLATION LEVEL READ COMMITTED", true, "SET @@SESSION.`tx_isolation_one_shot`=_UTF8MB4'READ-COMMITTED'"},
		{"SET TRANSACTION ISOLATION LEVEL READ UNCOMMITTED", true, "SET @@SESSION.`tx_isolation_one_shot`=_UTF8MB4'READ-UNCOMMITTED'"},
		{"SET TRANSACTION ISOLATION LEVEL SERIALIZABLE", true, "SET @@SESSION.`tx_isolation_one_shot`=_UTF8MB4'SERIALIZABLE'"},
		// for set names
		{"set names utf8", true, "SET NAMES 'utf8'"},
		{"set names utf8 collate utf8_unicode_ci", true, "SET NAMES 'utf8' COLLATE 'utf8_unicode_ci'"},
		{"set names binary", true, "SET NAMES 'binary'"},

		// for set character set | name default
		{"set names default", true, "SET NAMES DEFAULT"},
		{"set character set default", true, "SET CHARSET DEFAULT"},
		{"set charset default", true, "SET CHARSET DEFAULT"},
		{"set char set default", true, "SET CHARSET DEFAULT"},

		{"set role `role1`", true, "SET ROLE `role1`@`%`"},
		{"SET ROLE DEFAULT", true, "SET ROLE DEFAULT"},
		{"SET ROLE ALL", true, "SET ROLE ALL"},
		{"SET ROLE ALL EXCEPT `role1`, `role2`", true, "SET ROLE ALL EXCEPT `role1`@`%`, `role2`@`%`"},
		{"SET DEFAULT ROLE administrator, developer TO `joe`@`10.0.0.1`", true, "SET DEFAULT ROLE `administrator`@`%`, `developer`@`%` TO `joe`@`10.0.0.1`"},
		// for set names and set vars
		{"set names utf8, @@session.sql_mode=1;", true, "SET NAMES 'utf8', @@SESSION.`sql_mode`=1"},
		{"set @@session.sql_mode=1, names utf8, charset utf8;", true, "SET @@SESSION.`sql_mode`=1, NAMES 'utf8', CHARSET 'utf8'"},

		// for set/show config
		{"set config TIKV LOG.LEVEL='info'", true, "SET CONFIG TIKV LOG.LEVEL = _UTF8MB4'info'"},
		{"set config PD LOG.LEVEL='info'", true, "SET CONFIG PD LOG.LEVEL = _UTF8MB4'info'"},
		{"set config TIDB LOG.LEVEL='info'", true, "SET CONFIG TIDB LOG.LEVEL = _UTF8MB4'info'"},
		{"set config '127.0.0.1:3306' LOG.LEVEL='info'", true, "SET CONFIG '127.0.0.1:3306' LOG.LEVEL = _UTF8MB4'info'"},
		{"set config '127.0.0.1:3306' AUTO-COMPACTION-MODE=TRUE", true, "SET CONFIG '127.0.0.1:3306' AUTO-COMPACTION-MODE = TRUE"},
		{"set config '127.0.0.1:3306' LABEL-PROPERTY.REJECT-LEADER.KEY='zone'", true, "SET CONFIG '127.0.0.1:3306' LABEL-PROPERTY.REJECT-LEADER.KEY = _UTF8MB4'zone'"},
		{"show config", true, "SHOW CONFIG"},
		{"show config where type='tidb'", true, "SHOW CONFIG WHERE `type`=_UTF8MB4'tidb'"},
		{"show config where instance='127.0.0.1:3306'", true, "SHOW CONFIG WHERE `instance`=_UTF8MB4'127.0.0.1:3306'"},
		{"create table CONFIG (a int)", true, "CREATE TABLE `CONFIG` (`a` INT)"}, // check that `CONFIG` is unreserved keyword

		// for FLUSH statement
		{"flush no_write_to_binlog tables tbl1 with read lock", true, "FLUSH NO_WRITE_TO_BINLOG TABLES `tbl1` WITH READ LOCK"},
		{"flush table", true, "FLUSH TABLES"},
		{"flush tables", true, "FLUSH TABLES"},
		{"flush tables tbl1", true, "FLUSH TABLES `tbl1`"},
		{"flush no_write_to_binlog tables tbl1", true, "FLUSH NO_WRITE_TO_BINLOG TABLES `tbl1`"},
		{"flush local tables tbl1", true, "FLUSH NO_WRITE_TO_BINLOG TABLES `tbl1`"},
		{"flush table with read lock", true, "FLUSH TABLES WITH READ LOCK"},
		{"flush tables tbl1, tbl2, tbl3", true, "FLUSH TABLES `tbl1`, `tbl2`, `tbl3`"},
		{"flush tables tbl1, tbl2, tbl3 with read lock", true, "FLUSH TABLES `tbl1`, `tbl2`, `tbl3` WITH READ LOCK"},
		{"flush privileges", true, "FLUSH PRIVILEGES"},
		{"flush status", true, "FLUSH STATUS"},
		{"flush tidb plugins plugin1", true, "FLUSH TIDB PLUGINS plugin1"},
		{"flush tidb plugins plugin1, plugin2", true, "FLUSH TIDB PLUGINS plugin1, plugin2"},
		{"flush hosts", true, "FLUSH HOSTS"},
		{"flush logs", true, "FLUSH LOGS"},
		{"flush binary logs", true, "FLUSH BINARY LOGS"},
		{"flush engine logs", true, "FLUSH ENGINE LOGS"},
		{"flush error logs", true, "FLUSH ERROR LOGS"},
		{"flush general logs", true, "FLUSH GENERAL LOGS"},
		{"flush slow logs", true, "FLUSH SLOW LOGS"},
		{"flush client_errors_summary", true, "FLUSH CLIENT_ERRORS_SUMMARY"},

		// for call statement
		{"call ", false, ""},
		{"call test", true, "CALL `test`()"},
		{"call test()", true, "CALL `test`()"},
		{"call test(1, 'test', true)", true, "CALL `test`(1, _UTF8MB4'test', TRUE)"},
		{"call x.y;", true, "CALL `x`.`y`()"},
		{"call x.y();", true, "CALL `x`.`y`()"},
		{"call x.y('p', 'q', 'r');", true, "CALL `x`.`y`(_UTF8MB4'p', _UTF8MB4'q', _UTF8MB4'r')"},
		{"call `x`.`y`;", true, "CALL `x`.`y`()"},
		{"call `x`.`y`();", true, "CALL `x`.`y`()"},
		{"call `x`.`y`('p', 'q', 'r');", true, "CALL `x`.`y`(_UTF8MB4'p', _UTF8MB4'q', _UTF8MB4'r')"},
	}
	RunTest(t, table, false)
}

func TestSetVariable(t *testing.T) {
	table := []struct {
		Input    string
		Name     string
		IsGlobal bool
		IsSystem bool
	}{

		// Set system variable xx.xx, although xx.xx isn't a system variable, the parser should accept it.
		{"set xx.xx = 666", "xx.xx", false, true},
		// Set session system variable xx.xx
		{"set session xx.xx = 666", "xx.xx", false, true},
		{"set local xx.xx = 666", "xx.xx", false, true},
		{"set global xx.xx = 666", "xx.xx", true, true},

		{"set @@xx.xx = 666", "xx.xx", false, true},
		{"set @@session.xx.xx = 666", "xx.xx", false, true},
		{"set @@local.xx.xx = 666", "xx.xx", false, true},
		{"set @@global.xx.xx = 666", "xx.xx", true, true},

		// Set user defined variable xx.xx
		{"set @xx.xx = 666", "xx.xx", false, false},
	}

	p := parser.New()
	for _, tbl := range table {
		stmt, err := p.ParseOneStmt(tbl.Input, "", "")
		require.NoError(t, err)

		setStmt, ok := stmt.(*ast.SetStmt)
		require.True(t, ok)
		require.Len(t, setStmt.Variables, 1)

		v := setStmt.Variables[0]
		require.Equal(t, tbl.Name, v.Name)
		require.Equal(t, tbl.IsGlobal, v.IsGlobal)
		require.Equal(t, tbl.IsSystem, v.IsSystem)
	}

	_, err := p.ParseOneStmt("set xx.xx.xx = 666", "", "")
	require.Error(t, err)
}

func TestFlushTable(t *testing.T) {
	p := parser.New()
	stmt, _, err := p.Parse("flush local tables tbl1,tbl2 with read lock", "", "")
	require.NoError(t, err)
	flushTable := stmt[0].(*ast.FlushStmt)
	require.Equal(t, ast.FlushTables, flushTable.Tp)
	require.Equal(t, "tbl1", flushTable.Tables[0].Name.L)
	require.Equal(t, "tbl2", flushTable.Tables[1].Name.L)
	require.True(t, flushTable.NoWriteToBinLog)
	require.True(t, flushTable.ReadLock)
}

func TestFlushPrivileges(t *testing.T) {
	p := parser.New()
	stmt, _, err := p.Parse("flush privileges", "", "")
	require.NoError(t, err)
	flushPrivilege := stmt[0].(*ast.FlushStmt)
	require.Equal(t, ast.FlushPrivileges, flushPrivilege.Tp)
}

func TestExpression(t *testing.T) {
	table := []testCase{
		// sign expression
		{"SELECT ++1", true, "SELECT ++1"},
		{"SELECT -*1", false, "SELECT -*1"},
		{"SELECT -+1", true, "SELECT -+1"},
		{"SELECT -1", true, "SELECT -1"},
		{"SELECT --1", true, "SELECT --1"},

		// for string literal
		{`select '''a''', """a"""`, true, "SELECT _UTF8MB4'''a''',_UTF8MB4'\"a\"'"},
		{`select ''a''`, false, ""},
		{`select ""a""`, false, ""},
		{`select '''a''';`, true, "SELECT _UTF8MB4'''a'''"},
		{`select '\'a\'';`, true, "SELECT _UTF8MB4'''a'''"},
		{`select "\"a\"";`, true, "SELECT _UTF8MB4'\"a\"'"},
		{`select """a""";`, true, "SELECT _UTF8MB4'\"a\"'"},
		{`select _utf8"string";`, true, "SELECT _UTF8'string'"},
		{`select _binary"string";`, true, "SELECT _BINARY'string'"},
		{"select N'string'", true, "SELECT _UTF8'string'"},
		{"select n'string'", true, "SELECT _UTF8'string'"},
		{"select _utf8 0xD0B1;", true, "SELECT _UTF8 x'd0b1'"},
		{"select _utf8 X'D0B1';", true, "SELECT _UTF8 x'd0b1'"},
		{"select _utf8 0b1101000010110001;", true, "SELECT _UTF8 b'1101000010110001'"},
		{"select _utf8 B'1101000010110001';", true, "SELECT _UTF8 b'1101000010110001'"},
		// for comparison
		{"select 1 <=> 0, 1 <=> null, 1 = null", true, "SELECT 1<=>0,1<=>NULL,1=NULL"},
		// for date literal
		{"select date'1989-09-10'", true, "SELECT DATE '1989-09-10'"},
		{"select date 19890910", false, ""},
		// for time literal
		{"select time '00:00:00.111'", true, "SELECT TIME '00:00:00.111'"},
		{"select time 19890910", false, ""},
		// for timestamp literal
		{"select timestamp '1989-09-10 11:11:11'", true, "SELECT TIMESTAMP '1989-09-10 11:11:11'"},
		{"select timestamp 19890910", false, ""},

		// The ODBC syntax for time/date/timestamp literal.
		// See: https://dev.mysql.com/doc/refman/5.7/en/date-and-time-literals.html
		{"select {ts '1989-09-10 11:11:11'}", true, "SELECT TIMESTAMP '1989-09-10 11:11:11'"},
		{"select {d '1989-09-10'}", true, "SELECT DATE '1989-09-10'"},
		{"select {t '00:00:00.111'}", true, "SELECT TIME '00:00:00.111'"},
		{"select * from t where a > {ts '1989-09-10 11:11:11'}", true, "SELECT * FROM `t` WHERE `a`>TIMESTAMP '1989-09-10 11:11:11'"},
		{"select * from t where a > {ts {abc '1989-09-10 11:11:11'}}", true, "SELECT * FROM `t` WHERE `a`>TIMESTAMP '1989-09-10 11:11:11'"},
		// If the identifier is not in (t, d, ts), we just ignore it and consider the following expression as the value.
		// See: https://dev.mysql.com/doc/refman/5.7/en/expressions.html
		{"select {ts123 '1989-09-10 11:11:11'}", true, "SELECT _UTF8MB4'1989-09-10 11:11:11'"},
		{"select {ts123 123}", true, "SELECT 123"},
		{"select {ts123 1 xor 1}", true, "SELECT 1 XOR 1"},
		{"select * from t where a > {ts123 '1989-09-10 11:11:11'}", true, "SELECT * FROM `t` WHERE `a`>_UTF8MB4'1989-09-10 11:11:11'"},
		{"select .t.a from t", false, ""},
	}

	RunTest(t, table, false)
}

func TestBuiltin(t *testing.T) {
	table := []testCase{
		// for builtin functions
		{"SELECT POW(1, 2)", true, "SELECT POW(1, 2)"},
		{"SELECT POW(1, 2, 1)", true, "SELECT POW(1, 2, 1)"}, // illegal number of arguments shall pass too
		{"SELECT POW(1, 0.5)", true, "SELECT POW(1, 0.5)"},
		{"SELECT POW(1, -1)", true, "SELECT POW(1, -1)"},
		{"SELECT POW(-1, 1)", true, "SELECT POW(-1, 1)"},
		{"SELECT RAND();", true, "SELECT RAND()"},
		{"SELECT RAND(1);", true, "SELECT RAND(1)"},
		{"SELECT MOD(10, 2);", true, "SELECT 10%2"},
		{"SELECT ROUND(-1.23);", true, "SELECT ROUND(-1.23)"},
		{"SELECT ROUND(1.23, 1);", true, "SELECT ROUND(1.23, 1)"},
		{"SELECT ROUND(1.23, 1, 1);", true, "SELECT ROUND(1.23, 1, 1)"},
		{"SELECT CEIL(-1.23);", true, "SELECT CEIL(-1.23)"},
		{"SELECT CEILING(1.23);", true, "SELECT CEILING(1.23)"},
		{"SELECT FLOOR(-1.23);", true, "SELECT FLOOR(-1.23)"},
		{"SELECT LN(1);", true, "SELECT LN(1)"},
		{"SELECT LN(1, 2);", true, "SELECT LN(1, 2)"},
		{"SELECT LOG(-2);", true, "SELECT LOG(-2)"},
		{"SELECT LOG(2, 65536);", true, "SELECT LOG(2, 65536)"},
		{"SELECT LOG(2, 65536, 1);", true, "SELECT LOG(2, 65536, 1)"},
		{"SELECT LOG2(2);", true, "SELECT LOG2(2)"},
		{"SELECT LOG2(2, 2);", true, "SELECT LOG2(2, 2)"},
		{"SELECT LOG10(10);", true, "SELECT LOG10(10)"},
		{"SELECT LOG10(10, 1);", true, "SELECT LOG10(10, 1)"},
		{"SELECT ABS(10, 1);", true, "SELECT ABS(10, 1)"},
		{"SELECT ABS(10);", true, "SELECT ABS(10)"},
		{"SELECT ABS();", true, "SELECT ABS()"},
		{"SELECT CONV(10+'10'+'10'+X'0a',10,10);", true, "SELECT CONV(10+_UTF8MB4'10'+_UTF8MB4'10'+x'0a', 10, 10)"},
		{"SELECT CONV();", true, "SELECT CONV()"},
		{"SELECT CRC32('MySQL');", true, "SELECT CRC32(_UTF8MB4'MySQL')"},
		{"SELECT CRC32();", true, "SELECT CRC32()"},
		{"SELECT SIGN();", true, "SELECT SIGN()"},
		{"SELECT SIGN(0);", true, "SELECT SIGN(0)"},
		{"SELECT SQRT(0);", true, "SELECT SQRT(0)"},
		{"SELECT SQRT();", true, "SELECT SQRT()"},
		{"SELECT ACOS();", true, "SELECT ACOS()"},
		{"SELECT ACOS(1);", true, "SELECT ACOS(1)"},
		{"SELECT ACOS(1, 2);", true, "SELECT ACOS(1, 2)"},
		{"SELECT ASIN();", true, "SELECT ASIN()"},
		{"SELECT ASIN(1);", true, "SELECT ASIN(1)"},
		{"SELECT ASIN(1, 2);", true, "SELECT ASIN(1, 2)"},
		{"SELECT ATAN(0), ATAN(1), ATAN(1, 2);", true, "SELECT ATAN(0),ATAN(1),ATAN(1, 2)"},
		{"SELECT ATAN2(), ATAN2(1,2);", true, "SELECT ATAN2(),ATAN2(1, 2)"},
		{"SELECT COS(0);", true, "SELECT COS(0)"},
		{"SELECT COS(1);", true, "SELECT COS(1)"},
		{"SELECT COS(1, 2);", true, "SELECT COS(1, 2)"},
		{"SELECT COT();", true, "SELECT COT()"},
		{"SELECT COT(1);", true, "SELECT COT(1)"},
		{"SELECT COT(1, 2);", true, "SELECT COT(1, 2)"},
		{"SELECT DEGREES();", true, "SELECT DEGREES()"},
		{"SELECT DEGREES(0);", true, "SELECT DEGREES(0)"},
		{"SELECT EXP();", true, "SELECT EXP()"},
		{"SELECT EXP(1);", true, "SELECT EXP(1)"},
		{"SELECT PI();", true, "SELECT PI()"},
		{"SELECT PI(1);", true, "SELECT PI(1)"},
		{"SELECT RADIANS();", true, "SELECT RADIANS()"},
		{"SELECT RADIANS(1);", true, "SELECT RADIANS(1)"},
		{"SELECT SIN();", true, "SELECT SIN()"},
		{"SELECT SIN(1);", true, "SELECT SIN(1)"},
		{"SELECT TAN(1);", true, "SELECT TAN(1)"},
		{"SELECT TAN();", true, "SELECT TAN()"},
		{"SELECT TRUNCATE(1.223,1);", true, "SELECT TRUNCATE(1.223, 1)"},
		{"SELECT TRUNCATE();", true, "SELECT TRUNCATE()"},

		{"SELECT SUBSTR('Quadratically',5);", true, "SELECT SUBSTR(_UTF8MB4'Quadratically', 5)"},
		{"SELECT SUBSTR('Quadratically',5, 3);", true, "SELECT SUBSTR(_UTF8MB4'Quadratically', 5, 3)"},
		{"SELECT SUBSTR('Quadratically' FROM 5);", true, "SELECT SUBSTR(_UTF8MB4'Quadratically', 5)"},
		{"SELECT SUBSTR('Quadratically' FROM 5 FOR 3);", true, "SELECT SUBSTR(_UTF8MB4'Quadratically', 5, 3)"},

		{"SELECT SUBSTRING('Quadratically',5);", true, "SELECT SUBSTRING(_UTF8MB4'Quadratically', 5)"},
		{"SELECT SUBSTRING('Quadratically',5, 3);", true, "SELECT SUBSTRING(_UTF8MB4'Quadratically', 5, 3)"},
		{"SELECT SUBSTRING('Quadratically' FROM 5);", true, "SELECT SUBSTRING(_UTF8MB4'Quadratically', 5)"},
		{"SELECT SUBSTRING('Quadratically' FROM 5 FOR 3);", true, "SELECT SUBSTRING(_UTF8MB4'Quadratically', 5, 3)"},

		{"SELECT CONVERT('111', SIGNED);", true, "SELECT CONVERT(_UTF8MB4'111', SIGNED)"},

		{"SELECT LEAST(), LEAST(1, 2, 3);", true, "SELECT LEAST(),LEAST(1, 2, 3)"},

		{"SELECT INTERVAL(1, 0, 1, 2)", true, "SELECT INTERVAL(1, 0, 1, 2)"},
		{"SELECT (INTERVAL(1, 0, 1, 2)+5)*7+INTERVAL(1, 0, 1, 2)/2", true, "SELECT (INTERVAL(1, 0, 1, 2)+5)*7+INTERVAL(1, 0, 1, 2)/2"},
		{"SELECT INTERVAL(0, (1*5)/2)+INTERVAL(5, 4, 3)", true, "SELECT INTERVAL(0, (1*5)/2)+INTERVAL(5, 4, 3)"},
		{"SELECT DATE_ADD('2008-01-02', INTERVAL INTERVAL(1, 0, 1) DAY);", true, "SELECT DATE_ADD(_UTF8MB4'2008-01-02', INTERVAL INTERVAL(1, 0, 1) DAY)"},

		// information functions
		{"SELECT DATABASE();", true, "SELECT DATABASE()"},
		{"SELECT SCHEMA();", true, "SELECT SCHEMA()"},
		{"SELECT USER();", true, "SELECT USER()"},
		{"SELECT USER(1);", true, "SELECT USER(1)"},
		{"SELECT CURRENT_USER();", true, "SELECT CURRENT_USER()"},
		{"SELECT CURRENT_ROLE();", true, "SELECT CURRENT_ROLE()"},
		{"SELECT CURRENT_USER;", true, "SELECT CURRENT_USER()"},
		{"SELECT CONNECTION_ID();", true, "SELECT CONNECTION_ID()"},
		{"SELECT VERSION();", true, "SELECT VERSION()"},
		{"SELECT CURRENT_RESOURCE_GROUP();", true, "SELECT CURRENT_RESOURCE_GROUP()"},
		{"SELECT BENCHMARK(1000000, AES_ENCRYPT('text',UNHEX('F3229A0B371ED2D9441B830D21A390C3')));", true, "SELECT BENCHMARK(1000000, AES_ENCRYPT(_UTF8MB4'text', UNHEX(_UTF8MB4'F3229A0B371ED2D9441B830D21A390C3')))"},
		{"SELECT BENCHMARK(AES_ENCRYPT('text',UNHEX('F3229A0B371ED2D9441B830D21A390C3')));", true, "SELECT BENCHMARK(AES_ENCRYPT(_UTF8MB4'text', UNHEX(_UTF8MB4'F3229A0B371ED2D9441B830D21A390C3')))"},
		{"SELECT CHARSET('abc');", true, "SELECT CHARSET(_UTF8MB4'abc')"},
		{"SELECT COERCIBILITY('abc');", true, "SELECT COERCIBILITY(_UTF8MB4'abc')"},
		{"SELECT COERCIBILITY('abc', 'a');", true, "SELECT COERCIBILITY(_UTF8MB4'abc', _UTF8MB4'a')"},
		{"SELECT COLLATION('abc');", true, "SELECT COLLATION(_UTF8MB4'abc')"},
		{"SELECT ROW_COUNT();", true, "SELECT ROW_COUNT()"},
		{"SELECT SESSION_USER();", true, "SELECT SESSION_USER()"},
		{"SELECT SYSTEM_USER();", true, "SELECT SYSTEM_USER()"},
		{"SELECT FORMAT_BYTES(512);", true, "SELECT FORMAT_BYTES(512)"},
		{"SELECT FORMAT_NANO_TIME(3501);", true, "SELECT FORMAT_NANO_TIME(3501)"},

		{"SELECT SUBSTRING_INDEX('www.mysql.com', '.', 2);", true, "SELECT SUBSTRING_INDEX(_UTF8MB4'www.mysql.com', _UTF8MB4'.', 2)"},
		{"SELECT SUBSTRING_INDEX('www.mysql.com', '.', -2);", true, "SELECT SUBSTRING_INDEX(_UTF8MB4'www.mysql.com', _UTF8MB4'.', -2)"},

		{`SELECT ASCII(), ASCII(""), ASCII("A"), ASCII(1);`, true, "SELECT ASCII(),ASCII(_UTF8MB4''),ASCII(_UTF8MB4'A'),ASCII(1)"},

		{`SELECT LOWER("A"), UPPER("a")`, true, "SELECT LOWER(_UTF8MB4'A'),UPPER(_UTF8MB4'a')"},
		{`SELECT LCASE("A"), UCASE("a")`, true, "SELECT LCASE(_UTF8MB4'A'),UCASE(_UTF8MB4'a')"},

		{`SELECT REPLACE('www.mysql.com', 'w', 'Ww')`, true, "SELECT REPLACE(_UTF8MB4'www.mysql.com', _UTF8MB4'w', _UTF8MB4'Ww')"},

		{`SELECT LOCATE('bar', 'foobarbar');`, true, "SELECT LOCATE(_UTF8MB4'bar', _UTF8MB4'foobarbar')"},
		{`SELECT LOCATE('bar', 'foobarbar', 5);`, true, "SELECT LOCATE(_UTF8MB4'bar', _UTF8MB4'foobarbar', 5)"},

		{`SELECT tidb_version();`, true, "SELECT TIDB_VERSION()"},
		{`SELECT tidb_is_ddl_owner();`, true, "SELECT TIDB_IS_DDL_OWNER()"},
		{`SELECT tidb_decode_plan();`, true, "SELECT TIDB_DECODE_PLAN()"},
		{`SELECT tidb_decode_key('abc');`, true, "SELECT TIDB_DECODE_KEY(_UTF8MB4'abc')"},
		{`SELECT tidb_decode_base64_key('abc');`, true, "SELECT TIDB_DECODE_BASE64_KEY(_UTF8MB4'abc')"},
		{`SELECT tidb_decode_sql_digests('[]');`, true, "SELECT TIDB_DECODE_SQL_DIGESTS(_UTF8MB4'[]')"},

		// for time fsp
		{"CREATE TABLE t( c1 TIME(2), c2 DATETIME(2), c3 TIMESTAMP(2) );", true, "CREATE TABLE `t` (`c1` TIME(2),`c2` DATETIME(2),`c3` TIMESTAMP(2))"},

		// for row
		{"select row(1)", false, ""},
		{"select row(1, 1,)", false, ""},
		{"select (1, 1,)", false, ""},
		{"select row(1, 1) > row(1, 1), row(1, 1, 1) > row(1, 1, 1)", true, "SELECT ROW(1,1)>ROW(1,1),ROW(1,1,1)>ROW(1,1,1)"},
		{"Select (1, 1) > (1, 1)", true, "SELECT ROW(1,1)>ROW(1,1)"},
		{"create table t (`row` int)", true, "CREATE TABLE `t` (`row` INT)"},
		{"create table t (row int)", false, ""},

		// for cast with charset
		{"SELECT *, CAST(data AS CHAR CHARACTER SET utf8) FROM t;", true, "SELECT *,CAST(`data` AS CHAR CHARSET UTF8) FROM `t`"},
		{"SELECT CAST(data AS CHARACTER);", true, "SELECT CAST(`data` AS CHAR)"},
		{"SELECT CAST(data AS CHARACTER(10) CHARACTER SET utf8);", true, "SELECT CAST(`data` AS CHAR(10) CHARSET UTF8)"},
		{"SELECT CAST(data AS BINARY)", true, "SELECT CAST(`data` AS BINARY)"},

		// for cast as JSON
		{"SELECT *, CAST(data AS JSON) FROM t;", true, "SELECT *,CAST(`data` AS JSON) FROM `t`"},

		// for JSON_SUM_CRC32
		{"SELECT *, JSON_SUM_CRC32(data AS UNSIGNED ARRAY) FROM t;", true, "SELECT *,JSON_SUM_CRC32(`data` AS UNSIGNED ARRAY) FROM `t`"},
		{"SELECT *, JSON_SUM_CRC32(data AS DOUBLE ARRAY) FROM t;", true, "SELECT *,JSON_SUM_CRC32(`data` AS DOUBLE ARRAY) FROM `t`"},
		{"SELECT *, JSON_SUM_CRC32(data AS DOUBLE) FROM t;", false, ""},
		{"SELECT *, JSON_SUM_CRC32(data) FROM t;", false, ""},

		// for cast as signed int, fix issue #3691.
		{"select cast(1 as signed int);", true, "SELECT CAST(1 AS SIGNED)"},

		// for cast as double
		{"select cast(1 as double);", true, "SELECT CAST(1 AS DOUBLE)"},

		// for cast as float
		{"select cast(1 as float);", true, "SELECT CAST(1 AS FLOAT)"},
		{"select cast(1 as float(0));", true, "SELECT CAST(1 AS FLOAT)"},
		{"select cast(1 as float(24));", true, "SELECT CAST(1 AS FLOAT)"},
		{"select cast(1 as float(25));", true, "SELECT CAST(1 AS DOUBLE)"},
		{"select cast(1 as float(53));", true, "SELECT CAST(1 AS DOUBLE)"},
		{"select cast(1 as float(54));", false, ""},

		{"select cast(1 as real);", true, "SELECT CAST(1 AS DOUBLE)"},
		{"select cast('2000' as year);", true, "SELECT CAST(_UTF8MB4'2000' AS YEAR)"},
		{"select cast(time '2000' as year);", true, "SELECT CAST(TIME '2000' AS YEAR)"},

		{"select cast(b as signed array);", true, "SELECT CAST(`b` AS SIGNED ARRAY)"},
		{"select cast(b as char(10) array);", true, "SELECT CAST(`b` AS CHAR(10) ARRAY)"},

		// for last_insert_id
		{"SELECT last_insert_id();", true, "SELECT LAST_INSERT_ID()"},
		{"SELECT last_insert_id(1);", true, "SELECT LAST_INSERT_ID(1)"},

		// for binary operator
		{"SELECT binary 'a';", true, "SELECT BINARY _UTF8MB4'a'"},

		// for bit_count
		{`SELECT BIT_COUNT(1);`, true, "SELECT BIT_COUNT(1)"},

		// select time
		{"select current_timestamp", true, "SELECT CURRENT_TIMESTAMP()"},
		{"select current_timestamp()", true, "SELECT CURRENT_TIMESTAMP()"},
		{"select current_timestamp(6)", true, "SELECT CURRENT_TIMESTAMP(6)"},
		{"select current_timestamp(null)", false, ""},
		{"select current_timestamp(-1)", false, ""},
		{"select current_timestamp(1.0)", false, ""},
		{"select current_timestamp('2')", false, ""},
		{"select now()", true, "SELECT NOW()"},
		{"select now(6)", true, "SELECT NOW(6)"},
		{"select sysdate(), sysdate(6)", true, "SELECT SYSDATE(),SYSDATE(6)"},
		{"SELECT time('01:02:03');", true, "SELECT TIME(_UTF8MB4'01:02:03')"},
		{"SELECT time('01:02:03.1')", true, "SELECT TIME(_UTF8MB4'01:02:03.1')"},
		{"SELECT time('20.1')", true, "SELECT TIME(_UTF8MB4'20.1')"},
		{"SELECT TIMEDIFF('2000:01:01 00:00:00', '2000:01:01 00:00:00.000001');", true, "SELECT TIMEDIFF(_UTF8MB4'2000:01:01 00:00:00', _UTF8MB4'2000:01:01 00:00:00.000001')"},
		{"SELECT TIMESTAMPDIFF(MONTH,'2003-02-01','2003-05-01');", true, "SELECT TIMESTAMPDIFF(MONTH, _UTF8MB4'2003-02-01', _UTF8MB4'2003-05-01')"},
		{"SELECT TIMESTAMPDIFF(YEAR,'2002-05-01','2001-01-01');", true, "SELECT TIMESTAMPDIFF(YEAR, _UTF8MB4'2002-05-01', _UTF8MB4'2001-01-01')"},
		{"SELECT TIMESTAMPDIFF(MINUTE,'2003-02-01','2003-05-01 12:05:55');", true, "SELECT TIMESTAMPDIFF(MINUTE, _UTF8MB4'2003-02-01', _UTF8MB4'2003-05-01 12:05:55')"},

		// select current_time
		{"select current_time", true, "SELECT CURRENT_TIME()"},
		{"select current_time()", true, "SELECT CURRENT_TIME()"},
		{"select current_time(6)", true, "SELECT CURRENT_TIME(6)"},
		{"select current_time(-1)", false, ""},
		{"select current_time(1.0)", false, ""},
		{"select current_time('1')", false, ""},
		{"select current_time(null)", false, ""},
		{"select curtime()", true, "SELECT CURTIME()"},
		{"select curtime(6)", true, "SELECT CURTIME(6)"},
		{"select curtime(-1)", false, ""},
		{"select curtime(1.0)", false, ""},
		{"select curtime('1')", false, ""},
		{"select curtime(null)", false, ""},

		// select utc_timestamp
		{"select utc_timestamp", true, "SELECT UTC_TIMESTAMP()"},
		{"select utc_timestamp()", true, "SELECT UTC_TIMESTAMP()"},
		{"select utc_timestamp(6)", true, "SELECT UTC_TIMESTAMP(6)"},
		{"select utc_timestamp(-1)", false, ""},
		{"select utc_timestamp(1.0)", false, ""},
		{"select utc_timestamp('1')", false, ""},
		{"select utc_timestamp(null)", false, ""},

		// select utc_time
		{"select utc_time", true, "SELECT UTC_TIME()"},
		{"select utc_time()", true, "SELECT UTC_TIME()"},
		{"select utc_time(6)", true, "SELECT UTC_TIME(6)"},
		{"select utc_time(-1)", false, ""},
		{"select utc_time(1.0)", false, ""},
		{"select utc_time('1')", false, ""},
		{"select utc_time(null)", false, ""},

		// for microsecond, second, minute, hour
		{"SELECT MICROSECOND('2009-12-31 23:59:59.000010');", true, "SELECT MICROSECOND(_UTF8MB4'2009-12-31 23:59:59.000010')"},
		{"SELECT SECOND('10:05:03');", true, "SELECT SECOND(_UTF8MB4'10:05:03')"},
		{"SELECT MINUTE('2008-02-03 10:05:03');", true, "SELECT MINUTE(_UTF8MB4'2008-02-03 10:05:03')"},
		{"SELECT HOUR(), HOUR('10:05:03');", true, "SELECT HOUR(),HOUR(_UTF8MB4'10:05:03')"},

		// for date, day, weekday
		{"SELECT CURRENT_DATE, CURRENT_DATE(), CURDATE()", true, "SELECT CURRENT_DATE(),CURRENT_DATE(),CURDATE()"},
		{"SELECT CURRENT_DATE, CURRENT_DATE(), CURDATE(1)", false, ""},
		{"SELECT DATEDIFF('2003-12-31', '2003-12-30');", true, "SELECT DATEDIFF(_UTF8MB4'2003-12-31', _UTF8MB4'2003-12-30')"},
		{"SELECT DATE('2003-12-31 01:02:03');", true, "SELECT DATE(_UTF8MB4'2003-12-31 01:02:03')"},
		{"SELECT DATE();", true, "SELECT DATE()"},
		{"SELECT DATE('2003-12-31 01:02:03', '');", true, "SELECT DATE(_UTF8MB4'2003-12-31 01:02:03', _UTF8MB4'')"},
		{`SELECT DATE_FORMAT('2003-12-31 01:02:03', '%W %M %Y');`, true, "SELECT DATE_FORMAT(_UTF8MB4'2003-12-31 01:02:03', _UTF8MB4'%W %M %Y')"},
		{"SELECT DAY('2007-02-03');", true, "SELECT DAY(_UTF8MB4'2007-02-03')"},
		{"SELECT DAYOFMONTH('2007-02-03');", true, "SELECT DAYOFMONTH(_UTF8MB4'2007-02-03')"},
		{"SELECT DAYOFWEEK('2007-02-03');", true, "SELECT DAYOFWEEK(_UTF8MB4'2007-02-03')"},
		{"SELECT DAYOFYEAR('2007-02-03');", true, "SELECT DAYOFYEAR(_UTF8MB4'2007-02-03')"},
		{"SELECT DAYNAME('2007-02-03');", true, "SELECT DAYNAME(_UTF8MB4'2007-02-03')"},
		{"SELECT FROM_DAYS(1423);", true, "SELECT FROM_DAYS(1423)"},
		{"SELECT WEEKDAY('2007-02-03');", true, "SELECT WEEKDAY(_UTF8MB4'2007-02-03')"},

		// for utc_date
		{"SELECT UTC_DATE, UTC_DATE();", true, "SELECT UTC_DATE(),UTC_DATE()"},
		{"SELECT UTC_DATE(), UTC_DATE()+0", true, "SELECT UTC_DATE(),UTC_DATE()+0"},

		// for week, month, year
		{"SELECT WEEK();", true, "SELECT WEEK()"},
		{"SELECT WEEK('2007-02-03');", true, "SELECT WEEK(_UTF8MB4'2007-02-03')"},
		{"SELECT WEEK('2007-02-03', 0);", true, "SELECT WEEK(_UTF8MB4'2007-02-03', 0)"},
		{"SELECT WEEKOFYEAR('2007-02-03');", true, "SELECT WEEKOFYEAR(_UTF8MB4'2007-02-03')"},
		{"SELECT MONTH('2007-02-03');", true, "SELECT MONTH(_UTF8MB4'2007-02-03')"},
		{"SELECT MONTHNAME('2007-02-03');", true, "SELECT MONTHNAME(_UTF8MB4'2007-02-03')"},
		{"SELECT YEAR('2007-02-03');", true, "SELECT YEAR(_UTF8MB4'2007-02-03')"},
		{"SELECT YEARWEEK('2007-02-03');", true, "SELECT YEARWEEK(_UTF8MB4'2007-02-03')"},
		{"SELECT YEARWEEK('2007-02-03', 0);", true, "SELECT YEARWEEK(_UTF8MB4'2007-02-03', 0)"},

		// for ADDTIME, SUBTIME
		{"SELECT ADDTIME('01:00:00.999999', '02:00:00.999998');", true, "SELECT ADDTIME(_UTF8MB4'01:00:00.999999', _UTF8MB4'02:00:00.999998')"},
		{"SELECT ADDTIME('02:00:00.999998');", true, "SELECT ADDTIME(_UTF8MB4'02:00:00.999998')"},
		{"SELECT ADDTIME();", true, "SELECT ADDTIME()"},
		{"SELECT SUBTIME('01:00:00.999999', '02:00:00.999998');", true, "SELECT SUBTIME(_UTF8MB4'01:00:00.999999', _UTF8MB4'02:00:00.999998')"},

		// for CONVERT_TZ
		{"SELECT CONVERT_TZ();", true, "SELECT CONVERT_TZ()"},
		{"SELECT CONVERT_TZ('2004-01-01 12:00:00','+00:00','+10:00');", true, "SELECT CONVERT_TZ(_UTF8MB4'2004-01-01 12:00:00', _UTF8MB4'+00:00', _UTF8MB4'+10:00')"},
		{"SELECT CONVERT_TZ('2004-01-01 12:00:00','+00:00','+10:00', '+10:00');", true, "SELECT CONVERT_TZ(_UTF8MB4'2004-01-01 12:00:00', _UTF8MB4'+00:00', _UTF8MB4'+10:00', _UTF8MB4'+10:00')"},

		// for GET_FORMAT
		{"SELECT GET_FORMAT(DATE, 'USA');", true, "SELECT GET_FORMAT(DATE, _UTF8MB4'USA')"},
		{"SELECT GET_FORMAT(DATETIME, 'USA');", true, "SELECT GET_FORMAT(DATETIME, _UTF8MB4'USA')"},
		{"SELECT GET_FORMAT(TIME, 'USA');", true, "SELECT GET_FORMAT(TIME, _UTF8MB4'USA')"},
		{"SELECT GET_FORMAT(TIMESTAMP, 'USA');", true, "SELECT GET_FORMAT(DATETIME, _UTF8MB4'USA')"},

		// for LOCALTIME, LOCALTIMESTAMP
		{"SELECT LOCALTIME(), LOCALTIME(1)", true, "SELECT LOCALTIME(),LOCALTIME(1)"},
		{"SELECT LOCALTIMESTAMP(), LOCALTIMESTAMP(2)", true, "SELECT LOCALTIMESTAMP(),LOCALTIMESTAMP(2)"},

		// for MAKEDATE, MAKETIME
		{"SELECT MAKEDATE(2011,31);", true, "SELECT MAKEDATE(2011, 31)"},
		{"SELECT MAKETIME(12,15,30);", true, "SELECT MAKETIME(12, 15, 30)"},
		{"SELECT MAKEDATE();", true, "SELECT MAKEDATE()"},
		{"SELECT MAKETIME();", true, "SELECT MAKETIME()"},

		// for PERIOD_ADD, PERIOD_DIFF
		{"SELECT PERIOD_ADD(200801,2)", true, "SELECT PERIOD_ADD(200801, 2)"},
		{"SELECT PERIOD_DIFF(200802,200703)", true, "SELECT PERIOD_DIFF(200802, 200703)"},

		// for QUARTER
		{"SELECT QUARTER('2008-04-01');", true, "SELECT QUARTER(_UTF8MB4'2008-04-01')"},

		// for SEC_TO_TIME
		{"SELECT SEC_TO_TIME(2378)", true, "SELECT SEC_TO_TIME(2378)"},

		// for TIME_FORMAT
		{`SELECT TIME_FORMAT('100:00:00', '%H %k %h %I %l')`, true, "SELECT TIME_FORMAT(_UTF8MB4'100:00:00', _UTF8MB4'%H %k %h %I %l')"},

		// for TIME_TO_SEC
		{"SELECT TIME_TO_SEC('22:23:00')", true, "SELECT TIME_TO_SEC(_UTF8MB4'22:23:00')"},

		// for TIMESTAMPADD
		{"SELECT TIMESTAMPADD(WEEK,1,'2003-01-02');", true, "SELECT TIMESTAMPADD(WEEK, 1, _UTF8MB4'2003-01-02')"},
		{"SELECT TIMESTAMPADD(SQL_TSI_SECOND,1,'2003-01-02');", true, "SELECT TIMESTAMPADD(SECOND, 1, _UTF8MB4'2003-01-02')"},
		{"SELECT TIMESTAMPADD(SQL_TSI_MINUTE,1,'2003-01-02');", true, "SELECT TIMESTAMPADD(MINUTE, 1, _UTF8MB4'2003-01-02')"},
		{"SELECT TIMESTAMPADD(SQL_TSI_HOUR,1,'2003-01-02');", true, "SELECT TIMESTAMPADD(HOUR, 1, _UTF8MB4'2003-01-02')"},
		{"SELECT TIMESTAMPADD(SQL_TSI_DAY,1,'2003-01-02');", true, "SELECT TIMESTAMPADD(DAY, 1, _UTF8MB4'2003-01-02')"},
		{"SELECT TIMESTAMPADD(SQL_TSI_WEEK,1,'2003-01-02');", true, "SELECT TIMESTAMPADD(WEEK, 1, _UTF8MB4'2003-01-02')"},
		{"SELECT TIMESTAMPADD(SQL_TSI_MONTH,1,'2003-01-02');", true, "SELECT TIMESTAMPADD(MONTH, 1, _UTF8MB4'2003-01-02')"},
		{"SELECT TIMESTAMPADD(SQL_TSI_QUARTER,1,'2003-01-02');", true, "SELECT TIMESTAMPADD(QUARTER, 1, _UTF8MB4'2003-01-02')"},
		{"SELECT TIMESTAMPADD(SQL_TSI_YEAR,1,'2003-01-02');", true, "SELECT TIMESTAMPADD(YEAR, 1, _UTF8MB4'2003-01-02')"},
		{"SELECT TIMESTAMPADD(SQL_TSI_MICROSECOND,1,'2003-01-02');", false, ""},
		{"SELECT TIMESTAMPADD(MICROSECOND,1,'2003-01-02');", true, "SELECT TIMESTAMPADD(MICROSECOND, 1, _UTF8MB4'2003-01-02')"},

		// for TO_DAYS, TO_SECONDS
		{"SELECT TO_DAYS('2007-10-07')", true, "SELECT TO_DAYS(_UTF8MB4'2007-10-07')"},
		{"SELECT TO_SECONDS('2009-11-29')", true, "SELECT TO_SECONDS(_UTF8MB4'2009-11-29')"},

		// for LAST_DAY
		{"SELECT LAST_DAY('2003-02-05');", true, "SELECT LAST_DAY(_UTF8MB4'2003-02-05')"},

		// for UTC_TIME
		{"SELECT UTC_TIME(), UTC_TIME(1)", true, "SELECT UTC_TIME(),UTC_TIME(1)"},

		// for time extract
		{`select extract(microsecond from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(MICROSECOND FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(second from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(SECOND FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(minute from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(MINUTE FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(hour from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(HOUR FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(day from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(DAY FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(week from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(WEEK FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(month from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(MONTH FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(quarter from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(QUARTER FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(year from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(YEAR FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(second_microsecond from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(SECOND_MICROSECOND FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(minute_microsecond from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(MINUTE_MICROSECOND FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(minute_second from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(MINUTE_SECOND FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(hour_microsecond from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(HOUR_MICROSECOND FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(hour_second from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(HOUR_SECOND FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(hour_minute from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(HOUR_MINUTE FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(day_microsecond from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(DAY_MICROSECOND FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(day_second from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(DAY_SECOND FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(day_minute from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(DAY_MINUTE FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(day_hour from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(DAY_HOUR FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},
		{`select extract(year_month from "2011-11-11 10:10:10.123456")`, true, "SELECT EXTRACT(YEAR_MONTH FROM _UTF8MB4'2011-11-11 10:10:10.123456')"},

		// for from_unixtime
		{`select from_unixtime(1447430881)`, true, "SELECT FROM_UNIXTIME(1447430881)"},
		{`select from_unixtime(1447430881.123456)`, true, "SELECT FROM_UNIXTIME(1447430881.123456)"},
		{`select from_unixtime(1447430881.1234567)`, true, "SELECT FROM_UNIXTIME(1447430881.1234567)"},
		{`select from_unixtime(1447430881.9999999)`, true, "SELECT FROM_UNIXTIME(1447430881.9999999)"},
		{`select from_unixtime(1447430881, "%Y %D %M %h:%i:%s %x")`, true, "SELECT FROM_UNIXTIME(1447430881, _UTF8MB4'%Y %D %M %h:%i:%s %x')"},
		{`select from_unixtime(1447430881.123456, "%Y %D %M %h:%i:%s %x")`, true, "SELECT FROM_UNIXTIME(1447430881.123456, _UTF8MB4'%Y %D %M %h:%i:%s %x')"},
		{`select from_unixtime(1447430881.1234567, "%Y %D %M %h:%i:%s %x")`, true, "SELECT FROM_UNIXTIME(1447430881.1234567, _UTF8MB4'%Y %D %M %h:%i:%s %x')"},

		// for issue 224
		{`SELECT CAST('test collated returns' AS CHAR CHARACTER SET utf8) COLLATE utf8_bin;`, true, "SELECT CAST(_UTF8MB4'test collated returns' AS CHAR CHARSET UTF8) COLLATE utf8_bin"},

		// for string functions
		// trim
		{`SELECT TRIM('  bar   ');`, true, "SELECT TRIM(_UTF8MB4'  bar   ')"},
		{`SELECT TRIM(LEADING 'x' FROM 'xxxbarxxx');`, true, "SELECT TRIM(LEADING _UTF8MB4'x' FROM _UTF8MB4'xxxbarxxx')"},
		{`SELECT TRIM(BOTH 'x' FROM 'xxxbarxxx');`, true, "SELECT TRIM(BOTH _UTF8MB4'x' FROM _UTF8MB4'xxxbarxxx')"},
		{`SELECT TRIM(TRAILING 'xyz' FROM 'barxxyz');`, true, "SELECT TRIM(TRAILING _UTF8MB4'xyz' FROM _UTF8MB4'barxxyz')"},
		{`SELECT LTRIM(' foo ');`, true, "SELECT LTRIM(_UTF8MB4' foo ')"},
		{`SELECT RTRIM(' bar ');`, true, "SELECT RTRIM(_UTF8MB4' bar ')"},

		{`SELECT RPAD('hi', 6, 'c');`, true, "SELECT RPAD(_UTF8MB4'hi', 6, _UTF8MB4'c')"},
		{`SELECT BIT_LENGTH('hi');`, true, "SELECT BIT_LENGTH(_UTF8MB4'hi')"},
		{`SELECT CHAR(65);`, true, "SELECT CHAR_FUNC(65, NULL)"},
		{`SELECT CHAR_LENGTH('abc');`, true, "SELECT CHAR_LENGTH(_UTF8MB4'abc')"},
		{`SELECT CHARACTER_LENGTH('abc');`, true, "SELECT CHARACTER_LENGTH(_UTF8MB4'abc')"},
		{`SELECT FIELD('ej', 'Hej', 'ej', 'Heja', 'hej', 'foo');`, true, "SELECT FIELD(_UTF8MB4'ej', _UTF8MB4'Hej', _UTF8MB4'ej', _UTF8MB4'Heja', _UTF8MB4'hej', _UTF8MB4'foo')"},
		{`SELECT FIND_IN_SET('foo', 'foo,bar')`, true, "SELECT FIND_IN_SET(_UTF8MB4'foo', _UTF8MB4'foo,bar')"},
		{`SELECT FIND_IN_SET('foo')`, true, "SELECT FIND_IN_SET(_UTF8MB4'foo')"}, // illegal number of argument still pass
		{`SELECT MAKE_SET(1,'a'), MAKE_SET(1,'a','b','c')`, true, "SELECT MAKE_SET(1, _UTF8MB4'a'),MAKE_SET(1, _UTF8MB4'a', _UTF8MB4'b', _UTF8MB4'c')"},
		{`SELECT MID('Sakila', -5, 3)`, true, "SELECT MID(_UTF8MB4'Sakila', -5, 3)"},
		{`SELECT OCT(12)`, true, "SELECT OCT(12)"},
		{`SELECT OCTET_LENGTH('text')`, true, "SELECT OCTET_LENGTH(_UTF8MB4'text')"},
		{`SELECT ORD('2')`, true, "SELECT ORD(_UTF8MB4'2')"},
		{`SELECT POSITION('bar' IN 'foobarbar')`, true, "SELECT POSITION(_UTF8MB4'bar' IN _UTF8MB4'foobarbar')"},
		{`SELECT QUOTE('Don\'t!')`, true, "SELECT QUOTE(_UTF8MB4'Don''t!')"},
		{`SELECT BIN(12)`, true, "SELECT BIN(12)"},
		{`SELECT ELT(1, 'ej', 'Heja', 'hej', 'foo')`, true, "SELECT ELT(1, _UTF8MB4'ej', _UTF8MB4'Heja', _UTF8MB4'hej', _UTF8MB4'foo')"},
		{`SELECT EXPORT_SET(5,'Y','N'), EXPORT_SET(5,'Y','N',','), EXPORT_SET(5,'Y','N',',',4)`, true, "SELECT EXPORT_SET(5, _UTF8MB4'Y', _UTF8MB4'N'),EXPORT_SET(5, _UTF8MB4'Y', _UTF8MB4'N', _UTF8MB4','),EXPORT_SET(5, _UTF8MB4'Y', _UTF8MB4'N', _UTF8MB4',', 4)"},
		{`SELECT FORMAT(), FORMAT(12332.2,2,'de_DE'), FORMAT(12332.123456, 4)`, true, "SELECT FORMAT(),FORMAT(12332.2, 2, _UTF8MB4'de_DE'),FORMAT(12332.123456, 4)"},
		{`SELECT FROM_BASE64('abc')`, true, "SELECT FROM_BASE64(_UTF8MB4'abc')"},
		{`SELECT TO_BASE64('abc')`, true, "SELECT TO_BASE64(_UTF8MB4'abc')"},
		{`SELECT INSERT(), INSERT('Quadratic', 3, 4, 'What'), INSTR('foobarbar', 'bar')`, true, "SELECT INSERT_FUNC(),INSERT_FUNC(_UTF8MB4'Quadratic', 3, 4, _UTF8MB4'What'),INSTR(_UTF8MB4'foobarbar', _UTF8MB4'bar')"},
		{`SELECT LOAD_FILE('/tmp/picture')`, true, "SELECT LOAD_FILE(_UTF8MB4'/tmp/picture')"},
		{`SELECT LPAD('hi',4,'??')`, true, "SELECT LPAD(_UTF8MB4'hi', 4, _UTF8MB4'??')"},
		{`SELECT LEFT("foobar", 3)`, true, "SELECT LEFT(_UTF8MB4'foobar', 3)"},
		{`SELECT RIGHT("foobar", 3)`, true, "SELECT RIGHT(_UTF8MB4'foobar', 3)"},

		// repeat
		{`SELECT REPEAT("a", 10);`, true, "SELECT REPEAT(_UTF8MB4'a', 10)"},

		// for miscellaneous functions
		{`SELECT SLEEP(10);`, true, "SELECT SLEEP(10)"},
		{`SELECT ANY_VALUE(@arg);`, true, "SELECT ANY_VALUE(@`arg`)"},
		{`SELECT INET_ATON('10.0.5.9');`, true, "SELECT INET_ATON(_UTF8MB4'10.0.5.9')"},
		{`SELECT INET_NTOA(167773449);`, true, "SELECT INET_NTOA(167773449)"},
		{`SELECT INET6_ATON('fdfe::5a55:caff:fefa:9089');`, true, "SELECT INET6_ATON(_UTF8MB4'fdfe::5a55:caff:fefa:9089')"},
		{`SELECT INET6_NTOA(INET_NTOA(167773449));`, true, "SELECT INET6_NTOA(INET_NTOA(167773449))"},
		{`SELECT IS_FREE_LOCK(@str);`, true, "SELECT IS_FREE_LOCK(@`str`)"},
		{`SELECT IS_IPV4('10.0.5.9');`, true, "SELECT IS_IPV4(_UTF8MB4'10.0.5.9')"},
		{`SELECT IS_IPV4_COMPAT(INET6_ATON('::10.0.5.9'));`, true, "SELECT IS_IPV4_COMPAT(INET6_ATON(_UTF8MB4'::10.0.5.9'))"},
		{`SELECT IS_IPV4_MAPPED(INET6_ATON('::10.0.5.9'));`, true, "SELECT IS_IPV4_MAPPED(INET6_ATON(_UTF8MB4'::10.0.5.9'))"},
		{`SELECT IS_IPV6('10.0.5.9');`, true, "SELECT IS_IPV6(_UTF8MB4'10.0.5.9')"},
		{`SELECT IS_USED_LOCK(@str);`, true, "SELECT IS_USED_LOCK(@`str`)"},
		{`SELECT NAME_CONST('myname', 14);`, true, "SELECT NAME_CONST(_UTF8MB4'myname', 14)"},
		{`SELECT RELEASE_ALL_LOCKS();`, true, "SELECT RELEASE_ALL_LOCKS()"},
		{`SELECT UUID();`, true, "SELECT UUID()"},
		{`SELECT UUID_SHORT()`, true, "SELECT UUID_SHORT()"},
		{`SELECT UUID_TO_BIN('6ccd780c-baba-1026-9564-5b8c656024db')`, true, "SELECT UUID_TO_BIN(_UTF8MB4'6ccd780c-baba-1026-9564-5b8c656024db')"},
		{`SELECT UUID_TO_BIN('6ccd780c-baba-1026-9564-5b8c656024db', 1)`, true, "SELECT UUID_TO_BIN(_UTF8MB4'6ccd780c-baba-1026-9564-5b8c656024db', 1)"},
		{`SELECT BIN_TO_UUID(0x6ccd780cbaba102695645b8c656024db)`, true, "SELECT BIN_TO_UUID(x'6ccd780cbaba102695645b8c656024db')"},
		{`SELECT BIN_TO_UUID(0x6ccd780cbaba102695645b8c656024db, 1)`, true, "SELECT BIN_TO_UUID(x'6ccd780cbaba102695645b8c656024db', 1)"},
		// test illegal arguments
		{`SELECT SLEEP();`, true, "SELECT SLEEP()"},
		{`SELECT ANY_VALUE();`, true, "SELECT ANY_VALUE()"},
		{`SELECT INET_ATON();`, true, "SELECT INET_ATON()"},
		{`SELECT INET_NTOA();`, true, "SELECT INET_NTOA()"},
		{`SELECT INET6_ATON();`, true, "SELECT INET6_ATON()"},
		{`SELECT INET6_NTOA(INET_NTOA());`, true, "SELECT INET6_NTOA(INET_NTOA())"},
		{`SELECT IS_FREE_LOCK();`, true, "SELECT IS_FREE_LOCK()"},
		{`SELECT IS_IPV4();`, true, "SELECT IS_IPV4()"},
		{`SELECT IS_IPV4_COMPAT(INET6_ATON());`, true, "SELECT IS_IPV4_COMPAT(INET6_ATON())"},
		{`SELECT IS_IPV4_MAPPED(INET6_ATON());`, true, "SELECT IS_IPV4_MAPPED(INET6_ATON())"},
		{`SELECT IS_IPV6()`, true, "SELECT IS_IPV6()"},
		{`SELECT IS_USED_LOCK();`, true, "SELECT IS_USED_LOCK()"},
		{`SELECT NAME_CONST();`, true, "SELECT NAME_CONST()"},
		{`SELECT RELEASE_ALL_LOCKS(1);`, true, "SELECT RELEASE_ALL_LOCKS(1)"},
		{`SELECT UUID(1);`, true, "SELECT UUID(1)"},
		{`SELECT UUID_SHORT(1)`, true, "SELECT UUID_SHORT(1)"},
		// interval
		{`select "2011-11-11 10:10:10.123456" + interval 10 second`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 SECOND)"},
		{`select "2011-11-11 10:10:10.123456" - interval 10 second`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 SECOND)"},
		{`select  interval 10 second + "2011-11-11 10:10:10.123456"`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 SECOND)"},
		// for date_add
		{`select date_add("2011-11-11 10:10:10.123456", interval 10 microsecond)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 MICROSECOND)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 10 second)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 SECOND)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 10 minute)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 MINUTE)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 10 hour)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 HOUR)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 10 day)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 DAY)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 1 week)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 WEEK)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 1 month)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 MONTH)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 1 quarter)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 QUARTER)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 1 year)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 YEAR)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "10.10" second_microsecond)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10.10' SECOND_MICROSECOND)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "10:10.10" minute_microsecond)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10.10' MINUTE_MICROSECOND)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "10:10" minute_second)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10' MINUTE_SECOND)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "10:10:10.10" hour_microsecond)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10:10.10' HOUR_MICROSECOND)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "10:10:10" hour_second)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10:10' HOUR_SECOND)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "10:10" hour_minute)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10' HOUR_MINUTE)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 10.10 hour_minute)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10.10 HOUR_MINUTE)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "11 10:10:10.10" day_microsecond)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10:10.10' DAY_MICROSECOND)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "11 10:10:10" day_second)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10:10' DAY_SECOND)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "11 10:10" day_minute)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10' DAY_MINUTE)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "11 10" day_hour)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10' DAY_HOUR)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval "11-11" year_month)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11-11' YEAR_MONTH)"},
		{`select date_add("2011-11-11 10:10:10.123456", 10)`, false, ""},
		{`select date_add("2011-11-11 10:10:10.123456", 0.10)`, false, ""},
		{`select date_add("2011-11-11 10:10:10.123456", "11,11")`, false, ""},

		{`select date_add("2011-11-11 10:10:10.123456", interval 10 sql_tsi_microsecond)`, false, ""},
		{`select date_add("2011-11-11 10:10:10.123456", interval 10 sql_tsi_second)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 SECOND)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 10 sql_tsi_minute)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 MINUTE)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 10 sql_tsi_hour)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 HOUR)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 10 sql_tsi_day)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 DAY)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 1 sql_tsi_week)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 WEEK)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 1 sql_tsi_month)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 MONTH)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 1 sql_tsi_quarter)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 QUARTER)"},
		{`select date_add("2011-11-11 10:10:10.123456", interval 1 sql_tsi_year)`, true, "SELECT DATE_ADD(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 YEAR)"},

		// for strcmp
		{`select strcmp('abc', 'def')`, true, "SELECT STRCMP(_UTF8MB4'abc', _UTF8MB4'def')"},

		// for adddate
		{`select adddate("2011-11-11 10:10:10.123456", interval 10 microsecond)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 MICROSECOND)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval 10 second)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 SECOND)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval 10 minute)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 MINUTE)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval 10 hour)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 HOUR)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval 10 day)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 DAY)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval 1 week)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 WEEK)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval 1 month)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 MONTH)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval 1 quarter)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 QUARTER)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval 1 year)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 YEAR)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "10.10" second_microsecond)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10.10' SECOND_MICROSECOND)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "10:10.10" minute_microsecond)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10.10' MINUTE_MICROSECOND)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "10:10" minute_second)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10' MINUTE_SECOND)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "10:10:10.10" hour_microsecond)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10:10.10' HOUR_MICROSECOND)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "10:10:10" hour_second)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10:10' HOUR_SECOND)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "10:10" hour_minute)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10' HOUR_MINUTE)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval 10.10 hour_minute)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10.10 HOUR_MINUTE)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "11 10:10:10.10" day_microsecond)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10:10.10' DAY_MICROSECOND)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "11 10:10:10" day_second)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10:10' DAY_SECOND)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "11 10:10" day_minute)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10' DAY_MINUTE)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "11 10" day_hour)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10' DAY_HOUR)"},
		{`select adddate("2011-11-11 10:10:10.123456", interval "11-11" year_month)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11-11' YEAR_MONTH)"},
		{`select adddate("2011-11-11 10:10:10.123456", 10)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 DAY)"},
		{`select adddate("2011-11-11 10:10:10.123456", 0.10)`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 0.10 DAY)"},
		{`select adddate("2011-11-11 10:10:10.123456", "11,11")`, true, "SELECT ADDDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11,11' DAY)"},

		// for date_sub
		{`select date_sub("2011-11-11 10:10:10.123456", interval 10 microsecond)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 MICROSECOND)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval 10 second)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 SECOND)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval 10 minute)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 MINUTE)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval 10 hour)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 HOUR)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval 10 day)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 DAY)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval 1 week)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 WEEK)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval 1 month)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 MONTH)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval 1 quarter)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 QUARTER)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval 1 year)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 YEAR)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "10.10" second_microsecond)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10.10' SECOND_MICROSECOND)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "10:10.10" minute_microsecond)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10.10' MINUTE_MICROSECOND)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "10:10" minute_second)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10' MINUTE_SECOND)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "10:10:10.10" hour_microsecond)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10:10.10' HOUR_MICROSECOND)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "10:10:10" hour_second)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10:10' HOUR_SECOND)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "10:10" hour_minute)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10' HOUR_MINUTE)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval 10.10 hour_minute)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10.10 HOUR_MINUTE)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "11 10:10:10.10" day_microsecond)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10:10.10' DAY_MICROSECOND)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "11 10:10:10" day_second)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10:10' DAY_SECOND)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "11 10:10" day_minute)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10' DAY_MINUTE)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "11 10" day_hour)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10' DAY_HOUR)"},
		{`select date_sub("2011-11-11 10:10:10.123456", interval "11-11" year_month)`, true, "SELECT DATE_SUB(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11-11' YEAR_MONTH)"},
		{`select date_sub("2011-11-11 10:10:10.123456", 10)`, false, ""},
		{`select date_sub("2011-11-11 10:10:10.123456", 0.10)`, false, ""},
		{`select date_sub("2011-11-11 10:10:10.123456", "11,11")`, false, ""},

		// for subdate
		{`select subdate("2011-11-11 10:10:10.123456", interval 10 microsecond)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 MICROSECOND)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval 10 second)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 SECOND)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval 10 minute)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 MINUTE)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval 10 hour)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 HOUR)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval 10 day)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 DAY)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval 1 week)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 WEEK)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval 1 month)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 MONTH)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval 1 quarter)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 QUARTER)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval 1 year)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 1 YEAR)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "10.10" second_microsecond)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10.10' SECOND_MICROSECOND)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "10:10.10" minute_microsecond)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10.10' MINUTE_MICROSECOND)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "10:10" minute_second)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10' MINUTE_SECOND)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "10:10:10.10" hour_microsecond)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10:10.10' HOUR_MICROSECOND)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "10:10:10" hour_second)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10:10' HOUR_SECOND)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "10:10" hour_minute)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'10:10' HOUR_MINUTE)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval 10.10 hour_minute)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10.10 HOUR_MINUTE)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "11 10:10:10.10" day_microsecond)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10:10.10' DAY_MICROSECOND)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "11 10:10:10" day_second)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10:10' DAY_SECOND)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "11 10:10" day_minute)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10:10' DAY_MINUTE)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "11 10" day_hour)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11 10' DAY_HOUR)"},
		{`select subdate("2011-11-11 10:10:10.123456", interval "11-11" year_month)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11-11' YEAR_MONTH)"},
		{`select subdate("2011-11-11 10:10:10.123456", 10)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 10 DAY)"},
		{`select subdate("2011-11-11 10:10:10.123456", 0.10)`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL 0.10 DAY)"},
		{`select subdate("2011-11-11 10:10:10.123456", "11,11")`, true, "SELECT SUBDATE(_UTF8MB4'2011-11-11 10:10:10.123456', INTERVAL _UTF8MB4'11,11' DAY)"},

		// for unix_timestamp
		{`select unix_timestamp()`, true, "SELECT UNIX_TIMESTAMP()"},
		{`select unix_timestamp('2015-11-13 10:20:19.012')`, true, "SELECT UNIX_TIMESTAMP(_UTF8MB4'2015-11-13 10:20:19.012')"},

		// for misc functions
		{`SELECT GET_LOCK('lock1',10);`, true, "SELECT GET_LOCK(_UTF8MB4'lock1', 10)"},
		{`SELECT RELEASE_LOCK('lock1');`, true, "SELECT RELEASE_LOCK(_UTF8MB4'lock1')"},

		// for aggregate functions
		{`select avg(), avg(c1,c2) from t;`, false, "SELECT AVG(),AVG(`c1`, `c2`) FROM `t`"},
		{`select avg(distinct c1) from t;`, true, "SELECT AVG(DISTINCT `c1`) FROM `t`"},
		{`select avg(distinctrow c1) from t;`, true, "SELECT AVG(DISTINCT `c1`) FROM `t`"},
		{`select avg(distinct all c1) from t;`, true, "SELECT AVG(DISTINCT `c1`) FROM `t`"},
		{`select avg(distinctrow all c1) from t;`, true, "SELECT AVG(DISTINCT `c1`) FROM `t`"},
		{`select avg(c2) from t;`, true, "SELECT AVG(`c2`) FROM `t`"},
		{`select bit_and(c1) from t;`, true, "SELECT BIT_AND(`c1`) FROM `t`"},
		{`select bit_and(all c1) from t;`, true, "SELECT BIT_AND(`c1`) FROM `t`"},
		{`select bit_and(distinct c1) from t;`, false, ""},
		{`select bit_and(distinctrow c1) from t;`, false, ""},
		{`select bit_and(distinctrow all c1) from t;`, false, ""},
		{`select bit_and(distinct all c1) from t;`, false, ""},
		{`select bit_and(), bit_and(distinct c1) from t;`, false, ""},
		{`select bit_and(), bit_and(distinctrow c1) from t;`, false, ""},
		{`select bit_and(), bit_and(all c1) from t;`, false, ""},
		{`select bit_or(c1) from t;`, true, "SELECT BIT_OR(`c1`) FROM `t`"},
		{`select bit_or(all c1) from t;`, true, "SELECT BIT_OR(`c1`) FROM `t`"},
		{`select bit_or(distinct c1) from t;`, false, ""},
		{`select bit_or(distinctrow c1) from t;`, false, ""},
		{`select bit_or(distinctrow all c1) from t;`, false, ""},
		{`select bit_or(distinct all c1) from t;`, false, ""},
		{`select bit_or(), bit_or(distinct c1) from t;`, false, ""},
		{`select bit_or(), bit_or(distinctrow c1) from t;`, false, ""},
		{`select bit_or(), bit_or(all c1) from t;`, false, ""},
		{`select bit_xor(c1) from t;`, true, "SELECT BIT_XOR(`c1`) FROM `t`"},
		{`select bit_xor(all c1) from t;`, true, "SELECT BIT_XOR(`c1`) FROM `t`"},
		{`select bit_xor(distinct c1) from t;`, false, ""},
		{`select bit_xor(distinctrow c1) from t;`, false, ""},
		{`select bit_xor(distinctrow all c1) from t;`, false, ""},
		{`select bit_xor(), bit_xor(distinct c1) from t;`, false, ""},
		{`select bit_xor(), bit_xor(distinctrow c1) from t;`, false, ""},
		{`select bit_xor(), bit_xor(all c1) from t;`, false, ""},
		{`select max(c1,c2) from t;`, false, ""},
		{`select max(distinct c1) from t;`, true, "SELECT MAX(DISTINCT `c1`) FROM `t`"},
		{`select max(distinctrow c1) from t;`, true, "SELECT MAX(DISTINCT `c1`) FROM `t`"},
		{`select max(distinct all c1) from t;`, true, "SELECT MAX(DISTINCT `c1`) FROM `t`"},
		{`select max(distinctrow all c1) from t;`, true, "SELECT MAX(DISTINCT `c1`) FROM `t`"},
		{`select max(c2) from t;`, true, "SELECT MAX(`c2`) FROM `t`"},
		{`select min(c1,c2) from t;`, false, ""},
		{`select min(distinct c1) from t;`, true, "SELECT MIN(DISTINCT `c1`) FROM `t`"},
		{`select min(distinctrow c1) from t;`, true, "SELECT MIN(DISTINCT `c1`) FROM `t`"},
		{`select min(distinct all c1) from t;`, true, "SELECT MIN(DISTINCT `c1`) FROM `t`"},
		{`select min(distinctrow all c1) from t;`, true, "SELECT MIN(DISTINCT `c1`) FROM `t`"},
		{`select min(c2) from t;`, true, "SELECT MIN(`c2`) FROM `t`"},
		{`select sum(c1,c2) from t;`, false, ""},
		{`select sum(distinct c1) from t;`, true, "SELECT SUM(DISTINCT `c1`) FROM `t`"},
		{`select sum(distinctrow c1) from t;`, true, "SELECT SUM(DISTINCT `c1`) FROM `t`"},
		{`select sum(distinct all c1) from t;`, true, "SELECT SUM(DISTINCT `c1`) FROM `t`"},
		{`select sum(distinctrow all c1) from t;`, true, "SELECT SUM(DISTINCT `c1`) FROM `t`"},
		{`select sum(c2) from t;`, true, "SELECT SUM(`c2`) FROM `t`"},
		{`select count(c1) from t;`, true, "SELECT COUNT(`c1`) FROM `t`"},
		{`select count(distinct *) from t;`, false, ""},
		{`select count(distinctrow *) from t;`, false, ""},
		{`select count(*) from t;`, true, "SELECT COUNT(1) FROM `t`"},
		{`select count(distinct c1, c2) from t;`, true, "SELECT COUNT(DISTINCT `c1`, `c2`) FROM `t`"},
		{`select count(distinctrow c1, c2) from t;`, true, "SELECT COUNT(DISTINCT `c1`, `c2`) FROM `t`"},
		{`select count(c1, c2) from t;`, false, ""},
		{`select count(all c1) from t;`, true, "SELECT COUNT(`c1`) FROM `t`"},
		{`select count(distinct all c1) from t;`, false, ""},
		{`select count(distinctrow all c1) from t;`, false, ""},
		{`select approx_count_distinct(c1) from t;`, true, "SELECT APPROX_COUNT_DISTINCT(`c1`) FROM `t`"},
		{`select approx_count_distinct(c1, c2) from t;`, true, "SELECT APPROX_COUNT_DISTINCT(`c1`, `c2`) FROM `t`"},
		{`select approx_count_distinct(c1, 123) from t;`, true, "SELECT APPROX_COUNT_DISTINCT(`c1`, 123) FROM `t`"},
		{`select approx_percentile(c1) from t;`, true, "SELECT APPROX_PERCENTILE(`c1`) FROM `t`"},
		{`select approx_percentile(c1, c2) from t;`, true, "SELECT APPROX_PERCENTILE(`c1`, `c2`) FROM `t`"},
		{`select approx_percentile(c1, 123) from t;`, true, "SELECT APPROX_PERCENTILE(`c1`, 123) FROM `t`"},
		{`select group_concat(c2,c1) from t group by c1;`, true, "SELECT GROUP_CONCAT(`c2`, `c1` SEPARATOR ',') FROM `t` GROUP BY `c1`"},
		{`select group_concat(c2,c1 SEPARATOR ';') from t group by c1;`, true, "SELECT GROUP_CONCAT(`c2`, `c1` SEPARATOR ';') FROM `t` GROUP BY `c1`"},
		{`select group_concat(distinct c2,c1) from t group by c1;`, true, "SELECT GROUP_CONCAT(DISTINCT `c2`, `c1` SEPARATOR ',') FROM `t` GROUP BY `c1`"},
		{`select group_concat(distinctrow c2,c1) from t group by c1;`, true, "SELECT GROUP_CONCAT(DISTINCT `c2`, `c1` SEPARATOR ',') FROM `t` GROUP BY `c1`"},
		{`SELECT student_name, GROUP_CONCAT(DISTINCT test_score ORDER BY test_score DESC SEPARATOR ' ') FROM student GROUP BY student_name;`, true, "SELECT `student_name`,GROUP_CONCAT(DISTINCT `test_score` ORDER BY `test_score` DESC SEPARATOR ' ') FROM `student` GROUP BY `student_name`"},
		{`select std(c1), std(all c1), std(distinct c1) from t`, true, "SELECT STDDEV_POP(`c1`),STDDEV_POP(`c1`),STDDEV_POP(DISTINCT `c1`) FROM `t`"},
		{`select std(c1, c2) from t`, false, ""},
		{`select stddev(c1), stddev(all c1), stddev(distinct c1) from t`, true, "SELECT STDDEV_POP(`c1`),STDDEV_POP(`c1`),STDDEV_POP(DISTINCT `c1`) FROM `t`"},
		{`select stddev(c1, c2) from t`, false, ""},
		{`select stddev_pop(c1), stddev_pop(all c1), stddev_pop(distinct c1) from t`, true, "SELECT STDDEV_POP(`c1`),STDDEV_POP(`c1`),STDDEV_POP(DISTINCT `c1`) FROM `t`"},
		{`select stddev_pop(c1, c2) from t`, false, ""},
		{`select stddev_samp(c1), stddev_samp(all c1), stddev_samp(distinct c1) from t`, true, "SELECT STDDEV_SAMP(`c1`),STDDEV_SAMP(`c1`),STDDEV_SAMP(DISTINCT `c1`) FROM `t`"},
		{`select stddev_samp(c1, c2) from t`, false, ""},
		{`select variance(c1), variance(all c1), variance(distinct c1) from t`, true, "SELECT VAR_POP(`c1`),VAR_POP(`c1`),VAR_POP(DISTINCT `c1`) FROM `t`"},
		{`select variance(c1, c2) from t`, false, ""},
		{`select var_pop(c1), var_pop(all c1), var_pop(distinct c1) from t`, true, "SELECT VAR_POP(`c1`),VAR_POP(`c1`),VAR_POP(DISTINCT `c1`) FROM `t`"},
		{`select var_pop(c1, c2) from t`, false, ""},
		{`select var_samp(c1), var_samp(all c1), var_samp(distinct c1) from t`, true, "SELECT VAR_SAMP(`c1`),VAR_SAMP(`c1`),VAR_SAMP(DISTINCT `c1`) FROM `t`"},
		{`select var_samp(c1, c2) from t`, false, ""},
		{`select json_arrayagg(c2) from t group by c1`, true, "SELECT JSON_ARRAYAGG(`c2`) FROM `t` GROUP BY `c1`"},
		{`select json_arrayagg(c1, c2) from t group by c1`, false, ""},
		{`select json_arrayagg(distinct c2) from t group by c1`, false, "SELECT JSON_ARRAYAGG(DISTINCT `c2`) FROM `t` GROUP BY `c1`"},
		{`select json_arrayagg(all c2) from t group by c1`, true, "SELECT JSON_ARRAYAGG(`c2`) FROM `t` GROUP BY `c1`"},
		{`select json_objectagg(c1, c2) from t group by c1`, true, "SELECT JSON_OBJECTAGG(`c1`, `c2`) FROM `t` GROUP BY `c1`"},
		{`select json_objectagg(c1, c2, c3) from t group by c1`, false, ""},
		{`select json_objectagg(distinct c1, c2) from t group by c1`, false, "SELECT JSON_OBJECTAGG(DISTINCT `c1`, `c2`) FROM `t` GROUP BY `c1`"},
		{`select json_objectagg(c1, distinct c2) from t group by c1`, false, "SELECT JSON_OBJECTAGG(`c1`, DISTINCT `c2`) FROM `t` GROUP BY `c1`"},
		{`select json_objectagg(distinct c1, distinct c2) from t group by c1`, false, "SELECT JSON_OBJECTAGG(DISTINCT `c1`, DISTINCT `c2`) FROM `t` GROUP BY `c1`"},
		{`select json_objectagg(all c1, c2) from t group by c1`, true, "SELECT JSON_OBJECTAGG(`c1`, `c2`) FROM `t` GROUP BY `c1`"},
		{`select json_objectagg(c1, all c2) from t group by c1`, true, "SELECT JSON_OBJECTAGG(`c1`, `c2`) FROM `t` GROUP BY `c1`"},
		{`select json_objectagg(all c1, all c2) from t group by c1`, true, "SELECT JSON_OBJECTAGG(`c1`, `c2`) FROM `t` GROUP BY `c1`"},

		// for encryption and compression functions
		{`select AES_ENCRYPT('text',UNHEX('F3229A0B371ED2D9441B830D21A390C3'))`, true, "SELECT AES_ENCRYPT(_UTF8MB4'text', UNHEX(_UTF8MB4'F3229A0B371ED2D9441B830D21A390C3'))"},
		{`select AES_DECRYPT(@crypt_str,@key_str)`, true, "SELECT AES_DECRYPT(@`crypt_str`, @`key_str`)"},
		{`select AES_DECRYPT(@crypt_str,@key_str,@init_vector);`, true, "SELECT AES_DECRYPT(@`crypt_str`, @`key_str`, @`init_vector`)"},
		{`SELECT COMPRESS('');`, true, "SELECT COMPRESS(_UTF8MB4'')"},
		{`SELECT DECODE(@crypt_str, @pass_str);`, true, "SELECT DECODE(@`crypt_str`, @`pass_str`)"},
		{`SELECT DES_DECRYPT(@crypt_str), DES_DECRYPT(@crypt_str, @key_str);`, true, "SELECT DES_DECRYPT(@`crypt_str`),DES_DECRYPT(@`crypt_str`, @`key_str`)"},
		{`SELECT DES_ENCRYPT(@str), DES_ENCRYPT(@key_num);`, true, "SELECT DES_ENCRYPT(@`str`),DES_ENCRYPT(@`key_num`)"},
		{`SELECT ENCODE('cleartext', CONCAT('my_random_salt','my_secret_password'));`, true, "SELECT ENCODE(_UTF8MB4'cleartext', CONCAT(_UTF8MB4'my_random_salt', _UTF8MB4'my_secret_password'))"},
		{`SELECT ENCRYPT('hello'), ENCRYPT('hello', @salt);`, true, "SELECT ENCRYPT(_UTF8MB4'hello'),ENCRYPT(_UTF8MB4'hello', @`salt`)"},
		{`SELECT MD5('testing');`, true, "SELECT MD5(_UTF8MB4'testing')"},
		{`SELECT OLD_PASSWORD(@str);`, true, "SELECT OLD_PASSWORD(@`str`)"},
		{`SELECT PASSWORD(@str);`, true, "SELECT PASSWORD(@`str`)"},
		{`SELECT RANDOM_BYTES(@len);`, true, "SELECT RANDOM_BYTES(@`len`)"},
		{`SELECT SHA1('abc');`, true, "SELECT SHA1(_UTF8MB4'abc')"},
		{`SELECT SHA('abc');`, true, "SELECT SHA(_UTF8MB4'abc')"},
		{`SELECT SHA2('abc', 224);`, true, "SELECT SHA2(_UTF8MB4'abc', 224)"},
		{`SELECT SM3('abc');`, true, "SELECT SM3(_UTF8MB4'abc')"},
		{`SELECT UNCOMPRESS('any string');`, true, "SELECT UNCOMPRESS(_UTF8MB4'any string')"},
		{`SELECT UNCOMPRESSED_LENGTH(@compressed_string);`, true, "SELECT UNCOMPRESSED_LENGTH(@`compressed_string`)"},
		{`SELECT VALIDATE_PASSWORD_STRENGTH(@str);`, true, "SELECT VALIDATE_PASSWORD_STRENGTH(@`str`)"},

		// For JSON functions.
		{`SELECT JSON_EXTRACT();`, true, "SELECT JSON_EXTRACT()"},
		{`SELECT JSON_UNQUOTE();`, true, "SELECT JSON_UNQUOTE()"},
		{`SELECT JSON_TYPE('[123]');`, true, "SELECT JSON_TYPE(_UTF8MB4'[123]')"},
		{`SELECT JSON_TYPE();`, true, "SELECT JSON_TYPE()"},

		// For two json grammar sugar.
		{`SELECT a->'$.a' FROM t`, true, "SELECT JSON_EXTRACT(`a`, _UTF8MB4'$.a') FROM `t`"},
		{`SELECT a->>'$.a' FROM t`, true, "SELECT JSON_UNQUOTE(JSON_EXTRACT(`a`, _UTF8MB4'$.a')) FROM `t`"},
		{`SELECT '{}'->'$.a' FROM t`, false, ""},
		{`SELECT '{}'->>'$.a' FROM t`, false, ""},
		{`SELECT a->3 FROM t`, false, ""},
		{`SELECT a->>3 FROM t`, false, ""},

		{`SELECT 1 member of (a)`, true, "SELECT 1 MEMBER OF (`a`)"},
		{`SELECT 1 member of a`, false, ""},
		{`SELECT 1 member a`, false, ""},
		{`SELECT 1 not member of a`, false, ""},
		{`SELECT 1 member of (1+1)`, false, ""},
		{`SELECT concat('a') member of (cast(1 as char(1)))`, true, "SELECT CONCAT(_UTF8MB4'a') MEMBER OF (CAST(1 AS CHAR(1)))"},

		// Test that quoted identifier can be a function name.
		{"SELECT `uuid`()", true, "SELECT UUID()"},

		// Test sequence function.
		{"select nextval(seq)", true, "SELECT NEXTVAL(`seq`)"},
		{"select lastval(seq)", true, "SELECT LASTVAL(`seq`)"},
		{"select setval(seq, 100)", true, "SELECT SETVAL(`seq`, 100)"},
		{"select next value for seq", true, "SELECT NEXTVAL(`seq`)"},
		{"select next value for sequence", true, "SELECT NEXTVAL(`sequence`)"},
		{"select NeXt vAluE for seQuEncE2", true, "SELECT NEXTVAL(`seQuEncE2`)"},

		// Test regexp functions
		{"select regexp_like('aBc', 'abc', 'im');", true, "SELECT REGEXP_LIKE(_UTF8MB4'aBc', _UTF8MB4'abc', _UTF8MB4'im')"},
		{"select regexp_substr('aBc', 'abc', 1, 1, 'im');", true, "SELECT REGEXP_SUBSTR(_UTF8MB4'aBc', _UTF8MB4'abc', 1, 1, _UTF8MB4'im')"},
		{"select regexp_instr('aBc', 'abc', 1, 1, 0, 'im');", true, "SELECT REGEXP_INSTR(_UTF8MB4'aBc', _UTF8MB4'abc', 1, 1, 0, _UTF8MB4'im')"},
		{"select regexp_replace('aBc', 'abc', 'def', 1, 1, 'i');", true, "SELECT REGEXP_REPLACE(_UTF8MB4'aBc', _UTF8MB4'abc', _UTF8MB4'def', 1, 1, _UTF8MB4'i')"},

		// Test ilike functions
		{"select 'aBc' ilike 'abc';", true, "SELECT _UTF8MB4'aBc' ILIKE _UTF8MB4'abc'"},
	}
	RunTest(t, table, false)

	// Test in REAL_AS_FLOAT SQL mode.
	table2 := []testCase{
		// for cast as float
		{"select cast(1 as float);", true, "SELECT CAST(1 AS FLOAT)"},
		{"select cast(1 as float(0));", true, "SELECT CAST(1 AS FLOAT)"},
		{"select cast(1 as float(24));", true, "SELECT CAST(1 AS FLOAT)"},
		{"select cast(1 as float(25));", true, "SELECT CAST(1 AS DOUBLE)"},
		{"select cast(1 as float(53));", true, "SELECT CAST(1 AS DOUBLE)"},
		{"select cast(1 as float(54));", false, ""},

		// for cast as real
		{"select cast(1 as real);", true, "SELECT CAST(1 AS FLOAT)"},
	}
	RunTestInRealAsFloatMode(t, table2, false)
}

func TestIdentifier(t *testing.T) {
	table := []testCase{
		// for quote identifier
		{"select `a`, `a.b`, `a b` from t", true, "SELECT `a`,`a.b`,`a b` FROM `t`"},
		// for unquoted identifier
		{"create table MergeContextTest$Simple (value integer not null, primary key (value))", true, "CREATE TABLE `MergeContextTest$Simple` (`value` INT NOT NULL,PRIMARY KEY(`value`))"},
		// for as
		{"select 1 as a, 1 as `a`, 1 as \"a\", 1 as 'a'", true, "SELECT 1 AS `a`,1 AS `a`,1 AS `a`,1 AS `a`"},
		{`select 1 as a, 1 as "a", 1 as 'a'`, true, "SELECT 1 AS `a`,1 AS `a`,1 AS `a`"},
		{`select 1 a, 1 "a", 1 'a'`, true, "SELECT 1 AS `a`,1 AS `a`,1 AS `a`"},
		{`select * from t as "a"`, false, ""},
		{`select * from t a`, true, "SELECT * FROM `t` AS `a`"},
		// reserved keyword can't be used as identifier directly, but A.B pattern is an exception
		{`select * from ROW`, false, ""},
		{`select COUNT from DESC`, false, ""},
		{`select COUNT from SELECT.DESC`, true, "SELECT `COUNT` FROM `SELECT`.`DESC`"},
		{"use `select`", true, "USE `select`"},
		{"use `sel``ect`", true, "USE `sel``ect`"}, //nolint: misspell
		{"use select", false, "USE `select`"},
		{`select * from t as a`, true, "SELECT * FROM `t` AS `a`"},
		{"select 1 full, 1 row, 1 abs", false, ""},
		{"select 1 full, 1 `row`, 1 abs", true, "SELECT 1 AS `full`,1 AS `row`,1 AS `abs`"},
		{"select * from t full, t1 row, t2 abs", false, ""},
		{"select * from t full, t1 `row`, t2 abs", true, "SELECT * FROM ((`t` AS `full`) JOIN `t1` AS `row`) JOIN `t2` AS `abs`"},
		// for issue 1878, identifiers may begin with digit.
		{"create database 123test", true, "CREATE DATABASE `123test`"},
		{"create database 123", false, "CREATE DATABASE `123`"},
		{"create database `123`", true, "CREATE DATABASE `123`"},
		{"create database `12``3`", true, "CREATE DATABASE `12``3`"},
		{"create table `123` (123a1 int)", true, "CREATE TABLE `123` (`123a1` INT)"},
		{"create table 123 (123a1 int)", false, ""},
		{fmt.Sprintf("select * from t%cble", 0), false, ""},
		// for issue 3954, should NOT be recognized as identifiers.
		{`select .78+123`, true, "SELECT 0.78+123"},
		{`select .78+.21`, true, "SELECT 0.78+0.21"},
		{`select .78-123`, true, "SELECT 0.78-123"},
		{`select .78-.21`, true, "SELECT 0.78-0.21"},
		{`select .78--123`, true, "SELECT 0.78--123"},
		{`select .78*123`, true, "SELECT 0.78*123"},
		{`select .78*.21`, true, "SELECT 0.78*0.21"},
		{`select .78/123`, true, "SELECT 0.78/123"},
		{`select .78/.21`, true, "SELECT 0.78/0.21"},
		{`select .78,123`, true, "SELECT 0.78,123"},
		{`select .78,.21`, true, "SELECT 0.78,0.21"},
		{`select .78 , 123`, true, "SELECT 0.78,123"},
		{`select .78.123`, false, ""},
		{`select .78#123`, true, "SELECT 0.78"},
		{`insert float_test values(.67, 'string');`, true, "INSERT INTO `float_test` VALUES (0.67,_UTF8MB4'string')"},
		{`select .78'123'`, true, "SELECT 0.78 AS `123`"},
		{"select .78`123`", true, "SELECT 0.78 AS `123`"},
		{`select .78"123"`, true, "SELECT 0.78 AS `123`"},
		{"select 111 as \xd6\xf7", true, "SELECT 111 AS `??`"},
	}
	RunTest(t, table, false)
}

func TestBuiltinFuncAsIdentifier(t *testing.T) {
	whitespaceFuncs := []struct {
		funcName string
		args     string
	}{
		{"BIT_AND", "`c1`"},
		{"BIT_OR", "`c1`"},
		{"BIT_XOR", "`c1`"},
		{"CAST", "1 AS FLOAT"},
		{"COUNT", "1"},
		{"CURDATE", ""},
		{"CURTIME", ""},
		{"DATE_ADD", "_UTF8MB4'2011-11-11 10:10:10', INTERVAL 10 SECOND"},
		{"DATE_SUB", "_UTF8MB4'2011-11-11 10:10:10', INTERVAL 10 SECOND"},
		{"EXTRACT", "SECOND FROM _UTF8MB4'2011-11-11 10:10:10'"},
		{"GROUP_CONCAT", "`c2`, `c1` SEPARATOR ','"},
		{"MAX", "`c1`"},
		{"MID", "_UTF8MB4'Sakila', -5, 3"},
		{"MIN", "`c1`"},
		{"NOW", ""},
		{"POSITION", "_UTF8MB4'bar' IN _UTF8MB4'foobarbar'"},
		{"STDDEV_POP", "`c1`"},
		{"STDDEV_SAMP", "`c1`"},
		{"SUBSTR", "_UTF8MB4'Quadratically', 5"},
		{"SUBSTRING", "_UTF8MB4'Quadratically', 5"},
		{"SUM", "`c1`"},
		{"SYSDATE", ""},
		{"TRIM", "_UTF8MB4' foo '"},
		{"VAR_POP", "`c1`"},
		{"VAR_SAMP", "`c1`"},
	}

	testcases := make([]testCase, 0, 3*len(whitespaceFuncs))
	runTests := func(ignoreSpace bool) {
		p := parser.New()
		if ignoreSpace {
			p.SetSQLMode(mysql.ModeIgnoreSpace)
		}
		for _, c := range testcases {
			_, _, err := p.Parse(c.src, "", "")
			if !c.ok {
				require.Errorf(t, err, "source %v", c.src)
				continue
			}
			require.NoErrorf(t, err, "source %v", c.src)
			if c.ok && !ignoreSpace {
				RunRestoreTest(t, c.src, c.restore, false)
			}
		}
	}

	for _, function := range whitespaceFuncs {
		// `x` is recognized as a function name for `x()`.
		testcases = append(testcases, testCase{fmt.Sprintf("select %s(%s)", function.funcName, function.args), true, fmt.Sprintf("SELECT %s(%s)", function.funcName, function.args)})

		// In MySQL, `select x ()` is recognized as a stored function.
		// In TiDB, most of these functions are recognized as identifiers while some are builtin functions (such as COUNT, CURDATE)
		// because the later ones are not added to the token map. We'd better not to modify it since it breaks compatibility.
		// For example, `select CURDATE ()` reports an error in MySQL but it works well for TiDB.

		// `x` is recognized as an identifier for `x ()`.
		testcases = append(testcases, testCase{fmt.Sprintf("create table %s (a int)", function.funcName), true, fmt.Sprintf("CREATE TABLE `%s` (`a` INT)", function.funcName)})

		// `x` is recognized as a function name for `x()`.
		testcases = append(testcases, testCase{fmt.Sprintf("create table %s(a int)", function.funcName), false, ""})
	}
	runTests(false)

	testcases = make([]testCase, 0, 4*len(whitespaceFuncs))
	for _, function := range whitespaceFuncs {
		testcases = append(testcases, testCase{fmt.Sprintf("select %s(%s)", function.funcName, function.args), true, fmt.Sprintf("SELECT %s(%s)", function.funcName, function.args)})
		testcases = append(testcases, testCase{fmt.Sprintf("select %s (%s)", function.funcName, function.args), true, fmt.Sprintf("SELECT %s(%s)", function.funcName, function.args)})
		testcases = append(testcases, testCase{fmt.Sprintf("create table %s (a int)", function.funcName), false, ""})
		testcases = append(testcases, testCase{fmt.Sprintf("create table %s(a int)", function.funcName), false, ""})
	}
	runTests(true)

	normalFuncs := []struct {
		funcName string
		args     string
	}{
		{"ADDDATE", "_UTF8MB4'2011-11-11 10:10:10', INTERVAL 10 SECOND"},
		{"SESSION_USER", ""},
		{"SUBDATE", "_UTF8MB4'2011-11-11 10:10:10', INTERVAL 10 SECOND"},
		{"SYSTEM_USER", ""},
	}

	testcases = make([]testCase, 0, 4*len(normalFuncs))
	for _, function := range normalFuncs {
		// `x` is recognized as a function name for `select x()`.
		testcases = append(testcases, testCase{fmt.Sprintf("select %s(%s)", function.funcName, function.args), true, fmt.Sprintf("SELECT %s(%s)", function.funcName, function.args)})

		// `x` is recognized as a function name for `select x ()`.
		testcases = append(testcases, testCase{fmt.Sprintf("select %s (%s)", function.funcName, function.args), true, fmt.Sprintf("SELECT %s(%s)", function.funcName, function.args)})

		// `x` is recognized as an identifier for `create table x ()`.
		testcases = append(testcases, testCase{fmt.Sprintf("create table %s (a int)", function.funcName), true, fmt.Sprintf("CREATE TABLE `%s` (`a` INT)", function.funcName)})

		// `x` is recognized as an identifier for `create table x()`.
		testcases = append(testcases, testCase{fmt.Sprintf("create table %s(a int)", function.funcName), true, fmt.Sprintf("CREATE TABLE `%s` (`a` INT)", function.funcName)})
	}
	runTests(false)
	runTests(true)
}

func TestDDL(t *testing.T) {
	table := []testCase{
		{"CREATE", false, ""},
		{"CREATE TABLE", false, ""},
		{"CREATE TABLE foo (", false, ""},
		{"CREATE TABLE foo ()", false, ""},
		{"CREATE TABLE foo ();", false, ""},
		{"CREATE TABLE foo.* (a varchar(50), b int);", false, ""},
		{"CREATE TABLE foo (a varchar(50), b int);", true, "CREATE TABLE `foo` (`a` VARCHAR(50),`b` INT)"},
		{"CREATE TABLE foo (a TINYINT UNSIGNED);", true, "CREATE TABLE `foo` (`a` TINYINT UNSIGNED)"},
		{"CREATE TABLE foo (a SMALLINT UNSIGNED, b INT UNSIGNED)", true, "CREATE TABLE `foo` (`a` SMALLINT UNSIGNED,`b` INT UNSIGNED)"},
		{"CREATE TABLE foo (a bigint unsigned, b bool);", true, "CREATE TABLE `foo` (`a` BIGINT UNSIGNED,`b` TINYINT(1))"},
		{"CREATE TABLE foo (a TINYINT, b SMALLINT) CREATE TABLE bar (x INT, y int64)", false, ""},
		{"CREATE TABLE foo (a int, b float); CREATE TABLE bar (x double, y float)", true, "CREATE TABLE `foo` (`a` INT,`b` FLOAT); CREATE TABLE `bar` (`x` DOUBLE,`y` FLOAT)"},
		{"CREATE TABLE foo (a bytes)", false, ""},
		{"CREATE TABLE foo (a SMALLINT UNSIGNED, b INT UNSIGNED)", true, "CREATE TABLE `foo` (`a` SMALLINT UNSIGNED,`b` INT UNSIGNED)"},
		{"CREATE TABLE foo (a SMALLINT UNSIGNED, b INT UNSIGNED) -- foo", true, "CREATE TABLE `foo` (`a` SMALLINT UNSIGNED,`b` INT UNSIGNED)"},
		{"CREATE TABLE foo (a SMALLINT UNSIGNED, b INT UNSIGNED) // foo", false, ""},
		{"CREATE TABLE foo (a SMALLINT UNSIGNED, b INT UNSIGNED) /* foo */", true, "CREATE TABLE `foo` (`a` SMALLINT UNSIGNED,`b` INT UNSIGNED)"},
		{"CREATE TABLE foo /* foo */ (a SMALLINT UNSIGNED, b INT UNSIGNED) /* foo */", true, "CREATE TABLE `foo` (`a` SMALLINT UNSIGNED,`b` INT UNSIGNED)"},
		{"CREATE TABLE foo (name CHAR(50) BINARY);", true, "CREATE TABLE `foo` (`name` CHAR(50) BINARY)"},
		{"CREATE TABLE foo (name CHAR(50) COLLATE utf8_bin)", true, "CREATE TABLE `foo` (`name` CHAR(50) COLLATE utf8_bin)"},
		{"CREATE TABLE foo (id varchar(50) collate utf8_bin);", true, "CREATE TABLE `foo` (`id` VARCHAR(50) COLLATE utf8_bin)"},
		{"CREATE TABLE foo (name CHAR(50) CHARACTER SET UTF8)", true, "CREATE TABLE `foo` (`name` CHAR(50) CHARACTER SET UTF8)"},
		{"CREATE TABLE foo (name CHAR(50) CHARACTER SET utf8 BINARY)", true, "CREATE TABLE `foo` (`name` CHAR(50) BINARY CHARACTER SET UTF8)"},
		{"CREATE TABLE foo (name CHAR(50) CHARACTER SET utf8 BINARY CHARACTER set utf8)", false, ""},
		{"CREATE TABLE foo (name CHAR(50) BINARY CHARACTER SET utf8 COLLATE utf8_bin)", true, "CREATE TABLE `foo` (`name` CHAR(50) BINARY CHARACTER SET UTF8 COLLATE utf8_bin)"},
		{"CREATE TABLE foo (name CHAR(50) CHARACTER SET utf8 COLLATE utf8_bin COLLATE ascii_bin)", false, ""},
		{"CREATE TABLE foo (name CHAR(50) COLLATE ascii_bin COLLATE latin1_bin)", false, ""},
		{"CREATE TABLE foo (name CHAR(50) COLLATE ascii_bin PRIMARY KEY COLLATE latin1_bin)", false, ""},
		{"CREATE TABLE foo (a.b, b);", false, ""},
		{"CREATE TABLE foo (a, b.c);", false, ""},
		{"CREATE TABLE (name CHAR(50) BINARY)", false, ""},
		{"CREATE TABLE foo (name CHAR(50) COLLATE ascii_bin PRIMARY KEY COLLATE latin1_bin, INDEX (name ASC))", false, ""},
		{"CREATE TABLE foo (name CHAR(50) COLLATE ascii_bin PRIMARY KEY COLLATE latin1_bin, INDEX (name DESC))", false, ""},
		// test enable or disable cached table
		{"ALTER TABLE tmp CACHE", true, "ALTER TABLE `tmp` CACHE"},
		{"ALTER TABLE tmp NOCACHE", true, "ALTER TABLE `tmp` NOCACHE"},
		// for create temporary table
		{"CREATE TEMPORARY TABLE t (a varchar(50), b int);", true, "CREATE TEMPORARY TABLE `t` (`a` VARCHAR(50),`b` INT)"},
		{"CREATE TEMPORARY TABLE t LIKE t1", true, "CREATE TEMPORARY TABLE `t` LIKE `t1`"},
		{"DROP TEMPORARY TABLE t", true, "DROP TEMPORARY TABLE `t`"},
		{"create global temporary table t (a int, b varchar(255)) on commit delete rows", true, "CREATE GLOBAL TEMPORARY TABLE `t` (`a` INT,`b` VARCHAR(255)) ON COMMIT DELETE ROWS"},
		{"create temporary table t (a int, b varchar(255))", true, "CREATE TEMPORARY TABLE `t` (`a` INT,`b` VARCHAR(255))"},
		{"create global temporary table t (a int, b varchar(255))", false, ""}, // missing on commit delete rows
		{"create temporary table t (a int, b varchar(255)) on commit delete rows", false, ""},
		{"create table t (a int, b varchar(255)) on commit delete rows", false, ""},
		{"create global temporary table t (a int, b varchar(255)) partition by hash(a) partitions 10 on commit delete rows", true, "CREATE GLOBAL TEMPORARY TABLE `t` (`a` INT,`b` VARCHAR(255)) PARTITION BY HASH (`a`) PARTITIONS 10 ON COMMIT DELETE ROWS"},
		{"create global temporary table t (a int, b varchar(255)) on commit preserve rows", true, "CREATE GLOBAL TEMPORARY TABLE `t` (`a` INT,`b` VARCHAR(255)) ON COMMIT PRESERVE ROWS"},
		{"drop global temporary table t", true, "DROP GLOBAL TEMPORARY TABLE `t`"},
		{"drop temporary table t", true, "DROP TEMPORARY TABLE `t`"},
		// test use key word as column name
		{"CREATE TABLE foo (node_id varchar(50), b int);", true, "CREATE TABLE `foo` (`node_id` VARCHAR(50),`b` INT)"},
		{"CREATE TABLE foo (node_state varchar(50), b int);", true, "CREATE TABLE `foo` (`node_state` VARCHAR(50),`b` INT)"},
		// for table option
		{"create table t (c int) avg_row_length = 3", true, "CREATE TABLE `t` (`c` INT) AVG_ROW_LENGTH = 3"},
		{"create table t (c int) avg_row_length 3", true, "CREATE TABLE `t` (`c` INT) AVG_ROW_LENGTH = 3"},
		{"create table t (c int) checksum = 0", true, "CREATE TABLE `t` (`c` INT) CHECKSUM = 0"},
		{"create table t (c int) checksum 1", true, "CREATE TABLE `t` (`c` INT) CHECKSUM = 1"},
		{"create table t (c int) table_checksum = 0", true, "CREATE TABLE `t` (`c` INT) TABLE_CHECKSUM = 0"},
		{"create table t (c int) table_checksum 1", true, "CREATE TABLE `t` (`c` INT) TABLE_CHECKSUM = 1"},
		{"create table t (c int) compression = 'NONE'", true, "CREATE TABLE `t` (`c` INT) COMPRESSION = 'NONE'"},
		{"create table t (c int) compression 'lz4'", true, "CREATE TABLE `t` (`c` INT) COMPRESSION = 'lz4'"},
		{"create table t (c int) connection = 'abc'", true, "CREATE TABLE `t` (`c` INT) CONNECTION = 'abc'"},
		{"create table t (c int) connection 'abc'", true, "CREATE TABLE `t` (`c` INT) CONNECTION = 'abc'"},
		{"create table t (c int) key_block_size = 1024", true, "CREATE TABLE `t` (`c` INT) KEY_BLOCK_SIZE = 1024"},
		{"create table t (c int) key_block_size 1024", true, "CREATE TABLE `t` (`c` INT) KEY_BLOCK_SIZE = 1024"},
		{"create table t (c int) max_rows = 1000", true, "CREATE TABLE `t` (`c` INT) MAX_ROWS = 1000"},
		{"create table t (c int) max_rows 1000", true, "CREATE TABLE `t` (`c` INT) MAX_ROWS = 1000"},
		{"create table t (c int) min_rows = 1000", true, "CREATE TABLE `t` (`c` INT) MIN_ROWS = 1000"},
		{"create table t (c int) min_rows 1000", true, "CREATE TABLE `t` (`c` INT) MIN_ROWS = 1000"},
		{"create table t (c int) password = 'abc'", true, "CREATE TABLE `t` (`c` INT) PASSWORD = 'abc'"},
		{"create table t (c int) password 'abc'", true, "CREATE TABLE `t` (`c` INT) PASSWORD = 'abc'"},
		{"create table t (c int) DELAY_KEY_WRITE=1", true, "CREATE TABLE `t` (`c` INT) DELAY_KEY_WRITE = 1"},
		{"create table t (c int) DELAY_KEY_WRITE 1", true, "CREATE TABLE `t` (`c` INT) DELAY_KEY_WRITE = 1"},
		{"create table t (c int) ROW_FORMAT = default", true, "CREATE TABLE `t` (`c` INT) ROW_FORMAT = DEFAULT"},
		{"create table t (c int) ROW_FORMAT default", true, "CREATE TABLE `t` (`c` INT) ROW_FORMAT = DEFAULT"},
		{"create table t (c int) ROW_FORMAT = fixed", true, "CREATE TABLE `t` (`c` INT) ROW_FORMAT = FIXED"},
		{"create table t (c int) ROW_FORMAT = compressed", true, "CREATE TABLE `t` (`c` INT) ROW_FORMAT = COMPRESSED"},
		{"create table t (c int) ROW_FORMAT = compact", true, "CREATE TABLE `t` (`c` INT) ROW_FORMAT = COMPACT"},
		{"create table t (c int) ROW_FORMAT = redundant", true, "CREATE TABLE `t` (`c` INT) ROW_FORMAT = REDUNDANT"},
		{"create table t (c int) ROW_FORMAT = dynamic", true, "CREATE TABLE `t` (`c` INT) ROW_FORMAT = DYNAMIC"},
		{"create table t (c int) STATS_PERSISTENT = default", true, "CREATE TABLE `t` (`c` INT) STATS_PERSISTENT = DEFAULT /* TableOptionStatsPersistent is not supported */ "},
		{"create table t (c int) STATS_PERSISTENT = 0", true, "CREATE TABLE `t` (`c` INT) STATS_PERSISTENT = DEFAULT /* TableOptionStatsPersistent is not supported */ "},
		{"create table t (c int) STATS_PERSISTENT = 1", true, "CREATE TABLE `t` (`c` INT) STATS_PERSISTENT = DEFAULT /* TableOptionStatsPersistent is not supported */ "},
		{"create table t (c int) STATS_SAMPLE_PAGES 0", true, "CREATE TABLE `t` (`c` INT) STATS_SAMPLE_PAGES = 0"},
		{"create table t (c int) STATS_SAMPLE_PAGES 10", true, "CREATE TABLE `t` (`c` INT) STATS_SAMPLE_PAGES = 10"},
		{"create table t (c int) STATS_SAMPLE_PAGES = 10", true, "CREATE TABLE `t` (`c` INT) STATS_SAMPLE_PAGES = 10"},
		{"create table t (c int) STATS_SAMPLE_PAGES = default", true, "CREATE TABLE `t` (`c` INT) STATS_SAMPLE_PAGES = DEFAULT"},
		{"create table t (c int) PACK_KEYS = 1", true, "CREATE TABLE `t` (`c` INT) PACK_KEYS = DEFAULT /* TableOptionPackKeys is not supported */ "},
		{"create table t (c int) PACK_KEYS = 0", true, "CREATE TABLE `t` (`c` INT) PACK_KEYS = DEFAULT /* TableOptionPackKeys is not supported */ "},
		{"create table t (c int) PACK_KEYS = DEFAULT", true, "CREATE TABLE `t` (`c` INT) PACK_KEYS = DEFAULT /* TableOptionPackKeys is not supported */ "},
		{"create table t (c int) STORAGE DISK", true, "CREATE TABLE `t` (`c` INT) STORAGE DISK"},
		{"create table t (c int) STORAGE MEMORY", true, "CREATE TABLE `t` (`c` INT) STORAGE MEMORY"},
		{"create table t (c int) SECONDARY_ENGINE null", true, "CREATE TABLE `t` (`c` INT) SECONDARY_ENGINE = NULL"},
		{"create table t (c int) SECONDARY_ENGINE = innodb", true, "CREATE TABLE `t` (`c` INT) SECONDARY_ENGINE = 'innodb'"},
		{"create table t (c int) SECONDARY_ENGINE 'null'", true, "CREATE TABLE `t` (`c` INT) SECONDARY_ENGINE = 'null'"},
		{`create table testTableCompression (c VARCHAR(15000)) compression="ZLIB";`, true, "CREATE TABLE `testTableCompression` (`c` VARCHAR(15000)) COMPRESSION = 'ZLIB'"},
		{`create table t1 (c1 int) compression="zlib";`, true, "CREATE TABLE `t1` (`c1` INT) COMPRESSION = 'zlib'"},
		{`create table t1 (c1 int) collate=binary;`, true, "CREATE TABLE `t1` (`c1` INT) DEFAULT COLLATE = BINARY"},
		{`create table t1 (c1 int) collate=utf8mb4_0900_as_cs;`, true, "CREATE TABLE `t1` (`c1` INT) DEFAULT COLLATE = UTF8MB4_0900_AS_CS"},
		{`create table t1 (c1 int) default charset=binary collate=binary;`, true, "CREATE TABLE `t1` (`c1` INT) DEFAULT CHARACTER SET = BINARY DEFAULT COLLATE = BINARY"},

		// for table option `UNION`
		{"ALTER TABLE t_n UNION ( ), KEY_BLOCK_SIZE = 1", true, "ALTER TABLE `t_n` UNION = (), KEY_BLOCK_SIZE = 1"},
		{"ALTER TABLE d_n.t_n UNION ( t_n ) REMOVE PARTITIONING", true, "ALTER TABLE `d_n`.`t_n` UNION = (`t_n`) REMOVE PARTITIONING"},
		{"ALTER TABLE d_n.t_n LOCK DEFAULT , UNION = ( t_n , d_n.t_n ) REMOVE PARTITIONING", true, "ALTER TABLE `d_n`.`t_n` LOCK = DEFAULT, UNION = (`t_n`,`d_n`.`t_n`) REMOVE PARTITIONING"},
		{"ALTER TABLE d_n.t_n ALGORITHM = DEFAULT , MAX_ROWS 10, UNION ( d_n.t_n ) , ROW_FORMAT REDUNDANT, STATS_PERSISTENT = DEFAULT", true, "ALTER TABLE `d_n`.`t_n` ALGORITHM = DEFAULT, MAX_ROWS = 10, UNION = (`d_n`.`t_n`), ROW_FORMAT = REDUNDANT, STATS_PERSISTENT = DEFAULT /* TableOptionStatsPersistent is not supported */ "},

		// partition option
		{"create table t (b int) partition by range columns (b) (partition p0 values less than (not 3), partition p2 values less than (20));", false, ""},
		{"create table t (b int) partition by range columns (b) (partition p0 values less than (1 or 3), partition p2 values less than (20));", false, ""},
		{"create table t (b int) partition by range columns (b) (partition p0 values less than (3 is null), partition p2 values less than (20));", false, ""},
		{"create table t (b int) partition by range (b is null) (partition p0 values less than (10));", false, ""},
		{"create table t (b int) partition by list (not b) (partition p0 values in (10, 20));", false, ""},
		{"create table t (b int) partition by hash ( not b );", false, ""},
		{"create table t (b int) partition by range columns (b) (partition p0 values less than (3 in (3, 4, 5)), partition p2 values less than (20));", false, ""},
		{"CREATE TABLE t (id int) ENGINE = INNDB PARTITION BY RANGE (id) (PARTITION p0 VALUES LESS THAN (10), PARTITION p1 VALUES LESS THAN (20));", true, "CREATE TABLE `t` (`id` INT) ENGINE = INNDB PARTITION BY RANGE (`id`) (PARTITION `p0` VALUES LESS THAN (10),PARTITION `p1` VALUES LESS THAN (20))"},
		{"create table t (c int) PARTITION BY HASH (c) PARTITIONS 32;", true, "CREATE TABLE `t` (`c` INT) PARTITION BY HASH (`c`) PARTITIONS 32"},
		{"create table t (c int) PARTITION BY HASH (Year(VDate)) (PARTITION p1980 VALUES LESS THAN (1980) ENGINE = MyISAM, PARTITION p1990 VALUES LESS THAN (1990) ENGINE = MyISAM, PARTITION pothers VALUES LESS THAN MAXVALUE ENGINE = MyISAM)", false, ""},
		{"create table t (c int) PARTITION BY RANGE (Year(VDate)) (PARTITION p1980 VALUES LESS THAN (1980) ENGINE = MyISAM, PARTITION p1990 VALUES LESS THAN (1990) ENGINE = MyISAM, PARTITION pothers VALUES LESS THAN MAXVALUE ENGINE = MyISAM)", true, "CREATE TABLE `t` (`c` INT) PARTITION BY RANGE (YEAR(`VDate`)) (PARTITION `p1980` VALUES LESS THAN (1980) ENGINE = MyISAM,PARTITION `p1990` VALUES LESS THAN (1990) ENGINE = MyISAM,PARTITION `pothers` VALUES LESS THAN (MAXVALUE) ENGINE = MyISAM)"},
		{"create table t (c int, `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '') PARTITION BY RANGE (UNIX_TIMESTAMP(create_time)) (PARTITION p201610 VALUES LESS THAN(1477929600), PARTITION p201611 VALUES LESS THAN(1480521600),PARTITION p201612 VALUES LESS THAN(1483200000),PARTITION p201701 VALUES LESS THAN(1485878400),PARTITION p201702 VALUES LESS THAN(1488297600),PARTITION p201703 VALUES LESS THAN(1490976000))", true, "CREATE TABLE `t` (`c` INT,`create_time` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP() COMMENT '') PARTITION BY RANGE (UNIX_TIMESTAMP(`create_time`)) (PARTITION `p201610` VALUES LESS THAN (1477929600),PARTITION `p201611` VALUES LESS THAN (1480521600),PARTITION `p201612` VALUES LESS THAN (1483200000),PARTITION `p201701` VALUES LESS THAN (1485878400),PARTITION `p201702` VALUES LESS THAN (1488297600),PARTITION `p201703` VALUES LESS THAN (1490976000))"},
		{"CREATE TABLE `md_product_shop` (`shopCode` varchar(4) DEFAULT NULL COMMENT '地点') ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 /*!50100 PARTITION BY KEY (shopCode) PARTITIONS 19 */;", true, "CREATE TABLE `md_product_shop` (`shopCode` VARCHAR(4) DEFAULT NULL COMMENT '地点') ENGINE = InnoDB DEFAULT CHARACTER SET = UTF8MB4 PARTITION BY KEY (`shopCode`) PARTITIONS 19"},
		{"CREATE TABLE `payinfo1` (`id` bigint(20) NOT NULL AUTO_INCREMENT, `oderTime` datetime NOT NULL) ENGINE=InnoDB AUTO_INCREMENT=641533032 DEFAULT CHARSET=utf8 ROW_FORMAT=COMPRESSED KEY_BLOCK_SIZE=8 /*!50500 PARTITION BY RANGE COLUMNS(oderTime) (PARTITION P2011 VALUES LESS THAN ('2012-01-01 00:00:00') ENGINE = InnoDB, PARTITION P1201 VALUES LESS THAN ('2012-02-01 00:00:00') ENGINE = InnoDB, PARTITION PMAX VALUES LESS THAN (MAXVALUE) ENGINE = InnoDB)*/;", true, "CREATE TABLE `payinfo1` (`id` BIGINT(20) NOT NULL AUTO_INCREMENT,`oderTime` DATETIME NOT NULL) ENGINE = InnoDB AUTO_INCREMENT = 641533032 DEFAULT CHARACTER SET = UTF8 ROW_FORMAT = COMPRESSED KEY_BLOCK_SIZE = 8 PARTITION BY RANGE COLUMNS (`oderTime`) (PARTITION `P2011` VALUES LESS THAN (_UTF8MB4'2012-01-01 00:00:00') ENGINE = InnoDB,PARTITION `P1201` VALUES LESS THAN (_UTF8MB4'2012-02-01 00:00:00') ENGINE = InnoDB,PARTITION `PMAX` VALUES LESS THAN (MAXVALUE) ENGINE = InnoDB)"},
		{`CREATE TABLE app_channel_daily_report (id bigint(20) NOT NULL AUTO_INCREMENT, app_version varchar(32) COLLATE utf8_unicode_ci NOT NULL DEFAULT 'default', gmt_create datetime NOT NULL COMMENT '创建时间', PRIMARY KEY (id)) ENGINE=InnoDB AUTO_INCREMENT=33703438 DEFAULT CHARSET=utf8 COLLATE=utf8_unicode_ci
/*!50100 PARTITION BY RANGE (month(gmt_create)-1)
(PARTITION part0 VALUES LESS THAN (1) COMMENT = '1月份' ENGINE = InnoDB,
 PARTITION part1 VALUES LESS THAN (2) COMMENT = '2月份' ENGINE = InnoDB,
 PARTITION part2 VALUES LESS THAN (3) COMMENT = '3月份' ENGINE = InnoDB,
 PARTITION part3 VALUES LESS THAN (4) COMMENT = '4月份' ENGINE = InnoDB,
 PARTITION part4 VALUES LESS THAN (5) COMMENT = '5月份' ENGINE = InnoDB,
 PARTITION part5 VALUES LESS THAN (6) COMMENT = '6月份' ENGINE = InnoDB,
 PARTITION part6 VALUES LESS THAN (7) COMMENT = '7月份' ENGINE = InnoDB,
 PARTITION part7 VALUES LESS THAN (8) COMMENT = '8月份' ENGINE = InnoDB,
 PARTITION part8 VALUES LESS THAN (9) COMMENT = '9月份' ENGINE = InnoDB,
 PARTITION part9 VALUES LESS THAN (10) COMMENT = '10月份' ENGINE = InnoDB,
 PARTITION part10 VALUES LESS THAN (11) COMMENT = '11月份' ENGINE = InnoDB,
 PARTITION part11 VALUES LESS THAN (12) COMMENT = '12月份' ENGINE = InnoDB) */ ;`, true, "CREATE TABLE `app_channel_daily_report` (`id` BIGINT(20) NOT NULL AUTO_INCREMENT,`app_version` VARCHAR(32) COLLATE utf8_unicode_ci NOT NULL DEFAULT _UTF8MB4'default',`gmt_create` DATETIME NOT NULL COMMENT '创建时间',PRIMARY KEY(`id`)) ENGINE = InnoDB AUTO_INCREMENT = 33703438 DEFAULT CHARACTER SET = UTF8 DEFAULT COLLATE = UTF8_UNICODE_CI PARTITION BY RANGE (MONTH(`gmt_create`)-1) (PARTITION `part0` VALUES LESS THAN (1) COMMENT = '1月份' ENGINE = InnoDB,PARTITION `part1` VALUES LESS THAN (2) COMMENT = '2月份' ENGINE = InnoDB,PARTITION `part2` VALUES LESS THAN (3) COMMENT = '3月份' ENGINE = InnoDB,PARTITION `part3` VALUES LESS THAN (4) COMMENT = '4月份' ENGINE = InnoDB,PARTITION `part4` VALUES LESS THAN (5) COMMENT = '5月份' ENGINE = InnoDB,PARTITION `part5` VALUES LESS THAN (6) COMMENT = '6月份' ENGINE = InnoDB,PARTITION `part6` VALUES LESS THAN (7) COMMENT = '7月份' ENGINE = InnoDB,PARTITION `part7` VALUES LESS THAN (8) COMMENT = '8月份' ENGINE = InnoDB,PARTITION `part8` VALUES LESS THAN (9) COMMENT = '9月份' ENGINE = InnoDB,PARTITION `part9` VALUES LESS THAN (10) COMMENT = '10月份' ENGINE = InnoDB,PARTITION `part10` VALUES LESS THAN (11) COMMENT = '11月份' ENGINE = InnoDB,PARTITION `part11` VALUES LESS THAN (12) COMMENT = '12月份' ENGINE = InnoDB)"},

		// for placement option
		// 1. create table
		{`create table t (c int) primary_region="us";`, false, ""},
		{`create table t (c int) regions="us,3";`, false, ""},
		{`create table t (c int) followers="us,3";`, false, ""},
		{`create table t (c int) followers=3;`, false, ""},
		{`create table t (c int) followers=0;`, false, ""},
		{`create table t (c int) voters=3;`, false, ""},
		{`create table t (c int) learners="us,3";`, false, ""},
		{`create table t (c int) learners=3;`, false, ""},
		{`create table t (c int) schedule="even";`, false, ""},
		{`create table t (c int) constraints="ww";`, false, ""},
		{`create table t (c int) leader_constraints="ww";`, false, ""},
		{`create table t (c int) follower_constraints="ww";`, false, ""},
		{`create table t (c int) voter_constraints="ww";`, false, ""},
		{`create table t (c int) learner_constraints="ww";`, false, ""},
		{`create table t (c int) survival_preference="ww";`, false, ""},
		{`create table t (c int) /*T![placement] primary_region="us" */;`, false, ""},
		{`create table t (c int) /*T![placement] regions="us,3" */;`, false, ""},
		{`create table t (c int) /*T![placement] followers="us,3 */";`, false, ""},
		{`create table t (c int) /*T![placement] followers=3 */;`, false, ""},
		{`create table t (c int) /*T![placement] followers=0 */;`, false, ""},
		{`create table t (c int) /*T![placement] voters="us,3" */;`, false, ""},
		{`create table t (c int) /*T![placement] primary_region="us" regions="us,3"  */;`, false, ""},
		{`create table t (c int) placement policy="ww";`, true, "CREATE TABLE `t` (`c` INT) PLACEMENT POLICY = `ww`"},
		{"create table t (c int) /*T![placement] placement policy=`x` */;", true, "CREATE TABLE `t` (`c` INT) PLACEMENT POLICY = `x`"},
		{`create table t (c int) /*T![placement] placement policy="y" */;`, true, "CREATE TABLE `t` (`c` INT) PLACEMENT POLICY = `y`"},
		// 2. alter table
		{`alter table t primary_region="us";`, false, ""},
		{`alter table t regions="us,3";`, false, ""},
		{`alter table t followers=3;`, false, ""},
		{`alter table t followers=0;`, false, ""},
		{`alter table t voters=3;`, false, ""},
		{`alter table t learners=3;`, false, ""},
		{`alter table t schedule="even";`, false, ""},
		{`alter table t constraints="ww";`, false, ""},
		{`alter table t leader_constraints="ww";`, false, ""},
		{`alter table t follower_constraints="ww";`, false, ""},
		{`alter table t voter_constraints="ww";`, false, ""},
		{`alter table t learner_constraints="ww";`, false, ""},
		{`alter table t /*T![placement] primary_region="us" */;`, false, ""},
		{`alter table t placement policy="ww";`, true, "ALTER TABLE `t` PLACEMENT POLICY = `ww`"},
		{`alter table t /*T![placement] placement policy="ww" */;`, true, "ALTER TABLE `t` PLACEMENT POLICY = `ww`"},
		{`alter table t compact;`, true, "ALTER TABLE `t` COMPACT"},
		{`alter table t compact tiflash replica;`, true, "ALTER TABLE `t` COMPACT TIFLASH REPLICA"},
		{`alter table t compact partition p1,p2;`, true, "ALTER TABLE `t` COMPACT PARTITION `p1`,`p2`"},
		{`alter table t compact partition p1,p2 tiflash replica;`, true, "ALTER TABLE `t` COMPACT PARTITION `p1`,`p2` TIFLASH REPLICA"},
		// 3. create db
		{`create database t primary_region="us";`, false, ""},
		{`create database t regions="us,3";`, false, ""},
		{`create database t followers=3;`, false, ""},
		{`create database t followers=0;`, false, ""},
		{`create database t voters=3;`, false, ""},
		{`create database t learners=3;`, false, ""},
		{`create database t schedule="even";`, false, ""},
		{`create database t constraints="ww";`, false, ""},
		{`create database t leader_constraints="ww";`, false, ""},
		{`create database t follower_constraints="ww";`, false, ""},
		{`create database t voter_constraints="ww";`, false, ""},
		{`create database t learner_constraints="ww";`, false, ""},
		{`create database t /*T![placement] primary_region="us" */;`, false, ""},
		{`create database t placement policy="ww";`, true, "CREATE DATABASE `t` PLACEMENT POLICY = `ww`"},
		{`create database t default placement policy="ww";`, true, "CREATE DATABASE `t` PLACEMENT POLICY = `ww`"},
		{`create database t /*T![placement] placement policy="ww" */;`, true, "CREATE DATABASE `t` PLACEMENT POLICY = `ww`"},
		// 4. alter db
		{`alter database t primary_region="us";`, false, ""},
		{`alter database t regions="us,3";`, false, ""},
		{`alter database t followers=3;`, false, ""},
		{`alter database t followers=0;`, false, ""},
		{`alter database t voters=3;`, false, ""},
		{`alter database t learners=3;`, false, ""},
		{`alter database t schedule="even";`, false, ""},
		{`alter database t constraints="ww";`, false, ""},
		{`alter database t leader_constraints="ww";`, false, ""},
		{`alter database t follower_constraints="ww";`, false, ""},
		{`alter database t voter_constraints="ww";`, false, ""},
		{`alter database t learner_constraints="ww";`, false, ""},
		{`alter database t /*T![placement] primary_region="us" */;`, false, ""},
		{`alter database t placement policy="ww";`, true, "ALTER DATABASE `t` PLACEMENT POLICY = `ww`"},
		{`alter database t default placement policy="ww";`, true, "ALTER DATABASE `t` PLACEMENT POLICY = `ww`"},
		{`alter database t PLACEMENT POLICY='DEFAULT';`, true, "ALTER DATABASE `t` PLACEMENT POLICY = `DEFAULT`"},
		{`alter database t PLACEMENT POLICY=DEFAULT;`, true, "ALTER DATABASE `t` PLACEMENT POLICY = `DEFAULT`"},
		{`alter database t PLACEMENT POLICY = DEFAULT;`, true, "ALTER DATABASE `t` PLACEMENT POLICY = `DEFAULT`"},
		{`alter database t PLACEMENT POLICY SET DEFAULT`, true, "ALTER DATABASE `t` PLACEMENT POLICY = `DEFAULT`"},
		{"alter database t PLACEMENT POLICY=`DEFAULT`;", true, "ALTER DATABASE `t` PLACEMENT POLICY = `DEFAULT`"},
		{`alter database t /*T![placement] PLACEMENT POLICY='DEFAULT' */;`, true, "ALTER DATABASE `t` PLACEMENT POLICY = `DEFAULT`"},
		// 5. create partition
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) primary_region="us");`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) regions="us,3");`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) followers=3);`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) voters=3);`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) learners=3);`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) schedule="even");`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) constraints="ww");`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) leader_constraints="ww");`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) follower_constraints="ww");`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) voter_constraints="ww");`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) learner_constraints="ww");`, false, ""},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) placement policy="ww");`, true, "CREATE TABLE `m` (`c` INT) PARTITION BY RANGE (`c`) (PARTITION `p1` VALUES LESS THAN (200) PLACEMENT POLICY = `ww`)"},
		{`create table m (c int) partition by range (c) (partition p1 values less than (200) /*T![placement] placement policy="ww" */);`, true, "CREATE TABLE `m` (`c` INT) PARTITION BY RANGE (`c`) (PARTITION `p1` VALUES LESS THAN (200) PLACEMENT POLICY = `ww`)"},
		// 6. alter partition
		{`alter table m partition t primary_region="us";`, false, ""},
		{`alter table m partition t regions="us,3";`, false, ""},
		{`alter table m partition t followers=3;`, false, ""},
		{`alter table m partition t primary_region="us" followers=3;`, false, ""},
		{`alter table m partition t voters=3;`, false, ""},
		{`alter table m partition t learners=3;`, false, ""},
		{`alter table m partition t schedule="even";`, false, ""},
		{`alter table m partition t constraints="ww";`, false, ""},
		{`alter table m partition t leader_constraints="ww";`, false, ""},
		{`alter table m partition t follower_constraints="ww";`, false, ""},
		{`alter table m partition t voter_constraints="ww";`, false, ""},
		{`alter table m partition t learner_constraints="ww";`, false, ""},
		{`alter table m partition t placement policy="ww";`, true, "ALTER TABLE `m` PARTITION `t` PLACEMENT POLICY = `ww`"},
		{`alter table m partition t /*T![placement] placement policy="ww" */;`, true, "ALTER TABLE `m` PARTITION `t` PLACEMENT POLICY = `ww`"},
		// 7. add partition
		{`alter table m add partition (partition p1 values less than (200) primary_region="us");`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) regions="us,3");`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) followers=3);`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) voters=3);`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) learners=3);`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) schedule="even");`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) constraints="ww");`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) leader_constraints="ww");`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) follower_constraints="ww");`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) voter_constraints="ww");`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) learner_constraints="ww");`, false, ""},
		{`alter table m add partition (partition p1 values less than (200) placement policy="ww");`, true, "ALTER TABLE `m` ADD PARTITION (PARTITION `p1` VALUES LESS THAN (200) PLACEMENT POLICY = `ww`)"},
		{`alter table m add partition (partition p1 values less than (200) /*T![placement] placement policy="ww" */);`, true, "ALTER TABLE `m` ADD PARTITION (PARTITION `p1` VALUES LESS THAN (200) PLACEMENT POLICY = `ww`)"},
		{`alter table m add column a int, add partition (partition p1 values less than (200))`, true, "ALTER TABLE `m` ADD COLUMN `a` INT, ADD PARTITION (PARTITION `p1` VALUES LESS THAN (200))"},
		// TODO: Do not allow this order!
		{`alter table m add partition (partition p1 values less than (200)), add column a int`, true, "ALTER TABLE `m` ADD PARTITION (PARTITION `p1` VALUES LESS THAN (200)), ADD COLUMN `a` INT"},
		// for check clause
		{"create table t (c1 bool, c2 bool, check (c1 in (0, 1)) not enforced, check (c2 in (0, 1)))", true, "CREATE TABLE `t` (`c1` TINYINT(1),`c2` TINYINT(1),CHECK(`c1` IN (0,1)) NOT ENFORCED,CHECK(`c2` IN (0,1)) ENFORCED)"},
		{"CREATE TABLE Customer (SD integer CHECK (SD > 0), First_Name varchar(30));", true, "CREATE TABLE `Customer` (`SD` INT CHECK(`SD`>0) ENFORCED,`First_Name` VARCHAR(30))"},
		{"CREATE TABLE Customer (SD integer CHECK (SD > 0) not enforced, SS varchar(30) check(ss='test') enforced);", true, "CREATE TABLE `Customer` (`SD` INT CHECK(`SD`>0) NOT ENFORCED,`SS` VARCHAR(30) CHECK(`ss`=_UTF8MB4'test') ENFORCED)"},
		{"CREATE TABLE Customer (SD integer CHECK (SD > 0) not null, First_Name varchar(30) comment 'string' not null);", true, "CREATE TABLE `Customer` (`SD` INT CHECK(`SD`>0) ENFORCED NOT NULL,`First_Name` VARCHAR(30) COMMENT 'string' NOT NULL)"},
		{"CREATE TABLE Customer (SD integer comment 'string' CHECK (SD > 0) not null);", true, "CREATE TABLE `Customer` (`SD` INT COMMENT 'string' CHECK(`SD`>0) ENFORCED NOT NULL)"},
		{"CREATE TABLE Customer (SD integer comment 'string' not enforced, First_Name varchar(30));", false, ""},
		{"CREATE TABLE Customer (SD integer not enforced, First_Name varchar(30));", false, ""},

		{"create database xxx", true, "CREATE DATABASE `xxx`"},
		{"create database if exists xxx", false, ""},
		{"create database if not exists xxx", true, "CREATE DATABASE IF NOT EXISTS `xxx`"},

		// for create database with encryption
		{"create database xxx encryption = 'N'", true, "CREATE DATABASE `xxx` ENCRYPTION = 'N'"},
		{"create database xxx encryption 'N'", true, "CREATE DATABASE `xxx` ENCRYPTION = 'N'"},
		{"create database xxx default encryption = 'N'", true, "CREATE DATABASE `xxx` ENCRYPTION = 'N'"},
		{"create database xxx default encryption 'N'", true, "CREATE DATABASE `xxx` ENCRYPTION = 'N'"},
		{"create database xxx encryption = 'Y'", true, "CREATE DATABASE `xxx` ENCRYPTION = 'Y'"},
		{"create database xxx encryption 'Y'", true, "CREATE DATABASE `xxx` ENCRYPTION = 'Y'"},
		{"create database xxx default encryption = 'Y'", true, "CREATE DATABASE `xxx` ENCRYPTION = 'Y'"},
		{"create database xxx default encryption 'Y'", true, "CREATE DATABASE `xxx` ENCRYPTION = 'Y'"},
		{"create database xxx encryption = N", false, ""},

		{"create schema xxx", true, "CREATE DATABASE `xxx`"},
		{"create schema if exists xxx", false, ""},
		{"create schema if not exists xxx", true, "CREATE DATABASE IF NOT EXISTS `xxx`"},
		// for drop database/schema/table/view/stats
		{"drop database xxx", true, "DROP DATABASE `xxx`"},
		{"drop database if exists xxx", true, "DROP DATABASE IF EXISTS `xxx`"},
		{"drop database if not exists xxx", false, ""},
		{"drop schema xxx", true, "DROP DATABASE `xxx`"},
		{"drop schema if exists xxx", true, "DROP DATABASE IF EXISTS `xxx`"},
		{"drop schema if not exists xxx", false, ""},
		{"drop table", false, ""},
		{"drop table if exists t'xyz", false, ""},
		{"drop table if exists t'", false, ""},
		{"drop table if exists t`", false, ""},
		{`drop table if exists t'`, false, ""},
		{`drop table if exists t"`, false, ""},
		{"drop table xxx", true, "DROP TABLE `xxx`"},
		{"drop table xxx, yyy", true, "DROP TABLE `xxx`, `yyy`"},
		{"drop tables xxx", true, "DROP TABLE `xxx`"},
		{"drop tables xxx, yyy", true, "DROP TABLE `xxx`, `yyy`"},
		{"drop table if exists xxx", true, "DROP TABLE IF EXISTS `xxx`"},
		{"drop table if exists xxx, yyy", true, "DROP TABLE IF EXISTS `xxx`, `yyy`"},
		{"drop table if not exists xxx", false, ""},
		{"drop table xxx restrict", true, "DROP TABLE `xxx`"},
		{"drop table xxx, yyy cascade", true, "DROP TABLE `xxx`, `yyy`"},
		{"drop table if exists xxx restrict", true, "DROP TABLE IF EXISTS `xxx`"},
		{"drop view", false, "DROP VIEW"},
		{"drop view xxx", true, "DROP VIEW `xxx`"},
		{"drop view xxx, yyy", true, "DROP VIEW `xxx`, `yyy`"},
		{"drop view if exists xxx", true, "DROP VIEW IF EXISTS `xxx`"},
		{"drop view if exists xxx, yyy", true, "DROP VIEW IF EXISTS `xxx`, `yyy`"},
		{"drop stats t", true, "DROP STATS `t`"},
		{"drop stats t1, t2, t3", true, "DROP STATS `t1`, `t2`, `t3`"},
		{"drop stats t global", true, "DROP STATS `t` GLOBAL"},
		{"drop stats t partition p0", true, "DROP STATS `t` PARTITION `p0`"},
		{"drop stats t partition p0, p1, p2", true, "DROP STATS `t` PARTITION `p0`,`p1`,`p2`"},
		// for issue 974
		{`CREATE TABLE address (
		id bigint(20) NOT NULL AUTO_INCREMENT,
		create_at datetime NOT NULL,
		deleted tinyint(1) NOT NULL,
		update_at datetime NOT NULL,
		version bigint(20) DEFAULT NULL,
		address varchar(128) NOT NULL,
		address_detail varchar(128) NOT NULL,
		cellphone varchar(16) NOT NULL,
		latitude double NOT NULL,
		longitude double NOT NULL,
		name varchar(16) NOT NULL,
		sex tinyint(1) NOT NULL,
		user_id bigint(20) NOT NULL,
		PRIMARY KEY (id),
		CONSTRAINT FK_7rod8a71yep5vxasb0ms3osbg FOREIGN KEY (user_id) REFERENCES waimaiqa.user (id),
		INDEX FK_7rod8a71yep5vxasb0ms3osbg (user_id) comment ''
		) ENGINE=InnoDB AUTO_INCREMENT=30 DEFAULT CHARACTER SET UTF8 COLLATE UTF8_GENERAL_CI ROW_FORMAT=COMPACT COMMENT='' CHECKSUM=0 DELAY_KEY_WRITE=0;`, true, "CREATE TABLE `address` (`id` BIGINT(20) NOT NULL AUTO_INCREMENT,`create_at` DATETIME NOT NULL,`deleted` TINYINT(1) NOT NULL,`update_at` DATETIME NOT NULL,`version` BIGINT(20) DEFAULT NULL,`address` VARCHAR(128) NOT NULL,`address_detail` VARCHAR(128) NOT NULL,`cellphone` VARCHAR(16) NOT NULL,`latitude` DOUBLE NOT NULL,`longitude` DOUBLE NOT NULL,`name` VARCHAR(16) NOT NULL,`sex` TINYINT(1) NOT NULL,`user_id` BIGINT(20) NOT NULL,PRIMARY KEY(`id`),CONSTRAINT `FK_7rod8a71yep5vxasb0ms3osbg` FOREIGN KEY (`user_id`) REFERENCES `waimaiqa`.`user`(`id`),INDEX `FK_7rod8a71yep5vxasb0ms3osbg`(`user_id`)) ENGINE = InnoDB AUTO_INCREMENT = 30 DEFAULT CHARACTER SET = UTF8 DEFAULT COLLATE = UTF8_GENERAL_CI ROW_FORMAT = COMPACT COMMENT = '' CHECKSUM = 0 DELAY_KEY_WRITE = 0"},
		// for issue 975
		{`CREATE TABLE test_data (
		id bigint(20) NOT NULL AUTO_INCREMENT,
		create_at datetime NOT NULL,
		deleted tinyint(1) NOT NULL,
		update_at datetime NOT NULL,
		version bigint(20) DEFAULT NULL,
		address varchar(255) NOT NULL,
		amount decimal(19,2) DEFAULT NULL,
		charge_id varchar(32) DEFAULT NULL,
		paid_amount decimal(19,2) DEFAULT NULL,
		transaction_no varchar(64) DEFAULT NULL,
		wx_mp_app_id varchar(32) DEFAULT NULL,
		contacts varchar(50) DEFAULT NULL,
		deliver_fee decimal(19,2) DEFAULT NULL,
		deliver_info varchar(255) DEFAULT NULL,
		deliver_time varchar(255) DEFAULT NULL,
		description varchar(255) DEFAULT NULL,
		invoice varchar(255) DEFAULT NULL,
		order_from int(11) DEFAULT NULL,
		order_state int(11) NOT NULL,
		packing_fee decimal(19,2) DEFAULT NULL,
		payment_time datetime DEFAULT NULL,
		payment_type int(11) DEFAULT NULL,
		phone varchar(50) NOT NULL,
		store_employee_id bigint(20) DEFAULT NULL,
		store_id bigint(20) NOT NULL,
		user_id bigint(20) NOT NULL,
		payment_mode int(11) NOT NULL,
		current_latitude double NOT NULL,
		current_longitude double NOT NULL,
		address_latitude double NOT NULL,
		address_longitude double NOT NULL,
		PRIMARY KEY (id),
		CONSTRAINT food_order_ibfk_1 FOREIGN KEY (user_id) REFERENCES waimaiqa.user (id),
		CONSTRAINT food_order_ibfk_2 FOREIGN KEY (store_id) REFERENCES waimaiqa.store (id),
		CONSTRAINT food_order_ibfk_3 FOREIGN KEY (store_employee_id) REFERENCES waimaiqa.store_employee (id),
		UNIQUE FK_UNIQUE_charge_id USING BTREE (charge_id) comment '',
		INDEX FK_eqst2x1xisn3o0wbrlahnnqq8 USING BTREE (store_employee_id) comment '',
		INDEX FK_8jcmec4kb03f4dod0uqwm54o9 USING BTREE (store_id) comment '',
		INDEX FK_a3t0m9apja9jmrn60uab30pqd USING BTREE (user_id) comment ''
		) ENGINE=InnoDB AUTO_INCREMENT=95 DEFAULT CHARACTER SET utf8 COLLATE UTF8_GENERAL_CI ROW_FORMAT=COMPACT COMMENT='' CHECKSUM=0 DELAY_KEY_WRITE=0;`, true, "CREATE TABLE `test_data` (`id` BIGINT(20) NOT NULL AUTO_INCREMENT,`create_at` DATETIME NOT NULL,`deleted` TINYINT(1) NOT NULL,`update_at` DATETIME NOT NULL,`version` BIGINT(20) DEFAULT NULL,`address` VARCHAR(255) NOT NULL,`amount` DECIMAL(19,2) DEFAULT NULL,`charge_id` VARCHAR(32) DEFAULT NULL,`paid_amount` DECIMAL(19,2) DEFAULT NULL,`transaction_no` VARCHAR(64) DEFAULT NULL,`wx_mp_app_id` VARCHAR(32) DEFAULT NULL,`contacts` VARCHAR(50) DEFAULT NULL,`deliver_fee` DECIMAL(19,2) DEFAULT NULL,`deliver_info` VARCHAR(255) DEFAULT NULL,`deliver_time` VARCHAR(255) DEFAULT NULL,`description` VARCHAR(255) DEFAULT NULL,`invoice` VARCHAR(255) DEFAULT NULL,`order_from` INT(11) DEFAULT NULL,`order_state` INT(11) NOT NULL,`packing_fee` DECIMAL(19,2) DEFAULT NULL,`payment_time` DATETIME DEFAULT NULL,`payment_type` INT(11) DEFAULT NULL,`phone` VARCHAR(50) NOT NULL,`store_employee_id` BIGINT(20) DEFAULT NULL,`store_id` BIGINT(20) NOT NULL,`user_id` BIGINT(20) NOT NULL,`payment_mode` INT(11) NOT NULL,`current_latitude` DOUBLE NOT NULL,`current_longitude` DOUBLE NOT NULL,`address_latitude` DOUBLE NOT NULL,`address_longitude` DOUBLE NOT NULL,PRIMARY KEY(`id`),CONSTRAINT `food_order_ibfk_1` FOREIGN KEY (`user_id`) REFERENCES `waimaiqa`.`user`(`id`),CONSTRAINT `food_order_ibfk_2` FOREIGN KEY (`store_id`) REFERENCES `waimaiqa`.`store`(`id`),CONSTRAINT `food_order_ibfk_3` FOREIGN KEY (`store_employee_id`) REFERENCES `waimaiqa`.`store_employee`(`id`),UNIQUE `FK_UNIQUE_charge_id`(`charge_id`) USING BTREE,INDEX `FK_eqst2x1xisn3o0wbrlahnnqq8`(`store_employee_id`) USING BTREE,INDEX `FK_8jcmec4kb03f4dod0uqwm54o9`(`store_id`) USING BTREE,INDEX `FK_a3t0m9apja9jmrn60uab30pqd`(`user_id`) USING BTREE) ENGINE = InnoDB AUTO_INCREMENT = 95 DEFAULT CHARACTER SET = UTF8 DEFAULT COLLATE = UTF8_GENERAL_CI ROW_FORMAT = COMPACT COMMENT = '' CHECKSUM = 0 DELAY_KEY_WRITE = 0"},
		{`create table t (c int KEY);`, true, "CREATE TABLE `t` (`c` INT PRIMARY KEY)"},
		{`CREATE TABLE address (
		id bigint(20) NOT NULL AUTO_INCREMENT,
		create_at datetime NOT NULL,
		deleted tinyint(1) NOT NULL,
		update_at datetime NOT NULL,
		version bigint(20) DEFAULT NULL,
		address varchar(128) NOT NULL,
		address_detail varchar(128) NOT NULL,
		cellphone varchar(16) NOT NULL,
		latitude double NOT NULL,
		longitude double NOT NULL,
		name varchar(16) NOT NULL,
		sex tinyint(1) NOT NULL,
		user_id bigint(20) NOT NULL,
		PRIMARY KEY (id),
		CONSTRAINT FK_7rod8a71yep5vxasb0ms3osbg FOREIGN KEY (user_id) REFERENCES waimaiqa.user (id) ON DELETE CASCADE ON UPDATE NO ACTION,
		INDEX FK_7rod8a71yep5vxasb0ms3osbg (user_id) comment ''
		) ENGINE=InnoDB AUTO_INCREMENT=30 DEFAULT CHARACTER SET utf8 COLLATE UTF8_GENERAL_CI ROW_FORMAT=COMPACT COMMENT='' CHECKSUM=0 DELAY_KEY_WRITE=0;`, true, "CREATE TABLE `address` (`id` BIGINT(20) NOT NULL AUTO_INCREMENT,`create_at` DATETIME NOT NULL,`deleted` TINYINT(1) NOT NULL,`update_at` DATETIME NOT NULL,`version` BIGINT(20) DEFAULT NULL,`address` VARCHAR(128) NOT NULL,`address_detail` VARCHAR(128) NOT NULL,`cellphone` VARCHAR(16) NOT NULL,`latitude` DOUBLE NOT NULL,`longitude` DOUBLE NOT NULL,`name` VARCHAR(16) NOT NULL,`sex` TINYINT(1) NOT NULL,`user_id` BIGINT(20) NOT NULL,PRIMARY KEY(`id`),CONSTRAINT `FK_7rod8a71yep5vxasb0ms3osbg` FOREIGN KEY (`user_id`) REFERENCES `waimaiqa`.`user`(`id`) ON DELETE CASCADE ON UPDATE NO ACTION,INDEX `FK_7rod8a71yep5vxasb0ms3osbg`(`user_id`)) ENGINE = InnoDB AUTO_INCREMENT = 30 DEFAULT CHARACTER SET = UTF8 DEFAULT COLLATE = UTF8_GENERAL_CI ROW_FORMAT = COMPACT COMMENT = '' CHECKSUM = 0 DELAY_KEY_WRITE = 0"},
		{"CREATE TABLE address (\r\nid bigint(20) NOT NULL AUTO_INCREMENT,\r\ncreate_at datetime NOT NULL,\r\ndeleted tinyint(1) NOT NULL,\r\nupdate_at datetime NOT NULL,\r\nversion bigint(20) DEFAULT NULL,\r\naddress varchar(128) NOT NULL,\r\naddress_detail varchar(128) NOT NULL,\r\ncellphone varchar(16) NOT NULL,\r\nlatitude double NOT NULL,\r\nlongitude double NOT NULL,\r\nname varchar(16) NOT NULL,\r\nsex tinyint(1) NOT NULL,\r\nuser_id bigint(20) NOT NULL,\r\nPRIMARY KEY (id),\r\nCONSTRAINT FK_7rod8a71yep5vxasb0ms3osbg FOREIGN KEY (user_id) REFERENCES waimaiqa.user (id) ON DELETE CASCADE ON UPDATE NO ACTION,\r\nINDEX FK_7rod8a71yep5vxasb0ms3osbg (user_id) comment ''\r\n) ENGINE=InnoDB AUTO_INCREMENT=30 DEFAULT CHARACTER SET utf8 COLLATE utf8_general_ci ROW_FORMAT=COMPACT COMMENT='' CHECKSUM=0 DELAY_KEY_WRITE=0;", true, "CREATE TABLE `address` (`id` BIGINT(20) NOT NULL AUTO_INCREMENT,`create_at` DATETIME NOT NULL,`deleted` TINYINT(1) NOT NULL,`update_at` DATETIME NOT NULL,`version` BIGINT(20) DEFAULT NULL,`address` VARCHAR(128) NOT NULL,`address_detail` VARCHAR(128) NOT NULL,`cellphone` VARCHAR(16) NOT NULL,`latitude` DOUBLE NOT NULL,`longitude` DOUBLE NOT NULL,`name` VARCHAR(16) NOT NULL,`sex` TINYINT(1) NOT NULL,`user_id` BIGINT(20) NOT NULL,PRIMARY KEY(`id`),CONSTRAINT `FK_7rod8a71yep5vxasb0ms3osbg` FOREIGN KEY (`user_id`) REFERENCES `waimaiqa`.`user`(`id`) ON DELETE CASCADE ON UPDATE NO ACTION,INDEX `FK_7rod8a71yep5vxasb0ms3osbg`(`user_id`)) ENGINE = InnoDB AUTO_INCREMENT = 30 DEFAULT CHARACTER SET = UTF8 DEFAULT COLLATE = UTF8_GENERAL_CI ROW_FORMAT = COMPACT COMMENT = '' CHECKSUM = 0 DELAY_KEY_WRITE = 0"},
		// for issue 1802
		{`CREATE TABLE t1 (
		accout_id int(11) DEFAULT '0',
		summoner_id int(11) DEFAULT '0',
		union_name varbinary(52) NOT NULL,
		union_id int(11) DEFAULT '0',
		PRIMARY KEY (union_name)) ENGINE=MyISAM DEFAULT CHARSET=binary;`, true, "CREATE TABLE `t1` (`accout_id` INT(11) DEFAULT _UTF8MB4'0',`summoner_id` INT(11) DEFAULT _UTF8MB4'0',`union_name` VARBINARY(52) NOT NULL,`union_id` INT(11) DEFAULT _UTF8MB4'0',PRIMARY KEY(`union_name`)) ENGINE = MyISAM DEFAULT CHARACTER SET = BINARY"},
		// for issue pingcap/parser#310
		{`CREATE TABLE t (a DECIMAL(20,0), b DECIMAL(30), c FLOAT(25,0))`, true, "CREATE TABLE `t` (`a` DECIMAL(20,0),`b` DECIMAL(30),`c` FLOAT(25,0))"},
		// Create table with multiple index options.
		{`create table t (c int, index ci (c) USING BTREE COMMENT "123");`, true, "CREATE TABLE `t` (`c` INT,INDEX `ci`(`c`) USING BTREE COMMENT '123')"},
		// for default value
		{"CREATE TABLE sbtest (id INTEGER UNSIGNED NOT NULL AUTO_INCREMENT, k integer UNSIGNED DEFAULT '0' NOT NULL, c char(120) DEFAULT '' NOT NULL, pad char(60) DEFAULT '' NOT NULL, PRIMARY KEY  (id) )", true, "CREATE TABLE `sbtest` (`id` INT UNSIGNED NOT NULL AUTO_INCREMENT,`k` INT UNSIGNED DEFAULT _UTF8MB4'0' NOT NULL,`c` CHAR(120) DEFAULT _UTF8MB4'' NOT NULL,`pad` CHAR(60) DEFAULT _UTF8MB4'' NOT NULL,PRIMARY KEY(`id`))"},
		{"create table test (create_date TIMESTAMP NOT NULL COMMENT '创建日期 create date' DEFAULT now());", true, "CREATE TABLE `test` (`create_date` TIMESTAMP NOT NULL COMMENT '创建日期 create date' DEFAULT CURRENT_TIMESTAMP())"},
		{"create table ts (t int, v timestamp(3) default CURRENT_TIMESTAMP(3));", true, "CREATE TABLE `ts` (`t` INT,`v` TIMESTAMP(3) DEFAULT CURRENT_TIMESTAMP(3))"}, // TODO: The number yacc in parentheses has not been implemented yet.
		// Create table with primary key name.
		{"create table if not exists `t` (`id` int not null auto_increment comment '消息ID', primary key `pk_id` (`id`) );", true, "CREATE TABLE IF NOT EXISTS `t` (`id` INT NOT NULL AUTO_INCREMENT COMMENT '消息ID',PRIMARY KEY `pk_id`(`id`))"},
		// Create table with like.
		{"create table a like b", true, "CREATE TABLE `a` LIKE `b`"},
		{"create table a (id int REFERENCES a (id) ON delete NO ACTION )", true, "CREATE TABLE `a` (`id` INT REFERENCES `a`(`id`) ON DELETE NO ACTION)"},
		{"create table a (id int REFERENCES a (id) ON update set default )", true, "CREATE TABLE `a` (`id` INT REFERENCES `a`(`id`) ON UPDATE SET DEFAULT)"},
		{"create table a (id int REFERENCES a (id) ON delete set null on update CASCADE)", true, "CREATE TABLE `a` (`id` INT REFERENCES `a`(`id`) ON DELETE SET NULL ON UPDATE CASCADE)"},
		{"create table a (id int REFERENCES a (id) ON update set default on delete RESTRICT)", true, "CREATE TABLE `a` (`id` INT REFERENCES `a`(`id`) ON DELETE RESTRICT ON UPDATE SET DEFAULT)"},
		{"create table a (id int REFERENCES a (id) MATCH FULL ON delete NO ACTION )", true, "CREATE TABLE `a` (`id` INT REFERENCES `a`(`id`) MATCH FULL ON DELETE NO ACTION)"},
		{"create table a (id int REFERENCES a (id) MATCH PARTIAL ON update NO ACTION )", true, "CREATE TABLE `a` (`id` INT REFERENCES `a`(`id`) MATCH PARTIAL ON UPDATE NO ACTION)"},
		{"create table a (id int REFERENCES a (id) MATCH SIMPLE ON update NO ACTION )", true, "CREATE TABLE `a` (`id` INT REFERENCES `a`(`id`) MATCH SIMPLE ON UPDATE NO ACTION)"},
		{"create table a (id int REFERENCES a (id) ON update set default )", true, "CREATE TABLE `a` (`id` INT REFERENCES `a`(`id`) ON UPDATE SET DEFAULT)"},
		{"create table a (id int REFERENCES a (id) ON update set default on update CURRENT_TIMESTAMP)", false, ""},
		{"create table a (id int REFERENCES a (id) ON delete set default on update CURRENT_TIMESTAMP)", false, ""},
		{"create table a (like b)", true, "CREATE TABLE `a` LIKE `b`"},
		{"create table if not exists a like b", true, "CREATE TABLE IF NOT EXISTS `a` LIKE `b`"},
		{"create table if not exists a (like b)", true, "CREATE TABLE IF NOT EXISTS `a` LIKE `b`"},
		{"create table if not exists a like (b)", false, ""},
		{"create table a (t int) like b", false, ""},
		{"create table a (t int) like (b)", false, ""},
		// Create table with select statement
		{"create table a select * from b", true, "CREATE TABLE `a` AS SELECT * FROM `b`"},
		{"create table a as select * from b", true, "CREATE TABLE `a` AS SELECT * FROM `b`"},
		{"create table a (m int, n datetime) as select * from b", true, "CREATE TABLE `a` (`m` INT,`n` DATETIME) AS SELECT * FROM `b`"},
		{"create table a (unique(n)) as select n from b", true, "CREATE TABLE `a` (UNIQUE(`n`)) AS SELECT `n` FROM `b`"},
		{"create table a ignore as select n from b", true, "CREATE TABLE `a` IGNORE AS SELECT `n` FROM `b`"},
		{"create table a replace as select n from b", true, "CREATE TABLE `a` REPLACE AS SELECT `n` FROM `b`"},
		{"create table a (m int) replace as (select n as m from b union select n+1 as m from c group by 1 limit 2)", true, "CREATE TABLE `a` (`m` INT) REPLACE AS (SELECT `n` AS `m` FROM `b` UNION SELECT `n`+1 AS `m` FROM `c` GROUP BY 1 LIMIT 2)"},

		// Create table with no option is valid for parser
		{"create table a", true, "CREATE TABLE `a`"},

		{"create table t (a timestamp default now)", false, ""},
		{"create table t (a timestamp default now())", true, "CREATE TABLE `t` (`a` TIMESTAMP DEFAULT CURRENT_TIMESTAMP())"},
		{"create table t (a timestamp default (((now()))))", true, "CREATE TABLE `t` (`a` TIMESTAMP DEFAULT CURRENT_TIMESTAMP())"},
		{"create table t (a timestamp default now() on update now)", false, ""},
		{"create table t (a timestamp default now() on update now())", true, "CREATE TABLE `t` (`a` TIMESTAMP DEFAULT CURRENT_TIMESTAMP() ON UPDATE CURRENT_TIMESTAMP())"},
		{"create table t (a timestamp default now() on update (now()))", false, ""},
		{"CREATE TABLE t (c TEXT) default CHARACTER SET utf8, default COLLATE utf8_general_ci;", true, "CREATE TABLE `t` (`c` TEXT) DEFAULT CHARACTER SET = UTF8 DEFAULT COLLATE = UTF8_GENERAL_CI"},
		{"CREATE TABLE t (c TEXT) shard_row_id_bits = 1;", true, "CREATE TABLE `t` (`c` TEXT) SHARD_ROW_ID_BITS = 1"},
		{"CREATE TABLE t (c TEXT) shard_row_id_bits = 1, PRE_SPLIT_REGIONS = 1;", true, "CREATE TABLE `t` (`c` TEXT) SHARD_ROW_ID_BITS = 1 PRE_SPLIT_REGIONS = 1"},
		// Create table with ON UPDATE CURRENT_TIMESTAMP(6), specify fraction part.
		{"CREATE TABLE IF NOT EXISTS `general_log` (`event_time` timestamp(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),`user_host` mediumtext NOT NULL,`thread_id` bigint(20) unsigned NOT NULL,`server_id` int(10) unsigned NOT NULL,`command_type` varchar(64) NOT NULL,`argument` mediumblob NOT NULL) ENGINE=CSV DEFAULT CHARSET=utf8 COMMENT='General log'", true, "CREATE TABLE IF NOT EXISTS `general_log` (`event_time` TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6),`user_host` MEDIUMTEXT NOT NULL,`thread_id` BIGINT(20) UNSIGNED NOT NULL,`server_id` INT(10) UNSIGNED NOT NULL,`command_type` VARCHAR(64) NOT NULL,`argument` MEDIUMBLOB NOT NULL) ENGINE = CSV DEFAULT CHARACTER SET = UTF8 COMMENT = 'General log'"}, // TODO: The number yacc in parentheses has not been implemented yet.
		// For reference_definition in column_definition.
		{"CREATE TABLE followers ( f1 int NOT NULL REFERENCES user_profiles (uid) );", true, "CREATE TABLE `followers` (`f1` INT NOT NULL REFERENCES `user_profiles`(`uid`))"},

		// For column default expression
		{"create table t (a int default rand())", true, "CREATE TABLE `t` (`a` INT DEFAULT (RAND()))"},
		{"create table t (a int default rand(1))", true, "CREATE TABLE `t` (`a` INT DEFAULT (RAND(1)))"},
		{"create table t (a int default (rand()))", true, "CREATE TABLE `t` (`a` INT DEFAULT (RAND()))"},
		{"create table t (a int default (rand(1)))", true, "CREATE TABLE `t` (`a` INT DEFAULT (RAND(1)))"},
		{"create table t (a int default (((rand()))))", true, "CREATE TABLE `t` (`a` INT DEFAULT (RAND()))"},
		{"create table t (a int default (((rand(1)))))", true, "CREATE TABLE `t` (`a` INT DEFAULT (RAND(1)))"},
		{"create table t (d date default current_date())", true, "CREATE TABLE `t` (`d` DATE DEFAULT (CURRENT_DATE()))"},
		{"create table t (d date default current_date)", true, "CREATE TABLE `t` (`d` DATE DEFAULT (CURRENT_DATE()))"},
		{"create table t (d date default (current_date()))", true, "CREATE TABLE `t` (`d` DATE DEFAULT (CURRENT_DATE()))"},
		{"create table t (d date default (curdate()))", true, "CREATE TABLE `t` (`d` DATE DEFAULT (CURRENT_DATE()))"},
		{"create table t (d date default curdate())", true, "CREATE TABLE `t` (`d` DATE DEFAULT (CURRENT_DATE()))"},
		{"create table t (d date default current_date())", true, "CREATE TABLE `t` (`d` DATE DEFAULT (CURRENT_DATE()))"},
		{"create table t (d date default date_format(now(),'%Y-%m'))", true, "CREATE TABLE `t` (`d` DATE DEFAULT (DATE_FORMAT(NOW(), _UTF8MB4'%Y-%m')))"},
		{"create table t (d date default (date_format(now(),'%Y-%m')))", true, "CREATE TABLE `t` (`d` DATE DEFAULT (DATE_FORMAT(NOW(), _UTF8MB4'%Y-%m')))"},
		{"create table t (d date default date_format(now(),'%Y-%m-%d'))", true, "CREATE TABLE `t` (`d` DATE DEFAULT (DATE_FORMAT(NOW(), _UTF8MB4'%Y-%m-%d')))"},
		{"create table t (d date default date_format(now(),'%Y-%m-%d %H.%i.%s'))", true, "CREATE TABLE `t` (`d` DATE DEFAULT (DATE_FORMAT(NOW(), _UTF8MB4'%Y-%m-%d %H.%i.%s')))"},
		{"create table t (d date default date_format(now(),'%Y-%m-%d %H:%i:%s'))", true, "CREATE TABLE `t` (`d` DATE DEFAULT (DATE_FORMAT(NOW(), _UTF8MB4'%Y-%m-%d %H:%i:%s')))"},
		{"create table t (d date default date_format(now(),'%b %d %Y %h:%i %p'))", true, "CREATE TABLE `t` (`d` DATE DEFAULT (DATE_FORMAT(NOW(), _UTF8MB4'%b %d %Y %h:%i %p')))"},
		{"create table t (a varchar(32) default (replace(upper(uuid()), '-', '')))", true, "CREATE TABLE `t` (`a` VARCHAR(32) DEFAULT (REPLACE(UPPER(UUID()), _UTF8MB4'-', _UTF8MB4'')))"},
		{"create table t (a varchar(32) default replace(upper(uuid()), '-', ''))", true, "CREATE TABLE `t` (`a` VARCHAR(32) DEFAULT (REPLACE(UPPER(UUID()), _UTF8MB4'-', _UTF8MB4'')))"},
		{"create table t (a varchar(32) default (replace(convert(upper(uuid()) using utf8mb4), '-', '')))", true, "CREATE TABLE `t` (`a` VARCHAR(32) DEFAULT (REPLACE(CONVERT(UPPER(UUID()) USING 'utf8mb4'), _UTF8MB4'-', _UTF8MB4'')))"},
		{"create table t (a varchar(32) default replace(convert(upper(uuid()) using utf8mb4), '-', ''))", true, "CREATE TABLE `t` (`a` VARCHAR(32) DEFAULT (REPLACE(CONVERT(UPPER(UUID()) USING 'utf8mb4'), _UTF8MB4'-', _UTF8MB4'')))"},
		{"create table t (a int default upper(substring_index(user(),'@',1)))", true, "CREATE TABLE `t` (`a` INT DEFAULT (UPPER(SUBSTRING_INDEX(USER(), _UTF8MB4'@', 1))))"},
		{"create table t (a int default (upper(substring_index(user(),'@',1))))", true, "CREATE TABLE `t` (`a` INT DEFAULT (UPPER(SUBSTRING_INDEX(USER(), _UTF8MB4'@', 1))))"},
		{"create table t (a varchar(32) default (str_to_date('1980-01-01','%Y-%m-%d')))", true, "CREATE TABLE `t` (`a` VARCHAR(32) DEFAULT (STR_TO_DATE(_UTF8MB4'1980-01-01', _UTF8MB4'%Y-%m-%d')))"},
		{"create table t (a varchar(32) default str_to_date('1980-01-01','%Y-%m-%d'))", true, "CREATE TABLE `t` (`a` VARCHAR(32) DEFAULT (STR_TO_DATE(_UTF8MB4'1980-01-01', _UTF8MB4'%Y-%m-%d')))"},
		{"create table t (j json default (json_object()))", true, "CREATE TABLE `t` (`j` JSON DEFAULT (JSON_OBJECT()))"},
		{"create table t (j json default (json_array()))", true, "CREATE TABLE `t` (`j` JSON DEFAULT (JSON_ARRAY()))"},
		{"create table t (j json default (json_quote()))", true, "CREATE TABLE `t` (`j` JSON DEFAULT (JSON_QUOTE()))"},
		{"create table t (j json default (json_object('foo', 5, 'bar', 'barfoo')))", true, "CREATE TABLE `t` (`j` JSON DEFAULT (JSON_OBJECT(_UTF8MB4'foo', 5, _UTF8MB4'bar', _UTF8MB4'barfoo')))"},
		{"create table t (j json default (json_array(1,2,3)))", true, "CREATE TABLE `t` (`j` JSON DEFAULT (JSON_ARRAY(1, 2, 3)))"},
		{"create table t (j json default (json_quote('foobar')))", true, "CREATE TABLE `t` (`j` JSON DEFAULT (JSON_QUOTE(_UTF8MB4'foobar')))"},
		{"create table t (c char(33) default (nonexistingfunc('foobar')))", true, "CREATE TABLE `t` (`c` CHAR(33) DEFAULT (NONEXISTINGFUNC(_UTF8MB4'foobar')))"},
		{"create table t (c char(33) default 'foobar')", true, "CREATE TABLE `t` (`c` CHAR(33) DEFAULT _UTF8MB4'foobar')"},
		{"create table t (c char(33) default ('foobar'))", true, "CREATE TABLE `t` (`c` CHAR(33) DEFAULT _UTF8MB4'foobar')"},
		{"create table t (i int default (0))", true, "CREATE TABLE `t` (`i` INT DEFAULT 0)"},
		{"create table t (i int default (-1))", true, "CREATE TABLE `t` (`i` INT DEFAULT -1)"},
		{"create table t (i int default (+1))", true, "CREATE TABLE `t` (`i` INT DEFAULT +1)"},
		// For column default expression with column reference
		{"create table t (a int, b int, c char(33) default (b))", true, "CREATE TABLE `t` (`a` INT,`b` INT,`c` CHAR(33) DEFAULT (`b`))"},
		{"create table t (a int, b int, c char(33) default `b`)", false, ""},

		// For table option `ENCRYPTION`
		{"create table t (a int) encryption = 'n';", true, "CREATE TABLE `t` (`a` INT) ENCRYPTION = 'n'"},
		{"create table t (a int) encryption 'n';", true, "CREATE TABLE `t` (`a` INT) ENCRYPTION = 'n'"},
		{"alter table t encryption = 'y';", true, "ALTER TABLE `t` ENCRYPTION = 'y'"},
		{"alter table t encryption 'y';", true, "ALTER TABLE `t` ENCRYPTION = 'y'"},

		// For GLOBAL/LOCAL index
		{"create table t (a int key global)", true, "CREATE TABLE `t` (`a` INT PRIMARY KEY GLOBAL)"},
		{"create table t (a int key local)", true, "CREATE TABLE `t` (`a` INT PRIMARY KEY)"},
		{"create table t (a int primary key local)", true, "CREATE TABLE `t` (`a` INT PRIMARY KEY)"},
		{"create table t (a int primary key global)", true, "CREATE TABLE `t` (`a` INT PRIMARY KEY GLOBAL)"},
		{"create table t (a int UNIQUE local)", true, "CREATE TABLE `t` (`a` INT UNIQUE KEY)"},
		{"create table t (a int UNIQUE global)", true, "CREATE TABLE `t` (`a` INT UNIQUE KEY GLOBAL)"},
		{"create table t (a int UNIQUE key local)", true, "CREATE TABLE `t` (`a` INT UNIQUE KEY)"},
		{"create table t (a int UNIQUE key global)", true, "CREATE TABLE `t` (`a` INT UNIQUE KEY GLOBAL)"},
		{"alter table t add index (a)", true, "ALTER TABLE `t` ADD INDEX(`a`)"},
		{"alter table t add index (a) local", true, "ALTER TABLE `t` ADD INDEX(`a`)"},
		{"alter table t add index (a) global", true, "ALTER TABLE `t` ADD INDEX(`a`) GLOBAL"},
		{"alter table t add unique (a)", true, "ALTER TABLE `t` ADD UNIQUE(`a`)"},
		{"alter table t add unique (a) local", true, "ALTER TABLE `t` ADD UNIQUE(`a`)"},
		{"alter table t add unique (a) global", true, "ALTER TABLE `t` ADD UNIQUE(`a`) GLOBAL"},
		{"alter table t add unique key (a) global", true, "ALTER TABLE `t` ADD UNIQUE(`a`) GLOBAL"},
		{"alter table t add unique key (a)", true, "ALTER TABLE `t` ADD UNIQUE(`a`)"},
		{"alter table t add unique key (a) local", true, "ALTER TABLE `t` ADD UNIQUE(`a`)"},
		{"alter table t add primary key (a) global", true, "ALTER TABLE `t` ADD PRIMARY KEY(`a`) GLOBAL"},
		{"alter table t add primary key (a)", true, "ALTER TABLE `t` ADD PRIMARY KEY(`a`)"},
		{"alter table t add primary key (a) local", true, "ALTER TABLE `t` ADD PRIMARY KEY(`a`)"},
		{"create index i on t (a)", true, "CREATE INDEX `i` ON `t` (`a`)"},
		{"create index i on t (a) local", true, "CREATE INDEX `i` ON `t` (`a`)"},
		{"create index i on t (a) global", true, "CREATE INDEX `i` ON `t` (`a`) GLOBAL"},

		// for alter database/schema/table
		{"ALTER DATABASE t CHARACTER SET = 'utf8'", true, "ALTER DATABASE `t` CHARACTER SET = utf8"},
		{"ALTER DATABASE CHARACTER SET = 'utf8'", true, "ALTER DATABASE CHARACTER SET = utf8"},
		{"ALTER DATABASE t DEFAULT CHARACTER SET = 'utf8'", true, "ALTER DATABASE `t` CHARACTER SET = utf8"},
		{"ALTER SCHEMA t DEFAULT CHARACTER SET = 'utf8'", true, "ALTER DATABASE `t` CHARACTER SET = utf8"},
		{"ALTER SCHEMA DEFAULT CHARACTER SET = 'utf8'", true, "ALTER DATABASE CHARACTER SET = utf8"},
		{"ALTER SCHEMA t DEFAULT CHARSET = 'UTF8'", true, "ALTER DATABASE `t` CHARACTER SET = utf8"},

		{"ALTER DATABASE t COLLATE = binary", true, "ALTER DATABASE `t` COLLATE = binary"},
		{"ALTER DATABASE t CHARSET=binary COLLATE = binary", true, "ALTER DATABASE `t` CHARACTER SET = binary COLLATE = binary"},

		{"ALTER DATABASE t COLLATE = 'utf8_bin'", true, "ALTER DATABASE `t` COLLATE = utf8_bin"},
		{"ALTER DATABASE COLLATE = 'utf8_bin'", true, "ALTER DATABASE COLLATE = utf8_bin"},
		{"ALTER DATABASE t DEFAULT COLLATE = 'utf8_bin'", true, "ALTER DATABASE `t` COLLATE = utf8_bin"},
		{"ALTER SCHEMA t DEFAULT COLLATE = 'UTF8_BiN'", true, "ALTER DATABASE `t` COLLATE = utf8_bin"},
		{"ALTER SCHEMA DEFAULT COLLATE = 'UTF8_BiN'", true, "ALTER DATABASE COLLATE = utf8_bin"},
		{"ALTER SCHEMA `` DEFAULT COLLATE = 'UTF8_BiN'", true, "ALTER DATABASE `` COLLATE = utf8_bin"},

		{"ALTER DATABASE t CHARSET = 'utf8mb4' COLLATE = 'utf8_bin'", true, "ALTER DATABASE `t` CHARACTER SET = utf8mb4 COLLATE = utf8_bin"},
		{
			"ALTER DATABASE t DEFAULT CHARSET = 'utf8mb4' DEFAULT COLLATE = 'utf8mb4_general_ci' CHARACTER SET = 'utf8' COLLATE = 'utf8mb4_bin'",
			true,
			"ALTER DATABASE `t` CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci CHARACTER SET = utf8 COLLATE = utf8mb4_bin",
		},
		{"ALTER DATABASE DEFAULT CHARSET = 'utf8mb4' COLLATE = 'utf8_bin'", true, "ALTER DATABASE CHARACTER SET = utf8mb4 COLLATE = utf8_bin"},
		{
			"ALTER DATABASE DEFAULT CHARSET = 'utf8mb4' DEFAULT COLLATE = 'utf8mb4_general_ci' CHARACTER SET = 'utf8' COLLATE = 'utf8mb4_bin'",
			true,
			"ALTER DATABASE CHARACTER SET = utf8mb4 COLLATE = utf8mb4_general_ci CHARACTER SET = utf8 COLLATE = utf8mb4_bin",
		},

		{"ALTER TABLE t ADD COLUMN (a SMALLINT UNSIGNED)", true, "ALTER TABLE `t` ADD COLUMN (`a` SMALLINT UNSIGNED)"},
		{"ALTER TABLE t.* ADD COLUMN (a SMALLINT UNSIGNED)", false, ""},
		{"ALTER TABLE t ADD COLUMN IF NOT EXISTS (a SMALLINT UNSIGNED)", true, "ALTER TABLE `t` ADD COLUMN IF NOT EXISTS (`a` SMALLINT UNSIGNED)"},
		{"ALTER TABLE ADD COLUMN (a SMALLINT UNSIGNED)", false, ""},
		{"ALTER TABLE t ADD COLUMN (a SMALLINT UNSIGNED, b varchar(255))", true, "ALTER TABLE `t` ADD COLUMN (`a` SMALLINT UNSIGNED, `b` VARCHAR(255))"},
		{"ALTER TABLE t ADD COLUMN IF NOT EXISTS (a SMALLINT UNSIGNED, b varchar(255))", true, "ALTER TABLE `t` ADD COLUMN IF NOT EXISTS (`a` SMALLINT UNSIGNED, `b` VARCHAR(255))"},
		{"ALTER TABLE t ADD COLUMN (a SMALLINT UNSIGNED FIRST)", false, ""},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED FIRST", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED FIRST"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED AFTER b", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED AFTER `b`"},
		{"ALTER TABLE t ADD COLUMN IF NOT EXISTS a SMALLINT UNSIGNED AFTER b", true, "ALTER TABLE `t` ADD COLUMN IF NOT EXISTS `a` SMALLINT UNSIGNED AFTER `b`"},
		{"ALTER TABLE employees ADD PARTITION", true, "ALTER TABLE `employees` ADD PARTITION"},
		{"ALTER TABLE employees ADD PARTITION ( PARTITION P1 VALUES LESS THAN (2010))", true, "ALTER TABLE `employees` ADD PARTITION (PARTITION `P1` VALUES LESS THAN (2010))"},
		{"ALTER TABLE employees ADD PARTITION ( PARTITION P2 VALUES LESS THAN MAXVALUE)", true, "ALTER TABLE `employees` ADD PARTITION (PARTITION `P2` VALUES LESS THAN (MAXVALUE))"},
		{"ALTER TABLE employees ADD PARTITION IF NOT EXISTS ( PARTITION P2 VALUES LESS THAN MAXVALUE)", true, "ALTER TABLE `employees` ADD PARTITION IF NOT EXISTS (PARTITION `P2` VALUES LESS THAN (MAXVALUE))"},
		{"ALTER TABLE employees ADD PARTITION IF NOT EXISTS PARTITIONS 5", true, "ALTER TABLE `employees` ADD PARTITION IF NOT EXISTS PARTITIONS 5"},
		{`ALTER TABLE employees ADD PARTITION (
				PARTITION P1 VALUES LESS THAN (2010),
				PARTITION P2 VALUES LESS THAN (2015),
				PARTITION P3 VALUES LESS THAN MAXVALUE)`, true, "ALTER TABLE `employees` ADD PARTITION (PARTITION `P1` VALUES LESS THAN (2010), PARTITION `P2` VALUES LESS THAN (2015), PARTITION `P3` VALUES LESS THAN (MAXVALUE))"},
		{"alter table t add partition (partition x values in ((3, 4), (5, 6)))", true, "ALTER TABLE `t` ADD PARTITION (PARTITION `x` VALUES IN ((3, 4), (5, 6)))"},
		{"ALTER TABLE employees ADD PARTITION NO_WRITE_TO_BINLOG", true, "ALTER TABLE `employees` ADD PARTITION NO_WRITE_TO_BINLOG"},
		{"ALTER TABLE employees ADD PARTITION NO_WRITE_TO_BINLOG PARTITIONS 10", true, "ALTER TABLE `employees` ADD PARTITION NO_WRITE_TO_BINLOG PARTITIONS 10"},
		// LOCAL is alias to NO_WRITE_TO_BINLOG
		{"ALTER TABLE employees ADD PARTITION LOCAL", true, "ALTER TABLE `employees` ADD PARTITION NO_WRITE_TO_BINLOG"},
		{"ALTER TABLE employees ADD PARTITION LOCAL PARTITIONS 10", true, "ALTER TABLE `employees` ADD PARTITION NO_WRITE_TO_BINLOG PARTITIONS 10"},

		// For rebuild table partition statement.
		{"ALTER TABLE t_n REBUILD PARTITION ALL", true, "ALTER TABLE `t_n` REBUILD PARTITION ALL"},
		{"ALTER TABLE d_n.t_n REBUILD PARTITION LOCAL ALL", true, "ALTER TABLE `d_n`.`t_n` REBUILD PARTITION NO_WRITE_TO_BINLOG ALL"},
		{"ALTER TABLE t_n REBUILD PARTITION LOCAL ident", true, "ALTER TABLE `t_n` REBUILD PARTITION NO_WRITE_TO_BINLOG `ident`"},
		{"ALTER TABLE t_n REBUILD PARTITION NO_WRITE_TO_BINLOG ident , ident", true, "ALTER TABLE `t_n` REBUILD PARTITION NO_WRITE_TO_BINLOG `ident`,`ident`"},
		// The first `LOCAL` should be recognized as unreserved keyword `LOCAL` (alias to `NO_WRITE_TO_BINLOG`),
		// and the remains should re recognized as identifier, used as partition name here.
		{"ALTER TABLE t_n REBUILD PARTITION LOCAL", false, ""},
		{"ALTER TABLE t_n REBUILD PARTITION LOCAL local", true, "ALTER TABLE `t_n` REBUILD PARTITION NO_WRITE_TO_BINLOG `local`"},
		{"ALTER TABLE t_n REBUILD PARTITION LOCAL local, local", true, "ALTER TABLE `t_n` REBUILD PARTITION NO_WRITE_TO_BINLOG `local`,`local`"},

		// For drop table partition statement.
		{"alter table t drop partition p1;", true, "ALTER TABLE `t` DROP PARTITION `p1`"},
		{"alter table t drop partition p2;", true, "ALTER TABLE `t` DROP PARTITION `p2`"},
		{"alter table t drop partition if exists p2;", true, "ALTER TABLE `t` DROP PARTITION IF EXISTS `p2`"},
		{"alter table t drop partition p1, p2;", true, "ALTER TABLE `t` DROP PARTITION `p1`,`p2`"},
		{"alter table t drop partition if exists p1, p2;", true, "ALTER TABLE `t` DROP PARTITION IF EXISTS `p1`,`p2`"},
		// For check table partition statement
		{"alter table t check partition all;", true, "ALTER TABLE `t` CHECK PARTITION ALL"},
		{"alter table t check partition p;", true, "ALTER TABLE `t` CHECK PARTITION `p`"},
		{"alter table t check partition p1, p2;", true, "ALTER TABLE `t` CHECK PARTITION `p1`,`p2`"},
		{"alter table employees add partition partitions 1;", true, "ALTER TABLE `employees` ADD PARTITION PARTITIONS 1"},
		{"alter table employees add partition partitions 2;", true, "ALTER TABLE `employees` ADD PARTITION PARTITIONS 2"},
		{"alter table clients coalesce partition 3;", true, "ALTER TABLE `clients` COALESCE PARTITION 3"},
		{"alter table clients coalesce partition 4;", true, "ALTER TABLE `clients` COALESCE PARTITION 4"},
		{"alter table clients coalesce partition no_write_to_binlog 4;", true, "ALTER TABLE `clients` COALESCE PARTITION NO_WRITE_TO_BINLOG 4"},
		{"alter table clients coalesce partition local 4;", true, "ALTER TABLE `clients` COALESCE PARTITION NO_WRITE_TO_BINLOG 4"},
		{"ALTER TABLE t DISABLE KEYS", true, "ALTER TABLE `t` DISABLE KEYS"},
		{"ALTER TABLE t ENABLE KEYS", true, "ALTER TABLE `t` ENABLE KEYS"},
		{"ALTER TABLE t MODIFY COLUMN a varchar(255)", true, "ALTER TABLE `t` MODIFY COLUMN `a` VARCHAR(255)"},
		{"ALTER TABLE t MODIFY COLUMN IF EXISTS a varchar(255)", true, "ALTER TABLE `t` MODIFY COLUMN IF EXISTS `a` VARCHAR(255)"},
		{"ALTER TABLE t CHANGE COLUMN a b varchar(255)", true, "ALTER TABLE `t` CHANGE COLUMN `a` `b` VARCHAR(255)"},
		{"ALTER TABLE t CHANGE COLUMN IF EXISTS a b varchar(255)", true, "ALTER TABLE `t` CHANGE COLUMN IF EXISTS `a` `b` VARCHAR(255)"},
		{"ALTER TABLE t CHANGE COLUMN a b varchar(255) CHARACTER SET UTF8 BINARY", true, "ALTER TABLE `t` CHANGE COLUMN `a` `b` VARCHAR(255) BINARY CHARACTER SET UTF8"},
		{"ALTER TABLE t CHANGE COLUMN a b varchar(255) FIRST", true, "ALTER TABLE `t` CHANGE COLUMN `a` `b` VARCHAR(255) FIRST"},

		// For alter table rename statement.
		{"ALTER TABLE db.t RENAME to db1.t1", true, "ALTER TABLE `db`.`t` RENAME AS `db1`.`t1`"},
		{"ALTER TABLE db.t RENAME db1.t1", true, "ALTER TABLE `db`.`t` RENAME AS `db1`.`t1`"},
		{"ALTER TABLE db.t RENAME = db1.t1", true, "ALTER TABLE `db`.`t` RENAME AS `db1`.`t1`"},
		{"ALTER TABLE db.t RENAME as db1.t1", true, "ALTER TABLE `db`.`t` RENAME AS `db1`.`t1`"},
		{"ALTER TABLE t RENAME to t1", true, "ALTER TABLE `t` RENAME AS `t1`"},
		{"ALTER TABLE t RENAME t1", true, "ALTER TABLE `t` RENAME AS `t1`"},
		{"ALTER TABLE t RENAME = t1", true, "ALTER TABLE `t` RENAME AS `t1`"},
		{"ALTER TABLE t RENAME as t1", true, "ALTER TABLE `t` RENAME AS `t1`"},

		// For #499, alter table order by
		{"ALTER TABLE t_n ORDER BY ident", true, "ALTER TABLE `t_n` ORDER BY `ident`"},
		{"ALTER TABLE t_n ORDER BY ident ASC", true, "ALTER TABLE `t_n` ORDER BY `ident`"},
		{"ALTER TABLE t_n ORDER BY ident DESC", true, "ALTER TABLE `t_n` ORDER BY `ident` DESC"},
		{"ALTER TABLE t_n ORDER BY ident1, ident2", true, "ALTER TABLE `t_n` ORDER BY `ident1`,`ident2`"},
		{"ALTER TABLE t_n ORDER BY ident1 ASC, ident2", true, "ALTER TABLE `t_n` ORDER BY `ident1`,`ident2`"},
		{"ALTER TABLE t_n ORDER BY ident1 ASC, ident2 ASC", true, "ALTER TABLE `t_n` ORDER BY `ident1`,`ident2`"},
		{"ALTER TABLE t_n ORDER BY ident1 ASC, ident2 DESC", true, "ALTER TABLE `t_n` ORDER BY `ident1`,`ident2` DESC"},
		{"ALTER TABLE t_n ORDER BY ident1 DESC, ident2", true, "ALTER TABLE `t_n` ORDER BY `ident1` DESC,`ident2`"},
		{"ALTER TABLE t_n ORDER BY ident1 DESC, ident2 ASC", true, "ALTER TABLE `t_n` ORDER BY `ident1` DESC,`ident2`"},
		{"ALTER TABLE t_n ORDER BY ident1 DESC, ident2 DESC", true, "ALTER TABLE `t_n` ORDER BY `ident1` DESC,`ident2` DESC"},
		{"ALTER TABLE t_n ORDER BY ident1, ident2, ident3", true, "ALTER TABLE `t_n` ORDER BY `ident1`,`ident2`,`ident3`"},
		{"ALTER TABLE t_n ORDER BY ident1, ident2, ident3 ASC", true, "ALTER TABLE `t_n` ORDER BY `ident1`,`ident2`,`ident3`"},
		{"ALTER TABLE t_n ORDER BY ident1, ident2, ident3 DESC", true, "ALTER TABLE `t_n` ORDER BY `ident1`,`ident2`,`ident3` DESC"},
		{"ALTER TABLE t_n ORDER BY ident1 ASC, ident2 ASC, ident3 ASC", true, "ALTER TABLE `t_n` ORDER BY `ident1`,`ident2`,`ident3`"},
		{"ALTER TABLE t_n ORDER BY ident1 DESC, ident2 DESC, ident3 DESC", true, "ALTER TABLE `t_n` ORDER BY `ident1` DESC,`ident2` DESC,`ident3` DESC"},

		// For alter table rename column statement.
		{"ALTER TABLE t RENAME COLUMN a TO b", true, "ALTER TABLE `t` RENAME COLUMN `a` TO `b`"},
		{"ALTER TABLE t RENAME COLUMN t.a TO t.b", false, ""},
		{"ALTER TABLE t RENAME COLUMN a TO t.b", false, ""},
		{"ALTER TABLE t RENAME COLUMN t.a TO b", false, ""},

		{"ALTER TABLE t ALTER COLUMN a SET DEFAULT 1", true, "ALTER TABLE `t` ALTER COLUMN `a` SET DEFAULT 1"},
		{"ALTER TABLE t ALTER a SET DEFAULT 1", true, "ALTER TABLE `t` ALTER COLUMN `a` SET DEFAULT 1"},
		{"ALTER TABLE t ALTER COLUMN a SET DEFAULT CURRENT_TIMESTAMP", false, ""},
		{"ALTER TABLE t ALTER COLUMN a SET DEFAULT NOW()", false, ""},
		{"ALTER TABLE t ALTER COLUMN a SET DEFAULT 1+1", false, ""},
		{"ALTER TABLE t ALTER COLUMN a SET DEFAULT (CURRENT_TIMESTAMP())", true, "ALTER TABLE `t` ALTER COLUMN `a` SET DEFAULT (CURRENT_TIMESTAMP())"},
		{"ALTER TABLE t ALTER COLUMN a SET DEFAULT (NOW())", true, "ALTER TABLE `t` ALTER COLUMN `a` SET DEFAULT (NOW())"},
		{"ALTER TABLE t ALTER COLUMN a SET DEFAULT (1+1)", true, "ALTER TABLE `t` ALTER COLUMN `a` SET DEFAULT (1+1)"},
		{"ALTER TABLE t ALTER COLUMN a SET DEFAULT (1)", true, "ALTER TABLE `t` ALTER COLUMN `a` SET DEFAULT 1"},
		{"ALTER TABLE t ALTER COLUMN a DROP DEFAULT", true, "ALTER TABLE `t` ALTER COLUMN `a` DROP DEFAULT"},
		{"ALTER TABLE t ALTER a DROP DEFAULT", true, "ALTER TABLE `t` ALTER COLUMN `a` DROP DEFAULT"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, lock=none", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = NONE"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, lock=default", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = DEFAULT"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, lock=shared", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = SHARED"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, lock=exclusive", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = EXCLUSIVE"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, lock none", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = NONE"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, lock default", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = DEFAULT"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, lock shared", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = SHARED"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, lock exclusive", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = EXCLUSIVE"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, LOCK=NONE", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = NONE"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, LOCK=DEFAULT", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = DEFAULT"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, LOCK=SHARED", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = SHARED"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, LOCK=EXCLUSIVE", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, LOCK = EXCLUSIVE"},
		{"ALTER TABLE t ADD FULLTEXT KEY `FullText` (`name` ASC)", true, "ALTER TABLE `t` ADD FULLTEXT `FullText`(`name`)"},
		{"ALTER TABLE t ADD FULLTEXT `FullText` (`name` ASC)", true, "ALTER TABLE `t` ADD FULLTEXT `FullText`(`name`)"},
		{"ALTER TABLE t ADD FULLTEXT INDEX `FullText` (`name` ASC)", true, "ALTER TABLE `t` ADD FULLTEXT `FullText`(`name`)"},
		{"ALTER TABLE t ADD INDEX (a) USING BTREE COMMENT 'a'", true, "ALTER TABLE `t` ADD INDEX(`a`) USING BTREE COMMENT 'a'"},
		{"ALTER TABLE t ADD INDEX IF NOT EXISTS (a) USING BTREE COMMENT 'a'", true, "ALTER TABLE `t` ADD INDEX IF NOT EXISTS(`a`) USING BTREE COMMENT 'a'"},
		{"ALTER TABLE t ADD INDEX (a) USING RTREE COMMENT 'a'", true, "ALTER TABLE `t` ADD INDEX(`a`) USING RTREE COMMENT 'a'"},
		{"ALTER TABLE t ADD INDEX (a) PRE_SPLIT_REGIONS = 4", true, "ALTER TABLE `t` ADD INDEX(`a`) PRE_SPLIT_REGIONS = 4"},
		{"ALTER TABLE t ADD INDEX (a) PRE_SPLIT_REGIONS 4", true, "ALTER TABLE `t` ADD INDEX(`a`) PRE_SPLIT_REGIONS = 4"},
		{"ALTER TABLE t ADD INDEX (a) PRE_SPLIT_REGIONS = 'a'", false, ""},
		{"ALTER TABLE t ADD PRIMARY KEY (a) CLUSTERED PRE_SPLIT_REGIONS = 4", true, "ALTER TABLE `t` ADD PRIMARY KEY(`a`) CLUSTERED PRE_SPLIT_REGIONS = 4"},
		{"ALTER TABLE t ADD PRIMARY KEY (a) PRE_SPLIT_REGIONS = 4 NONCLUSTERED", true, "ALTER TABLE `t` ADD PRIMARY KEY(`a`) NONCLUSTERED PRE_SPLIT_REGIONS = 4"},
		{"ALTER TABLE t ADD INDEX (a) PRE_SPLIT_REGIONS = (between (1, 'a') and (2, 'b') regions 4);", true, "ALTER TABLE `t` ADD INDEX(`a`) PRE_SPLIT_REGIONS = (BETWEEN (1,_UTF8MB4'a') AND (2,_UTF8MB4'b') REGIONS 4)"},
		{"ALTER TABLE t ADD INDEX (a) PRE_SPLIT_REGIONS = (by (1, 'a'), (2, 'b'), (3, 'c'));", true, "ALTER TABLE `t` ADD INDEX(`a`) PRE_SPLIT_REGIONS = (BY (1,_UTF8MB4'a'),(2,_UTF8MB4'b'),(3,_UTF8MB4'c'))"},
		{"ALTER TABLE t ADD INDEX (a) comment 'a' PRE_SPLIT_REGIONS = (between (1, 'a') and (2, 'b') regions 4);", true, "ALTER TABLE `t` ADD INDEX(`a`) COMMENT 'a' PRE_SPLIT_REGIONS = (BETWEEN (1,_UTF8MB4'a') AND (2,_UTF8MB4'b') REGIONS 4)"},
		{"CREATE INDEX idx ON t (a, b) pre_split_regions = 100", true, "CREATE INDEX `idx` ON `t` (`a`, `b`) PRE_SPLIT_REGIONS = 100"},
		{"CREATE INDEX idx ON t (a, b) PRE_SPLIT_REGIONS = (between (1, 'a') and (2, 'b') regions 4);", true, "CREATE INDEX `idx` ON `t` (`a`, `b`) PRE_SPLIT_REGIONS = (BETWEEN (1,_UTF8MB4'a') AND (2,_UTF8MB4'b') REGIONS 4)"},
		{"ALTER TABLE t ADD INDEX idx(a) pre_split_regions = 100, ADD INDEX idx2(b) pre_split_regions = (by(1),(2),(3))", true, "ALTER TABLE `t` ADD INDEX `idx`(`a`) PRE_SPLIT_REGIONS = 100, ADD INDEX `idx2`(`b`) PRE_SPLIT_REGIONS = (BY (1),(2),(3))"},
		{"ALTER TABLE t ADD KEY (a) USING HASH COMMENT 'a'", true, "ALTER TABLE `t` ADD INDEX(`a`) USING HASH COMMENT 'a'"},
		{"ALTER TABLE t ADD INDEX (a) USING BTREE /*T![global_index] GLOBAL */ COMMENT 'a'", true, "ALTER TABLE `t` ADD INDEX(`a`) USING BTREE COMMENT 'a' GLOBAL"},
		{"ALTER TABLE t ADD UNIQUE INDEX (a) /*T![global_index] GLOBAL */", true, "ALTER TABLE `t` ADD UNIQUE(`a`) GLOBAL"},
		{"ALTER TABLE t ADD UNIQUE INDEX (a) LOCAL", true, "ALTER TABLE `t` ADD UNIQUE(`a`)"},
		{"ALTER TABLE t ADD KEY IF NOT EXISTS (a) USING HASH COMMENT 'a'", true, "ALTER TABLE `t` ADD INDEX IF NOT EXISTS(`a`) USING HASH COMMENT 'a'"},
		{"ALTER TABLE t ADD PRIMARY KEY ident USING RTREE ( a DESC , b   )", true, "ALTER TABLE `t` ADD PRIMARY KEY `ident`(`a` DESC, `b`) USING RTREE"},
		{"ALTER TABLE t ADD KEY USING RTREE   ( a ) ", true, "ALTER TABLE `t` ADD INDEX(`a`) USING RTREE"},
		{"ALTER TABLE t ADD KEY USING RTREE ( ident ASC , ident ( 123 ) )", true, "ALTER TABLE `t` ADD INDEX(`ident`, `ident`(123)) USING RTREE"},
		{"ALTER TABLE t ADD PRIMARY KEY (a) COMMENT 'a'", true, "ALTER TABLE `t` ADD PRIMARY KEY(`a`) COMMENT 'a'"},
		{"ALTER TABLE t ADD UNIQUE (a) COMMENT 'a'", true, "ALTER TABLE `t` ADD UNIQUE(`a`) COMMENT 'a'"},
		{"ALTER TABLE t ADD UNIQUE KEY (a) COMMENT 'a'", true, "ALTER TABLE `t` ADD UNIQUE(`a`) COMMENT 'a'"},
		{"ALTER TABLE t ADD UNIQUE INDEX (a) COMMENT 'a'", true, "ALTER TABLE `t` ADD UNIQUE(`a`) COMMENT 'a'"},
		{"ALTER TABLE t ADD VECTOR (a) USING HNSW COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD VECTOR (VEC_COSINE_DISTANCE(a)) USING HNSW COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD VECTOR ((VEC_COSINE_DISTANCE(a))) USING HASH COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD VECTOR ((VEC_COSINE_DISTANCE(a, b))) USING HNSW COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD VECTOR KEY ((VEC_COSINE_DISTANCE(a, b))) USING HNSW COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD VECTOR INDEX ((VEC_COSINE_DISTANCE(a, b))) USING HNSW COMMENT 'a'", true, "ALTER TABLE `t` ADD VECTOR INDEX((VEC_COSINE_DISTANCE(`a`, `b`))) USING HNSW COMMENT 'a'"},
		{"ALTER TABLE t ADD VECTOR INDEX ((lower(a))) USING HNSW COMMENT 'a'", true, "ALTER TABLE `t` ADD VECTOR INDEX((LOWER(`a`))) USING HNSW COMMENT 'a'"},
		{"ALTER TABLE t ADD VECTOR INDEX ((VEC_COSINE_DISTANCE(a), a)) USING HNSW COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD VECTOR INDEX (a, (VEC_COSINE_DISTANCE(a))) USING HNSW COMMENT 'a'", true, "ALTER TABLE `t` ADD VECTOR INDEX(`a`, (VEC_COSINE_DISTANCE(`a`))) USING HNSW COMMENT 'a'"},
		{"ALTER TABLE t ADD VECTOR INDEX ((VEC_COSINE_DISTANCE(a))) USING HYPO COMMENT 'a'", true, "ALTER TABLE `t` ADD VECTOR INDEX((VEC_COSINE_DISTANCE(`a`))) USING HYPO COMMENT 'a'"},
		{"ALTER TABLE t ADD VECTOR INDEX ((VEC_COSINE_DISTANCE(a))) COMMENT 'a'", true, "ALTER TABLE `t` ADD VECTOR INDEX((VEC_COSINE_DISTANCE(`a`))) COMMENT 'a'"},
		{"ALTER TABLE t ADD VECTOR INDEX ((VEC_COSINE_DISTANCE(a))) USING HNSW COMMENT 'a'", true, "ALTER TABLE `t` ADD VECTOR INDEX((VEC_COSINE_DISTANCE(`a`))) USING HNSW COMMENT 'a'"},
		{"ALTER TABLE t ADD VECTOR INDEX IF NOT EXISTS ((VEC_COSINE_DISTANCE(a))) USING HNSW COMMENT 'a'", true, "ALTER TABLE `t` ADD VECTOR INDEX IF NOT EXISTS((VEC_COSINE_DISTANCE(`a`))) USING HNSW COMMENT 'a'"},
		{"ALTER TABLE t ADD VECTOR INDEX IF NOT EXISTS ((VEC_COSINE_DISTANCE(a))) ADD_COLUMNAR_REPLICA_ON_DEMAND USING HNSW COMMENT 'a'", true, "ALTER TABLE `t` ADD VECTOR INDEX IF NOT EXISTS((VEC_COSINE_DISTANCE(`a`))) ADD_COLUMNAR_REPLICA_ON_DEMAND USING HNSW COMMENT 'a'"},
		{"ALTER TABLE t ADD COLUMNAR (a) USING INVERTED COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD COLUMNAR ((a - 1)) USING INVERTED COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD COLUMNAR (a) USING HASH COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD COLUMNAR (a, b) USING INVERTED COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD COLUMNAR KEY (a, b) USING INVERTED COMMENT 'a'", false, ""},
		{"ALTER TABLE t ADD COLUMNAR INDEX (a, b) USING INVERTED COMMENT 'a'", true, "ALTER TABLE `t` ADD COLUMNAR INDEX(`a`, `b`) USING INVERTED COMMENT 'a'"},
		{"ALTER TABLE t ADD COLUMNAR INDEX (a) USING INVERTED COMMENT 'a'", true, "ALTER TABLE `t` ADD COLUMNAR INDEX(`a`) USING INVERTED COMMENT 'a'"},
		{"ALTER TABLE t ADD COLUMNAR INDEX (a) USING HYPO COMMENT 'a'", true, "ALTER TABLE `t` ADD COLUMNAR INDEX(`a`) USING HYPO COMMENT 'a'"},
		{"ALTER TABLE t ADD COLUMNAR INDEX ((a - 1)) USING HYPO COMMENT 'a'", true, "ALTER TABLE `t` ADD COLUMNAR INDEX((`a`-1)) USING HYPO COMMENT 'a'"},
		{"ALTER TABLE t ADD COLUMNAR INDEX IF NOT EXISTS (a) USING INVERTED COMMENT 'a'", true, "ALTER TABLE `t` ADD COLUMNAR INDEX IF NOT EXISTS(`a`) USING INVERTED COMMENT 'a'"},
		{"ALTER TABLE t ADD CONSTRAINT fk_t2_id FOREIGN KEY (t2_id) REFERENCES t(id)", true, "ALTER TABLE `t` ADD CONSTRAINT `fk_t2_id` FOREIGN KEY (`t2_id`) REFERENCES `t`(`id`)"},
		{"ALTER TABLE t ADD CONSTRAINT fk_t2_id FOREIGN KEY IF NOT EXISTS (t2_id) REFERENCES t(id)", true, "ALTER TABLE `t` ADD CONSTRAINT `fk_t2_id` FOREIGN KEY IF NOT EXISTS (`t2_id`) REFERENCES `t`(`id`)"},
		{"ALTER TABLE t ADD CONSTRAINT c_1 CHECK (1+1) NOT ENFORCED, ADD UNIQUE (a)", true, "ALTER TABLE `t` ADD CONSTRAINT `c_1` CHECK(1+1) NOT ENFORCED, ADD UNIQUE(`a`)"},
		{"ALTER TABLE t ADD CONSTRAINT c_1 CHECK (1+1) ENFORCED, ADD UNIQUE (a)", true, "ALTER TABLE `t` ADD CONSTRAINT `c_1` CHECK(1+1) ENFORCED, ADD UNIQUE(`a`)"},
		{"ALTER TABLE t ADD CONSTRAINT c_1 CHECK (1+1), ADD UNIQUE (a)", true, "ALTER TABLE `t` ADD CONSTRAINT `c_1` CHECK(1+1) ENFORCED, ADD UNIQUE(`a`)"},
		{"ALTER TABLE t ENGINE ''", true, "ALTER TABLE `t` ENGINE = ''"},
		{"ALTER TABLE t ENGINE = ''", true, "ALTER TABLE `t` ENGINE = ''"},
		{"ALTER TABLE t ENGINE = 'innodb'", true, "ALTER TABLE `t` ENGINE = innodb"},
		{"ALTER TABLE t ENGINE = innodb", true, "ALTER TABLE `t` ENGINE = innodb"},
		{"ALTER TABLE `db`.`t` ENGINE = ``", true, "ALTER TABLE `db`.`t` ENGINE = ''"},
		{"ALTER TABLE t INSERT_METHOD = FIRST", true, "ALTER TABLE `t` INSERT_METHOD = FIRST"},
		{"ALTER TABLE t INSERT_METHOD LAST", true, "ALTER TABLE `t` INSERT_METHOD = LAST"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT UNSIGNED, ADD COLUMN a SMALLINT", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT UNSIGNED, ADD COLUMN `a` SMALLINT"},
		{"ALTER TABLE t ADD COLUMN a SMALLINT, ENGINE = '', default COLLATE = UTF8_GENERAL_CI", true, "ALTER TABLE `t` ADD COLUMN `a` SMALLINT, ENGINE = '', DEFAULT COLLATE = UTF8_GENERAL_CI"},
		{"ALTER TABLE t ENGINE = '', COMMENT='', default COLLATE = UTF8_GENERAL_CI", true, "ALTER TABLE `t` ENGINE = '', COMMENT = '', DEFAULT COLLATE = UTF8_GENERAL_CI"},
		{"ALTER TABLE t ENGINE = '', ADD COLUMN a SMALLINT", true, "ALTER TABLE `t` ENGINE = '', ADD COLUMN `a` SMALLINT"},
		{"ALTER TABLE t default COLLATE = UTF8_GENERAL_CI, ENGINE = '', ADD COLUMN a SMALLINT", true, "ALTER TABLE `t` DEFAULT COLLATE = UTF8_GENERAL_CI, ENGINE = '', ADD COLUMN `a` SMALLINT"},
		{"ALTER TABLE t shard_row_id_bits = 1", true, "ALTER TABLE `t` SHARD_ROW_ID_BITS = 1"},
		{"ALTER TABLE t AUTO_INCREMENT 3", true, "ALTER TABLE `t` AUTO_INCREMENT = 3"},
		{"ALTER TABLE t AUTO_INCREMENT = 3", true, "ALTER TABLE `t` AUTO_INCREMENT = 3"},
		{"ALTER TABLE t FORCE AUTO_INCREMENT 3", true, "ALTER TABLE `t` FORCE AUTO_INCREMENT = 3"},
		{"ALTER TABLE t FORCE AUTO_INCREMENT = 3", true, "ALTER TABLE `t` FORCE AUTO_INCREMENT = 3"},
		{"ALTER TABLE `hello-world@dev`.`User` ADD COLUMN `name` mediumtext CHARACTER SET UTF8MB4 COLLATE UTF8MB4_UNICODE_CI NOT NULL , ALGORITHM = DEFAULT;", true, "ALTER TABLE `hello-world@dev`.`User` ADD COLUMN `name` MEDIUMTEXT CHARACTER SET UTF8MB4 COLLATE utf8mb4_unicode_ci NOT NULL, ALGORITHM = DEFAULT"},
		{"ALTER TABLE `hello-world@dev`.`User` ADD COLUMN `name` mediumtext CHARACTER SET UTF8MB4 COLLATE UTF8MB4_UNICODE_CI NOT NULL , ALGORITHM = INPLACE;", true, "ALTER TABLE `hello-world@dev`.`User` ADD COLUMN `name` MEDIUMTEXT CHARACTER SET UTF8MB4 COLLATE utf8mb4_unicode_ci NOT NULL, ALGORITHM = INPLACE"},
		{"ALTER TABLE `hello-world@dev`.`User` ADD COLUMN `name` mediumtext CHARACTER SET UTF8MB4 COLLATE UTF8MB4_UNICODE_CI NOT NULL , ALGORITHM = COPY;", true, "ALTER TABLE `hello-world@dev`.`User` ADD COLUMN `name` MEDIUMTEXT CHARACTER SET UTF8MB4 COLLATE utf8mb4_unicode_ci NOT NULL, ALGORITHM = COPY"},
		{"ALTER TABLE `hello-world@dev`.`User` ADD COLUMN `name` MEDIUMTEXT CHARACTER SET UTF8MB4 COLLATE UTF8MB4_UNICODE_CI NOT NULL, ALGORITHM = INSTANT;", true, "ALTER TABLE `hello-world@dev`.`User` ADD COLUMN `name` MEDIUMTEXT CHARACTER SET UTF8MB4 COLLATE utf8mb4_unicode_ci NOT NULL, ALGORITHM = INSTANT"},
		{"ALTER TABLE t CONVERT TO CHARACTER SET UTF8;", true, "ALTER TABLE `t` CONVERT TO CHARACTER SET UTF8"},
		{"ALTER TABLE t CONVERT TO CHARSET UTF8;", true, "ALTER TABLE `t` CONVERT TO CHARACTER SET UTF8"},
		{"ALTER TABLE t CONVERT TO CHARACTER SET UTF8 COLLATE UTF8_BIN;", true, "ALTER TABLE `t` CONVERT TO CHARACTER SET UTF8 COLLATE UTF8_BIN"},
		{"ALTER TABLE t CONVERT TO CHARSET UTF8 COLLATE UTF8_BIN;", true, "ALTER TABLE `t` CONVERT TO CHARACTER SET UTF8 COLLATE UTF8_BIN"},

		// alter table convert to character set default, issue #498
		{"alter table d_n.t_n convert to character set default", true, "ALTER TABLE `d_n`.`t_n` CONVERT TO CHARACTER SET DEFAULT"},
		{"alter table d_n.t_n convert to charset default", true, "ALTER TABLE `d_n`.`t_n` CONVERT TO CHARACTER SET DEFAULT"},
		{"alter table d_n.t_n convert to char set default", true, "ALTER TABLE `d_n`.`t_n` CONVERT TO CHARACTER SET DEFAULT"},
		{"alter table d_n.t_n convert to character set default collate utf8mb4_0900_ai_ci", true, "ALTER TABLE `d_n`.`t_n` CONVERT TO CHARACTER SET DEFAULT COLLATE UTF8MB4_0900_AI_CI"},

		{"ALTER TABLE t FORCE", true, "ALTER TABLE `t` FORCE /* AlterTableForce is not supported */ "},
		{"ALTER TABLE t DROP INDEX;", false, "ALTER TABLE `t` DROP INDEX"},
		{"ALTER TABLE t DROP INDEX a", true, "ALTER TABLE `t` DROP INDEX `a`"},
		{"ALTER TABLE t DROP INDEX IF EXISTS a", true, "ALTER TABLE `t` DROP INDEX IF EXISTS `a`"},

		// For alter table alter index statement
		{"ALTER TABLE t ALTER INDEX a INVISIBLE", true, "ALTER TABLE `t` ALTER INDEX `a` INVISIBLE"},
		{"ALTER TABLE t ALTER INDEX a VISIBLE", true, "ALTER TABLE `t` ALTER INDEX `a` VISIBLE"},

		{"ALTER TABLE t DROP FOREIGN KEY a", true, "ALTER TABLE `t` DROP FOREIGN KEY `a`"},
		{"ALTER TABLE t DROP COLUMN a CASCADE", true, "ALTER TABLE `t` DROP COLUMN `a`"},
		{"ALTER TABLE t DROP COLUMN IF EXISTS a CASCADE", true, "ALTER TABLE `t` DROP COLUMN IF EXISTS `a`"},
		{`ALTER TABLE testTableCompression COMPRESSION="LZ4";`, true, "ALTER TABLE `testTableCompression` COMPRESSION = 'LZ4'"},
		{`ALTER TABLE t1 COMPRESSION="zlib";`, true, "ALTER TABLE `t1` COMPRESSION = 'zlib'"},
		{"ALTER TABLE t1", true, "ALTER TABLE `t1`"},
		{"ALTER TABLE t1 ,", false, ""},

		// For #6405
		{"ALTER TABLE t RENAME KEY a TO b;", true, "ALTER TABLE `t` RENAME INDEX `a` TO `b`"},
		{"ALTER TABLE t RENAME INDEX a TO b;", true, "ALTER TABLE `t` RENAME INDEX `a` TO `b`"},

		// For #497, support `ALTER TABLE ALTER CHECK` and `ALTER TABLE DROP CHECK` syntax
		{"ALTER TABLE d_n.t_n DROP CHECK ident;", true, "ALTER TABLE `d_n`.`t_n` DROP CHECK `ident`"},
		{"ALTER TABLE t_n LOCK = DEFAULT , DROP CHECK ident;", true, "ALTER TABLE `t_n` LOCK = DEFAULT, DROP CHECK `ident`"},
		{"ALTER TABLE t_n ALTER CHECK ident ENFORCED;", true, "ALTER TABLE `t_n` ALTER CHECK `ident` ENFORCED"},
		{"ALTER TABLE t_n ALTER CHECK ident NOT ENFORCED;", true, "ALTER TABLE `t_n` ALTER CHECK `ident` NOT ENFORCED"},
		{"ALTER TABLE t_n DROP CONSTRAINT ident", true, "ALTER TABLE `t_n` DROP CHECK `ident`"},
		{"ALTER TABLE t_n DROP CHECK ident", true, "ALTER TABLE `t_n` DROP CHECK `ident`"},
		{"ALTER TABLE t_n ALTER CONSTRAINT ident", false, ""},
		{"ALTER TABLE t_n ALTER CONSTRAINT ident enforced", true, "ALTER TABLE `t_n` ALTER CHECK `ident` ENFORCED"},
		{"ALTER TABLE t_n ALTER CHECK ident not enforced", true, "ALTER TABLE `t_n` ALTER CHECK `ident` NOT ENFORCED"},

		{"alter table t analyze partition a", true, "ANALYZE TABLE `t` PARTITION `a`"},
		{"alter table t analyze partition a with 4 buckets", true, "ANALYZE TABLE `t` PARTITION `a` WITH 4 BUCKETS"},
		{"alter table t analyze partition a index b", true, "ANALYZE TABLE `t` PARTITION `a` INDEX `b`"},
		{"alter table t analyze partition a index b with 4 buckets", true, "ANALYZE TABLE `t` PARTITION `a` INDEX `b` WITH 4 BUCKETS"},

		{"alter table t partition by hash(a)", true, "ALTER TABLE `t` PARTITION BY HASH (`a`) PARTITIONS 1"},
		{"alter table t add column a int partition by hash(a)", true, "ALTER TABLE `t` ADD COLUMN `a` INT PARTITION BY HASH (`a`) PARTITIONS 1"},
		{"alter table t add column a int partition by hash(a) update indexes (idx_a global)", true, "ALTER TABLE `t` ADD COLUMN `a` INT PARTITION BY HASH (`a`) PARTITIONS 1 UPDATE INDEXES (`idx_a` GLOBAL)"},
		{"alter table t add column a int partition by hash(a) update indexes (idx_a global, idx_b local)", true, "ALTER TABLE `t` ADD COLUMN `a` INT PARTITION BY HASH (`a`) PARTITIONS 1 UPDATE INDEXES (`idx_a` GLOBAL,`idx_b` LOCAL)"},
		{"alter table t add column a int partition by hash(a) update indexes (idx_a normal)", false, ""},
		{"alter table t add column a int partition by hash(a) update indexes (global)", false, ""},
		{"alter table t partition by range(a)", false, ""},
		{"alter table t partition by range(a) update indexes (a local)", false, ""},
		{"alter table t partition by range(a) (partition x values less than (75))", true, "ALTER TABLE `t` PARTITION BY RANGE (`a`) (PARTITION `x` VALUES LESS THAN (75))"},
		{"alter table t add column a int, partition by range(a) (partition x values less than (75))", false, ""},
		{"alter table t comment 'cmt' partition by hash(a)", true, "ALTER TABLE `t` COMMENT = 'cmt' PARTITION BY HASH (`a`) PARTITIONS 1"},
		{"alter table t enable keys, comment = 'cmt' partition by hash(a)", true, "ALTER TABLE `t` ENABLE KEYS, COMMENT = 'cmt' PARTITION BY HASH (`a`) PARTITIONS 1"},
		{"alter table t enable keys, comment = 'cmt', partition by hash(a)", false, ""},
		{"alter table t partition by hash(a) enable keys", false, ""},
		{"alter table t partition by hash(a), enable keys", false, ""},

		// Test keyword `FIELDS`
		{"alter table t partition by range FIELDS(a) (partition x values less than maxvalue)", true, "ALTER TABLE `t` PARTITION BY RANGE COLUMNS (`a`) (PARTITION `x` VALUES LESS THAN (MAXVALUE))"},
		{"alter table t partition by list FIELDS(a) (PARTITION p0 VALUES IN (5, 10, 15))", true, "ALTER TABLE `t` PARTITION BY LIST COLUMNS (`a`) (PARTITION `p0` VALUES IN (5, 10, 15))"},
		{"alter table t partition by range FIELDS(a,b,c) (partition p1 values less than (1,1,1));", true, "ALTER TABLE `t` PARTITION BY RANGE COLUMNS (`a`,`b`,`c`) (PARTITION `p1` VALUES LESS THAN (1, 1, 1))"},
		{"alter table t partition by list FIELDS(a,b,c) (PARTITION p0 VALUES IN ((5, 10, 15)))", true, "ALTER TABLE `t` PARTITION BY LIST COLUMNS (`a`,`b`,`c`) (PARTITION `p0` VALUES IN ((5, 10, 15)))"},

		{"alter table t with validation, add column b int as (a + 1)", true, "ALTER TABLE `t` WITH VALIDATION, ADD COLUMN `b` INT GENERATED ALWAYS AS(`a`+1) VIRTUAL"},
		{"alter table t without validation, add column b int as (a + 1)", true, "ALTER TABLE `t` WITHOUT VALIDATION, ADD COLUMN `b` INT GENERATED ALWAYS AS(`a`+1) VIRTUAL"},
		{"alter table t without validation, with validation, add column b int as (a + 1)", true, "ALTER TABLE `t` WITHOUT VALIDATION, WITH VALIDATION, ADD COLUMN `b` INT GENERATED ALWAYS AS(`a`+1) VIRTUAL"},
		{"alter table t with validation, modify column b int as (a + 2) ", true, "ALTER TABLE `t` WITH VALIDATION, MODIFY COLUMN `b` INT GENERATED ALWAYS AS(`a`+2) VIRTUAL"},
		{"alter table t with validation, change column b c int as (a + 2)", true, "ALTER TABLE `t` WITH VALIDATION, CHANGE COLUMN `b` `c` INT GENERATED ALWAYS AS(`a`+2) VIRTUAL"},

		{"ALTER TABLE d_n.t_n ADD PARTITION NO_WRITE_TO_BINLOG", true, "ALTER TABLE `d_n`.`t_n` ADD PARTITION NO_WRITE_TO_BINLOG"},
		{"ALTER TABLE d_n.t_n ADD PARTITION LOCAL", true, "ALTER TABLE `d_n`.`t_n` ADD PARTITION NO_WRITE_TO_BINLOG"},

		{"alter table t with validation, exchange partition p with table nt without validation;", true, "ALTER TABLE `t` WITH VALIDATION, EXCHANGE PARTITION `p` WITH TABLE `nt` WITHOUT VALIDATION"},
		{"alter table t exchange partition p with table nt with validation;", true, "ALTER TABLE `t` EXCHANGE PARTITION `p` WITH TABLE `nt`"},

		// For reorganize partition statement
		{"alter table t reorganize partition;", true, "ALTER TABLE `t` REORGANIZE PARTITION"},
		{"alter table t reorganize partition local;", true, "ALTER TABLE `t` REORGANIZE PARTITION NO_WRITE_TO_BINLOG"},
		{"alter table t reorganize partition no_write_to_binlog;", true, "ALTER TABLE `t` REORGANIZE PARTITION NO_WRITE_TO_BINLOG"},
		{"ALTER TABLE members REORGANIZE PARTITION n0 INTO (PARTITION s0 VALUES LESS THAN (1960), PARTITION s1 VALUES LESS THAN (1970));", true, "ALTER TABLE `members` REORGANIZE PARTITION `n0` INTO (PARTITION `s0` VALUES LESS THAN (1960), PARTITION `s1` VALUES LESS THAN (1970))"},
		{"ALTER TABLE members REORGANIZE PARTITION LOCAL n0 INTO (PARTITION s0 VALUES LESS THAN (1960), PARTITION s1 VALUES LESS THAN (1970));", true, "ALTER TABLE `members` REORGANIZE PARTITION NO_WRITE_TO_BINLOG `n0` INTO (PARTITION `s0` VALUES LESS THAN (1960), PARTITION `s1` VALUES LESS THAN (1970))"},
		{"ALTER TABLE members REORGANIZE PARTITION p1,p2,p3 INTO ( PARTITION s0 VALUES LESS THAN (1960), PARTITION s1 VALUES LESS THAN (1970));", true, "ALTER TABLE `members` REORGANIZE PARTITION `p1`,`p2`,`p3` INTO (PARTITION `s0` VALUES LESS THAN (1960), PARTITION `s1` VALUES LESS THAN (1970))"},
		{"alter table t reorganize partition remove partition;", false, ""},
		{"alter table t reorganize partition no_write_to_binlog remove into (partition p0 VALUES LESS THAN (1991));", true, "ALTER TABLE `t` REORGANIZE PARTITION NO_WRITE_TO_BINLOG `remove` INTO (PARTITION `p0` VALUES LESS THAN (1991))"},

		// alter attributes
		{"ALTER TABLE t ATTRIBUTES='str'", true, "ALTER TABLE `t` ATTRIBUTES='str'"},
		{"ALTER TABLE t ATTRIBUTES='str1,str2'", true, "ALTER TABLE `t` ATTRIBUTES='str1,str2'"},
		{"ALTER TABLE t ATTRIBUTES=\"str1,str2\"", true, "ALTER TABLE `t` ATTRIBUTES='str1,str2'"},
		{"ALTER TABLE t ATTRIBUTES 'str1,str2'", true, "ALTER TABLE `t` ATTRIBUTES='str1,str2'"},
		{"ALTER TABLE t ATTRIBUTES \"str1,str2\"", true, "ALTER TABLE `t` ATTRIBUTES='str1,str2'"},
		{"ALTER TABLE t ATTRIBUTES=DEFAULT", true, "ALTER TABLE `t` ATTRIBUTES=DEFAULT"},
		{"ALTER TABLE t ATTRIBUTES=default", true, "ALTER TABLE `t` ATTRIBUTES=DEFAULT"},
		{"ALTER TABLE t ATTRIBUTES=DeFaUlT", true, "ALTER TABLE `t` ATTRIBUTES=DEFAULT"},
		{"ALTER TABLE t ATTRIBUTES", false, ""},
		{"ALTER TABLE t PARTITION p ATTRIBUTES='str'", true, "ALTER TABLE `t` PARTITION `p` ATTRIBUTES='str'"},
		{"ALTER TABLE t PARTITION p ATTRIBUTES='str1,str2'", true, "ALTER TABLE `t` PARTITION `p` ATTRIBUTES='str1,str2'"},
		{"ALTER TABLE t PARTITION p ATTRIBUTES=\"str1,str2\"", true, "ALTER TABLE `t` PARTITION `p` ATTRIBUTES='str1,str2'"},
		{"ALTER TABLE t PARTITION p ATTRIBUTES 'str1,str2'", true, "ALTER TABLE `t` PARTITION `p` ATTRIBUTES='str1,str2'"},
		{"ALTER TABLE t PARTITION p ATTRIBUTES \"str1,str2\"", true, "ALTER TABLE `t` PARTITION `p` ATTRIBUTES='str1,str2'"},
		{"ALTER TABLE t PARTITION p ATTRIBUTES=DEFAULT", true, "ALTER TABLE `t` PARTITION `p` ATTRIBUTES=DEFAULT"},
		{"ALTER TABLE t PARTITION p ATTRIBUTES=default", true, "ALTER TABLE `t` PARTITION `p` ATTRIBUTES=DEFAULT"},
		{"ALTER TABLE t PARTITION p ATTRIBUTES=DeFaUlT", true, "ALTER TABLE `t` PARTITION `p` ATTRIBUTES=DEFAULT"},
		{"ALTER TABLE t PARTITION p ATTRIBUTES", false, ""},
		// For https://github.com/pingcap/tidb/issues/26778
		{"CREATE TABLE t1 (attributes int);", true, "CREATE TABLE `t1` (`attributes` INT)"},

		// For create index statement
		{"CREATE INDEX idx ON t (a)", true, "CREATE INDEX `idx` ON `t` (`a`)"},
		{"CREATE INDEX IF NOT EXISTS idx ON t (a)", true, "CREATE INDEX IF NOT EXISTS `idx` ON `t` (`a`)"},
		{"CREATE UNIQUE INDEX idx ON t (a)", true, "CREATE UNIQUE INDEX `idx` ON `t` (`a`)"},
		{"CREATE UNIQUE INDEX IF NOT EXISTS idx ON t (a)", true, "CREATE UNIQUE INDEX IF NOT EXISTS `idx` ON `t` (`a`)"},
		{"CREATE UNIQUE INDEX ident ON d_n.t_n ( ident , ident ASC ) TYPE BTREE", true, "CREATE UNIQUE INDEX `ident` ON `d_n`.`t_n` (`ident`, `ident`) USING BTREE"},
		{"CREATE UNIQUE INDEX ident ON d_n.t_n ( ident , ident ASC ) TYPE HASH", true, "CREATE UNIQUE INDEX `ident` ON `d_n`.`t_n` (`ident`, `ident`) USING HASH"},
		{"CREATE UNIQUE INDEX ident ON d_n.t_n ( ident , ident ASC ) TYPE RTREE", true, "CREATE UNIQUE INDEX `ident` ON `d_n`.`t_n` (`ident`, `ident`) USING RTREE"},
		{"CREATE UNIQUE INDEX ident TYPE BTREE ON d_n.t_n ( ident , ident ASC )", true, "CREATE UNIQUE INDEX `ident` ON `d_n`.`t_n` (`ident`, `ident`) USING BTREE"},
		{"CREATE UNIQUE INDEX ident USING BTREE ON d_n.t_n ( ident , ident ASC )", true, "CREATE UNIQUE INDEX `ident` ON `d_n`.`t_n` (`ident`, `ident`) USING BTREE"},
		{"CREATE SPATIAL INDEX idx ON t (a)", true, "CREATE SPATIAL INDEX `idx` ON `t` (`a`)"},
		{"CREATE SPATIAL INDEX IF NOT EXISTS idx ON t (a)", true, "CREATE SPATIAL INDEX IF NOT EXISTS `idx` ON `t` (`a`)"},
		{"CREATE FULLTEXT INDEX idx ON t (a)", true, "CREATE FULLTEXT INDEX `idx` ON `t` (`a`)"},
		{"CREATE FULLTEXT INDEX IF NOT EXISTS idx ON t (a)", true, "CREATE FULLTEXT INDEX IF NOT EXISTS `idx` ON `t` (`a`)"},
		{"CREATE FULLTEXT INDEX idx ON t (a) WITH PARSER ident", true, "CREATE FULLTEXT INDEX `idx` ON `t` (`a`) WITH PARSER `ident`"},
		{"CREATE FULLTEXT INDEX idx ON t (a) WITH PARSER ident comment 'string'", true, "CREATE FULLTEXT INDEX `idx` ON `t` (`a`) WITH PARSER `ident` COMMENT 'string'"},
		{"CREATE FULLTEXT INDEX idx ON t (a) comment 'string' with parser ident", true, "CREATE FULLTEXT INDEX `idx` ON `t` (`a`) WITH PARSER `ident` COMMENT 'string'"},
		{"CREATE FULLTEXT INDEX idx ON t (a) WITH PARSER ident comment 'string' lock default", true, "CREATE FULLTEXT INDEX `idx` ON `t` (`a`) WITH PARSER `ident` COMMENT 'string'"},
		{"CREATE INDEX idx ON t (a) USING HASH", true, "CREATE INDEX `idx` ON `t` (`a`) USING HASH"},
		{"CREATE INDEX idx ON t (a) COMMENT 'foo'", true, "CREATE INDEX `idx` ON `t` (`a`) COMMENT 'foo'"},
		{"CREATE INDEX idx ON t (a) USING HASH COMMENT 'foo'", true, "CREATE INDEX `idx` ON `t` (`a`) USING HASH COMMENT 'foo'"},
		{"CREATE INDEX idx ON t (a) LOCK=NONE", true, "CREATE INDEX `idx` ON `t` (`a`) LOCK = NONE"},
		{"CREATE INDEX idx USING BTREE ON t (a) USING HASH COMMENT 'foo'", true, "CREATE INDEX `idx` ON `t` (`a`) USING HASH COMMENT 'foo'"},
		{"CREATE INDEX idx USING BTREE ON t (a)", true, "CREATE INDEX `idx` ON `t` (`a`) USING BTREE"},
		{"CREATE INDEX idx ON t ( a ) VISIBLE", true, "CREATE INDEX `idx` ON `t` (`a`) VISIBLE"},
		{"CREATE INDEX idx ON t ( a ) INVISIBLE", true, "CREATE INDEX `idx` ON `t` (`a`) INVISIBLE"},
		{"CREATE INDEX idx ON t ( a ) INVISIBLE VISIBLE", true, "CREATE INDEX `idx` ON `t` (`a`) VISIBLE"},
		{"CREATE INDEX idx ON t ( a ) VISIBLE INVISIBLE", true, "CREATE INDEX `idx` ON `t` (`a`) INVISIBLE"},
		{"CREATE INDEX idx ON t ( a ) USING HASH VISIBLE", true, "CREATE INDEX `idx` ON `t` (`a`) USING HASH VISIBLE"},
		{"CREATE INDEX idx ON t ( a ) USING HASH INVISIBLE", true, "CREATE INDEX `idx` ON `t` (`a`) USING HASH INVISIBLE"},

		// For create vector index statement
		{"CREATE VECTOR INDEX idx ON t (a) USING HNSW ", true, "CREATE VECTOR INDEX `idx` ON `t` (`a`) USING HNSW"},
		{"CREATE VECTOR INDEX idx ON t (a, b) USING HNSW ", true, "CREATE VECTOR INDEX `idx` ON `t` (`a`, `b`) USING HNSW"},
		{"CREATE VECTOR INDEX idx ON t ((VEC_COSINE_DISTANCE(a)))", true, "CREATE VECTOR INDEX `idx` ON `t` ((VEC_COSINE_DISTANCE(`a`)))"},
		{"CREATE VECTOR INDEX idx ON t ((VEC_COSINE_DISTANCE(a))) TYPE BTREE", true, "CREATE VECTOR INDEX `idx` ON `t` ((VEC_COSINE_DISTANCE(`a`))) USING BTREE"},
		{"CREATE VECTOR INDEX idx ON t USING HNSW ((VEC_COSINE_DISTANCE(a)))", false, ""},
		{"CREATE VECTOR idx ON t ((VEC_COSINE_DISTANCE(a))) USING HNSW", false, ""},
		{"CREATE VECTOR INDEX idx ON t ((VEC_COSINE_DISTANCE(a)), a) USING HNSW", true, "CREATE VECTOR INDEX `idx` ON `t` ((VEC_COSINE_DISTANCE(`a`)), `a`) USING HNSW"},
		{"CREATE VECTOR INDEX idx ON t (a, (VEC_COSINE_DISTANCE(a))) USING HNSW", true, "CREATE VECTOR INDEX `idx` ON `t` (`a`, (VEC_COSINE_DISTANCE(`a`))) USING HNSW"},
		{"CREATE VECTOR KEY idx ON t ((VEC_COSINE_DISTANCE(a))) USING HNSW", false, ""},
		{"CREATE VECTOR INDEX idx ON t ((VEC_COSINE_DISTANCE(a))) USING HNSW", true, "CREATE VECTOR INDEX `idx` ON `t` ((VEC_COSINE_DISTANCE(`a`))) USING HNSW"},
		{"CREATE VECTOR INDEX IF NOT EXISTS idx ON t ((VEC_COSINE_DISTANCE(a))) USING HNSW", true, "CREATE VECTOR INDEX IF NOT EXISTS `idx` ON `t` ((VEC_COSINE_DISTANCE(`a`))) USING HNSW"},
		{"CREATE VECTOR INDEX IF NOT EXISTS idx ON t ((VEC_COSINE_DISTANCE(a))) TYPE HNSW", true, "CREATE VECTOR INDEX IF NOT EXISTS `idx` ON `t` ((VEC_COSINE_DISTANCE(`a`))) USING HNSW"},
		{"CREATE VECTOR INDEX ident TYPE HNSW ON d_n.t_n ((VEC_COSINE_DISTANCE(a)))", true, "CREATE VECTOR INDEX `ident` ON `d_n`.`t_n` ((VEC_COSINE_DISTANCE(`a`))) USING HNSW"},
		{"CREATE VECTOR INDEX idx USING HNSW ON t ((VEC_COSINE_DISTANCE(a)))", true, "CREATE VECTOR INDEX `idx` ON `t` ((VEC_COSINE_DISTANCE(`a`))) USING HNSW"},
		{"CREATE VECTOR INDEX ident ON d_n.t_n ( ident , ident ASC ) TYPE HNSW", true, "CREATE VECTOR INDEX `ident` ON `d_n`.`t_n` (`ident`, `ident`) USING HNSW"},
		{"CREATE UNIQUE INDEX ident USING HNSW ON d_n.t_n ( ident , ident ASC )", true, "CREATE UNIQUE INDEX `ident` ON `d_n`.`t_n` (`ident`, `ident`) USING HNSW"},

		// For create index with algorithm
		{"CREATE INDEX idx ON t ( a ) ALGORITHM = DEFAULT", true, "CREATE INDEX `idx` ON `t` (`a`)"},
		{"CREATE INDEX idx ON t ( a ) ALGORITHM DEFAULT", true, "CREATE INDEX `idx` ON `t` (`a`)"},
		{"CREATE INDEX idx ON t ( a ) ALGORITHM = INPLACE", true, "CREATE INDEX `idx` ON `t` (`a`) ALGORITHM = INPLACE"},
		{"CREATE INDEX idx ON t ( a ) ALGORITHM INPLACE", true, "CREATE INDEX `idx` ON `t` (`a`) ALGORITHM = INPLACE"},
		{"CREATE INDEX idx ON t ( a ) ALGORITHM = COPY", true, "CREATE INDEX `idx` ON `t` (`a`) ALGORITHM = COPY"},
		{"CREATE INDEX idx ON t ( a ) ALGORITHM COPY", true, "CREATE INDEX `idx` ON `t` (`a`) ALGORITHM = COPY"},
		{"CREATE INDEX idx ON t ( a ) ALGORITHM = DEFAULT LOCK = DEFAULT", true, "CREATE INDEX `idx` ON `t` (`a`)"},
		{"CREATE INDEX idx ON t ( a ) LOCK = DEFAULT ALGORITHM = DEFAULT", true, "CREATE INDEX `idx` ON `t` (`a`)"},
		{"CREATE INDEX idx ON t ( a ) ALGORITHM = INPLACE LOCK = EXCLUSIVE", true, "CREATE INDEX `idx` ON `t` (`a`) ALGORITHM = INPLACE LOCK = EXCLUSIVE"},
		{"CREATE INDEX idx ON t ( a ) LOCK = EXCLUSIVE ALGORITHM = INPLACE", true, "CREATE INDEX `idx` ON `t` (`a`) ALGORITHM = INPLACE LOCK = EXCLUSIVE"},
		{"CREATE INDEX idx ON t ( a ) ALGORITHM = ident", false, ""},
		{"CREATE INDEX idx ON t ( a ) ALGORITHM ident", false, ""},

		// For dorp index statement
		{"drop index a on t", true, "DROP INDEX `a` ON `t`"},
		{"drop index a on db.t", true, "DROP INDEX `a` ON `db`.`t`"},
		{"drop index a on db.`tb-ttb`", true, "DROP INDEX `a` ON `db`.`tb-ttb`"},
		{"drop index if exists a on t", true, "DROP INDEX IF EXISTS `a` ON `t`"},
		{"drop index if exists a on db.t", true, "DROP INDEX IF EXISTS `a` ON `db`.`t`"},
		{"drop index if exists a on db.`tb-ttb`", true, "DROP INDEX IF EXISTS `a` ON `db`.`tb-ttb`"},
		{"drop index idx on t algorithm = default", true, "DROP INDEX `idx` ON `t`"},
		{"drop index idx on t algorithm default", true, "DROP INDEX `idx` ON `t`"},
		{"drop index idx on t algorithm = inplace", true, "DROP INDEX `idx` ON `t` ALGORITHM = INPLACE"},
		{"drop index idx on t algorithm inplace", true, "DROP INDEX `idx` ON `t` ALGORITHM = INPLACE"},
		{"drop index idx on t lock = default", true, "DROP INDEX `idx` ON `t`"},
		{"drop index idx on t lock default", true, "DROP INDEX `idx` ON `t`"},
		{"drop index idx on t lock = shared", true, "DROP INDEX `idx` ON `t` LOCK = SHARED"},
		{"drop index idx on t lock shared", true, "DROP INDEX `idx` ON `t` LOCK = SHARED"},
		{"drop index idx on t algorithm = default lock = default", true, "DROP INDEX `idx` ON `t`"},
		{"drop index idx on t lock = default algorithm = default", true, "DROP INDEX `idx` ON `t`"},
		{"drop index idx on t algorithm = inplace lock = exclusive", true, "DROP INDEX `idx` ON `t` ALGORITHM = INPLACE LOCK = EXCLUSIVE"},
		{"drop index idx on t lock = exclusive algorithm = inplace", true, "DROP INDEX `idx` ON `t` ALGORITHM = INPLACE LOCK = EXCLUSIVE"},
		{"drop index idx on t algorithm = algorithm_type", false, ""},
		{"drop index idx on t algorithm algorithm_type", false, ""},
		{"drop index idx on t lock = lock_type", false, ""},
		{"drop index idx on t lock lock_type", false, ""},

		// for rename table statement
		{"RENAME TABLE t TO t1", true, "RENAME TABLE `t` TO `t1`"},
		{"RENAME TABLE t t1", false, "RENAME TABLE `t` TO `t1`"},
		{"RENAME TABLE d.t TO d1.t1", true, "RENAME TABLE `d`.`t` TO `d1`.`t1`"},
		{"RENAME TABLE t1 TO t2, t3 TO t4", true, "RENAME TABLE `t1` TO `t2`, `t3` TO `t4`"},

		// for truncate statement
		{"TRUNCATE TABLE t1", true, "TRUNCATE TABLE `t1`"},
		{"TRUNCATE t1", true, "TRUNCATE TABLE `t1`"},

		// for empty alert table index
		{"ALTER TABLE t ADD INDEX () ", false, ""},
		{"ALTER TABLE t ADD UNIQUE ()", false, ""},
		{"ALTER TABLE t ADD UNIQUE INDEX ()", false, ""},
		{"ALTER TABLE t ADD UNIQUE KEY ()", false, ""},

		// for keyword `SECONDARY_LOAD`, `SECONDARY_UNLOAD`
		{"ALTER TABLE d_n.t_n SECONDARY_LOAD", true, "ALTER TABLE `d_n`.`t_n` SECONDARY_LOAD"},
		{"ALTER TABLE d_n.t_n SECONDARY_UNLOAD", true, "ALTER TABLE `d_n`.`t_n` SECONDARY_UNLOAD"},
		{"ALTER TABLE t_n LOCK = DEFAULT , SECONDARY_LOAD", true, "ALTER TABLE `t_n` LOCK = DEFAULT, SECONDARY_LOAD"},
		{"ALTER TABLE d_n.t_n ALGORITHM = DEFAULT , SECONDARY_LOAD", true, "ALTER TABLE `d_n`.`t_n` ALGORITHM = DEFAULT, SECONDARY_LOAD"},
		{"ALTER TABLE d_n.t_n ALGORITHM = DEFAULT , SECONDARY_UNLOAD", true, "ALTER TABLE `d_n`.`t_n` ALGORITHM = DEFAULT, SECONDARY_UNLOAD"},

		// for issue 4538
		{"create table a (process double)", true, "CREATE TABLE `a` (`process` DOUBLE)"},

		// for issue 4740
		{"create table t (a int1, b int2, c int3, d int4, e int8)", true, "CREATE TABLE `t` (`a` TINYINT,`b` SMALLINT,`c` MEDIUMINT,`d` INT,`e` BIGINT)"},

		// for issue 5918
		{"create table t (lv long varchar null)", true, "CREATE TABLE `t` (`lv` MEDIUMTEXT NULL)"},

		// special table name
		{"CREATE TABLE cdp_test.`test2-1` (id int(11) DEFAULT NULL,key(id));", true, "CREATE TABLE `cdp_test`.`test2-1` (`id` INT(11) DEFAULT NULL,INDEX(`id`))"},
		{"CREATE TABLE miantiao (`扁豆焖面`       INT(11));", true, "CREATE TABLE `miantiao` (`扁豆焖面` INT(11))"},

		// for create table select
		{"CREATE TABLE bar (m INT)  SELECT n FROM foo;", true, "CREATE TABLE `bar` (`m` INT) AS SELECT `n` FROM `foo`"},
		{"CREATE TABLE bar (m INT) IGNORE SELECT n FROM foo;", true, "CREATE TABLE `bar` (`m` INT) IGNORE AS SELECT `n` FROM `foo`"},
		{"CREATE TABLE bar (m INT) REPLACE SELECT n FROM foo;", true, "CREATE TABLE `bar` (`m` INT) REPLACE AS SELECT `n` FROM `foo`"},

		// for generated column definition
		{"create table t (a timestamp, b timestamp as (a) not null on update current_timestamp);", false, ""},
		{"create table t (a bigint, b bigint as (a) primary key auto_increment);", false, ""},
		{"create table t (a bigint, b bigint as (a) not null default 10);", false, ""},
		{"create table t (a bigint, b bigint as (a+1) not null);", true, "CREATE TABLE `t` (`a` BIGINT,`b` BIGINT GENERATED ALWAYS AS(`a`+1) VIRTUAL NOT NULL)"},
		{"create table t (a bigint, b bigint as (a+1) not null);", true, "CREATE TABLE `t` (`a` BIGINT,`b` BIGINT GENERATED ALWAYS AS(`a`+1) VIRTUAL NOT NULL)"},
		{"create table t (a bigint, b bigint as (a+1) not null comment 'ttt');", true, "CREATE TABLE `t` (`a` BIGINT,`b` BIGINT GENERATED ALWAYS AS(`a`+1) VIRTUAL NOT NULL COMMENT 'ttt')"},
		{"create table t(a int, index idx((cast(a as binary(1)))));", true, "CREATE TABLE `t` (`a` INT,INDEX `idx`((CAST(`a` AS BINARY(1)))))"},
		{"alter table t add column (f timestamp as (a+1) default '2019-01-01 11:11:11');", false, ""},
		{"alter table t modify column f int as (a+1) default 55;", false, ""},

		// for column format
		{"create table t (a int column_format fixed)", true, "CREATE TABLE `t` (`a` INT COLUMN_FORMAT FIXED)"},
		{"create table t (a int column_format default)", true, "CREATE TABLE `t` (`a` INT COLUMN_FORMAT DEFAULT)"},
		{"create table t (a int column_format dynamic)", true, "CREATE TABLE `t` (`a` INT COLUMN_FORMAT DYNAMIC)"},
		{"alter table t modify column a bigint column_format default", true, "ALTER TABLE `t` MODIFY COLUMN `a` BIGINT COLUMN_FORMAT DEFAULT"},

		// for recover table
		{"recover table by job 11", true, "RECOVER TABLE BY JOB 11"},
		{"recover table by job 11,12,13", false, ""},
		{"recover table by job", false, ""},
		{"recover table by job 0", true, "RECOVER TABLE BY JOB 0"},
		{"recover table t1", true, "RECOVER TABLE `t1`"},
		{"recover table t1,t2", false, ""},
		{"recover table ", false, ""},
		{"recover table t1 100", true, "RECOVER TABLE `t1` 100"},
		{"recover table t1 abc", false, ""},

		// for flashback table.
		{"flashback table t", true, "FLASHBACK TABLE `t`"},
		{"flashback table t TO t1", true, "FLASHBACK TABLE `t` TO `t1`"},
		{"flashback table t TO timestamp", true, "FLASHBACK TABLE `t` TO `timestamp`"},

		// for flashback database.
		{"flashback database db1", true, "FLASHBACK DATABASE `db1`"},
		{"flashback schema db1", true, "FLASHBACK DATABASE `db1`"},
		{"flashback database db1 to db2", true, "FLASHBACK DATABASE `db1` TO `db2`"},
		{"flashback schema db1 to db2", true, "FLASHBACK DATABASE `db1` TO `db2`"},

		// for flashback to timestamp
		{"flashback cluster to timestamp '2021-05-26 16:45:26'", true, "FLASHBACK CLUSTER TO TIMESTAMP '2021-05-26 16:45:26'"},
		{"flashback table t to timestamp '2021-05-26 16:45:26'", true, "FLASHBACK TABLE `t` TO TIMESTAMP '2021-05-26 16:45:26'"},
		{"flashback table t,t1 to timestamp '2021-05-26 16:45:26'", true, "FLASHBACK TABLE `t`, `t1` TO TIMESTAMP '2021-05-26 16:45:26'"},
		{"flashback database test to timestamp '2021-05-26 16:45:26'", true, "FLASHBACK DATABASE `test` TO TIMESTAMP '2021-05-26 16:45:26'"},
		{"flashback schema test to timestamp '2021-05-26 16:45:26'", true, "FLASHBACK DATABASE `test` TO TIMESTAMP '2021-05-26 16:45:26'"},
		{"flashback cluster to timestamp TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())", false, ""},
		{"flashback cluster to timestamp DATE_SUB(NOW(), INTERVAL 3 SECOND)", false, ""},
		{"flashback table to timestamp '2021-05-26 16:45:26'", false, ""},
		{"flashback database to timestamp '2021-05-26 16:45:26'", false, ""},

		// for flashback to tso
		{"flashback cluster to tso 445494955052105721", true, "FLASHBACK CLUSTER TO TSO 445494955052105721"},
		{"flashback table t to tso 445494955052105722", true, "FLASHBACK TABLE `t` TO TSO 445494955052105722"},
		{"flashback table t,t1 to tso 445494955052105723", true, "FLASHBACK TABLE `t`, `t1` TO TSO 445494955052105723"},
		{"flashback database test to tso 445494955052105724", true, "FLASHBACK DATABASE `test` TO TSO 445494955052105724"},
		{"flashback schema test to tso 445494955052105725", true, "FLASHBACK DATABASE `test` TO TSO 445494955052105725"},
		{"flashback table to tso 445494955052105726", false, ""},
		{"flashback database to tso 445494955052105727", false, ""},
		{"flashback schema test to tso 0", false, ""},
		{"flashback schema test to tso -100", false, ""},

		// for remove partitioning
		{"alter table t remove partitioning", true, "ALTER TABLE `t` REMOVE PARTITIONING"},
		{"alter table db.ident remove partitioning", true, "ALTER TABLE `db`.`ident` REMOVE PARTITIONING"},
		{"alter table t lock = default remove partitioning", true, "ALTER TABLE `t` LOCK = DEFAULT REMOVE PARTITIONING"},
		{"alter table t add column a int remove partitioning", true, "ALTER TABLE `t` ADD COLUMN `a` INT REMOVE PARTITIONING"},
		{"alter table t add column a int, add index (c) remove partitioning", true, "ALTER TABLE `t` ADD COLUMN `a` INT, ADD INDEX(`c`) REMOVE PARTITIONING"},
		{"alter table t add column a int, remove partitioning", false, ""},
		{"alter table t add column a int, add index (c), remove partitioning", false, ""},
		{"alter table t remove partitioning add column a int", false, ""},
		{"alter table t remove partitioning, add column a int", false, ""},

		// for references without IndexColNameList
		{"alter table t add column a double (4,2) zerofill references b match full on update set null first", true, "ALTER TABLE `t` ADD COLUMN `a` DOUBLE(4,2) UNSIGNED ZEROFILL REFERENCES `b` MATCH FULL ON UPDATE SET NULL FIRST"},
		{"alter table d_n.t_n add constraint foreign key ident (ident(1)) references d_n.t_n match full on delete set null", true, "ALTER TABLE `d_n`.`t_n` ADD CONSTRAINT `ident` FOREIGN KEY (`ident`(1)) REFERENCES `d_n`.`t_n` MATCH FULL ON DELETE SET NULL"},
		{"alter table t_n add constraint ident foreign key (ident,ident(1)) references t_n match full on update set null on delete restrict", true, "ALTER TABLE `t_n` ADD CONSTRAINT `ident` FOREIGN KEY (`ident`, `ident`(1)) REFERENCES `t_n` MATCH FULL ON DELETE RESTRICT ON UPDATE SET NULL"},
		{"alter table d_n.t_n add foreign key ident (ident, ident(1) asc) references t_n match partial on delete cascade remove partitioning", true, "ALTER TABLE `d_n`.`t_n` ADD CONSTRAINT `ident` FOREIGN KEY (`ident`, `ident`(1)) REFERENCES `t_n` MATCH PARTIAL ON DELETE CASCADE REMOVE PARTITIONING"},
		{"alter table d_n.t_n add constraint foreign key (ident asc) references d_n.t_n match simple on update cascade on delete cascade", true, "ALTER TABLE `d_n`.`t_n` ADD CONSTRAINT FOREIGN KEY (`ident`) REFERENCES `d_n`.`t_n` MATCH SIMPLE ON DELETE CASCADE ON UPDATE CASCADE"},

		// for character vary syntax
		{"create table t (a character varying(1));", true, "CREATE TABLE `t` (`a` VARCHAR(1))"},
		{"create table t (a character varying(255));", true, "CREATE TABLE `t` (`a` VARCHAR(255))"},
		{"create table t (a char varying(50));", true, "CREATE TABLE `t` (`a` VARCHAR(50))"},
		{"create table t (a varcharacter(1));", true, "CREATE TABLE `t` (`a` VARCHAR(1))"},
		{"create table t (a varcharacter(50));", true, "CREATE TABLE `t` (`a` VARCHAR(50))"},
		{"create table t (a varcharacter(1), b varcharacter(255));", true, "CREATE TABLE `t` (`a` VARCHAR(1),`b` VARCHAR(255))"},
		{"create table t (a char);", true, "CREATE TABLE `t` (`a` CHAR)"},
		{"create table t (a character);", true, "CREATE TABLE `t` (`a` CHAR)"},
		{"create table t (a character varying(50), b int);", true, "CREATE TABLE `t` (`a` VARCHAR(50),`b` INT)"},
		{"create table t (a character, b int);", true, "CREATE TABLE `t` (`a` CHAR,`b` INT)"},
		{"create table t (a national character varying(50));", true, "CREATE TABLE `t` (`a` VARCHAR(50))"},
		{"create table t (a national char varying(50));", true, "CREATE TABLE `t` (`a` VARCHAR(50))"},
		{"create table t (a national char);", true, "CREATE TABLE `t` (`a` CHAR)"},
		{"create table t (a national character);", true, "CREATE TABLE `t` (`a` CHAR)"},
		{"create table t (a nchar);", true, "CREATE TABLE `t` (`a` CHAR)"},
		{"create table t (a nchar varchar(50));", true, "CREATE TABLE `t` (`a` VARCHAR(50))"},
		{"create table t (a nchar varcharacter(50));", true, "CREATE TABLE `t` (`a` VARCHAR(50))"},
		{"create table t (a national varchar);", false, ""},
		{"create table t (a national varchar(50));", true, "CREATE TABLE `t` (`a` VARCHAR(50))"},
		{"create table t (a national varcharacter(50));", true, "CREATE TABLE `t` (`a` VARCHAR(50))"},
		{"create table t (a nchar varying(50));", true, "CREATE TABLE `t` (`a` VARCHAR(50))"},
		{"create table t (a nvarchar(50));", true, "CREATE TABLE `t` (`a` VARCHAR(50))"},
		{"create table nchar (a int);", true, "CREATE TABLE `nchar` (`a` INT)"},
		{"create table nchar (a int, b nchar);", true, "CREATE TABLE `nchar` (`a` INT,`b` CHAR)"},
		{"create table nchar (a int, b nchar(50));", true, "CREATE TABLE `nchar` (`a` INT,`b` CHAR(50))"},
		{"alter table t_n storage disk , modify ident national varcharacter(12) column_format fixed first;", true, "ALTER TABLE `t_n` STORAGE DISK, MODIFY COLUMN `ident` VARCHAR(12) COLUMN_FORMAT FIXED FIRST"},

		// Test keyword `SERIAL`
		{"create table t (a serial);", true, "CREATE TABLE `t` (`a` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT UNIQUE KEY)"},
		{"create table t (a serial null);", true, "CREATE TABLE `t` (`a` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT UNIQUE KEY NULL)"},
		{"create table t (b int, a serial);", true, "CREATE TABLE `t` (`b` INT,`a` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT UNIQUE KEY)"},
		{"create table t (a int serial default value);", true, "CREATE TABLE `t` (`a` INT NOT NULL AUTO_INCREMENT UNIQUE KEY)"},
		{"create table t (a int serial default value null);", true, "CREATE TABLE `t` (`a` INT NOT NULL AUTO_INCREMENT UNIQUE KEY NULL)"},
		{"create table t (a bigint serial default value);", true, "CREATE TABLE `t` (`a` BIGINT NOT NULL AUTO_INCREMENT UNIQUE KEY)"},
		{"create table t (a smallint serial default value);", true, "CREATE TABLE `t` (`a` SMALLINT NOT NULL AUTO_INCREMENT UNIQUE KEY)"},

		// for LONG syntax
		{"create table t (a long);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT)"},
		{"create table t (a long varchar);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT)"},
		{"create table t (a long varcharacter);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT)"},
		{"create table t (a long char varying);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT)"},
		{"create table t (a long character varying);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT)"},
		{"create table t (a mediumtext, b long varchar, c long, d long varcharacter, e long char varying, f long character varying, g long);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT,`b` MEDIUMTEXT,`c` MEDIUMTEXT,`d` MEDIUMTEXT,`e` MEDIUMTEXT,`f` MEDIUMTEXT,`g` MEDIUMTEXT)"},
		{"create table t (a long varbinary);", true, "CREATE TABLE `t` (`a` MEDIUMBLOB)"},
		{"create table t (a long char varying, b long varbinary);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT,`b` MEDIUMBLOB)"},
		{"create table t (a long char set utf8);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT CHARACTER SET UTF8)"},
		{"create table t (a long char varying char set utf8);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT CHARACTER SET UTF8)"},
		{"create table t (a long character set utf8);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT CHARACTER SET UTF8)"},
		{"create table t (a long character varying character set utf8);", true, "CREATE TABLE `t` (`a` MEDIUMTEXT CHARACTER SET UTF8)"},
		{"alter table d_n.t_n modify column ident long after ident remove partitioning", true, "ALTER TABLE `d_n`.`t_n` MODIFY COLUMN `ident` MEDIUMTEXT AFTER `ident` REMOVE PARTITIONING"},
		{"alter table d_n.t_n modify column ident long char varying after ident remove partitioning", true, "ALTER TABLE `d_n`.`t_n` MODIFY COLUMN `ident` MEDIUMTEXT AFTER `ident` REMOVE PARTITIONING"},
		{"alter table d_n.t_n modify column ident long character varying after ident remove partitioning", true, "ALTER TABLE `d_n`.`t_n` MODIFY COLUMN `ident` MEDIUMTEXT AFTER `ident` REMOVE PARTITIONING"},
		{"alter table d_n.t_n modify column ident long varchar after ident remove partitioning", true, "ALTER TABLE `d_n`.`t_n` MODIFY COLUMN `ident` MEDIUMTEXT AFTER `ident` REMOVE PARTITIONING"},
		{"alter table d_n.t_n modify column ident long varcharacter after ident remove partitioning", true, "ALTER TABLE `d_n`.`t_n` MODIFY COLUMN `ident` MEDIUMTEXT AFTER `ident` REMOVE PARTITIONING"},
		{"alter table t_n change column ident ident long char varying binary charset utf8 first , tablespace ident", true, "ALTER TABLE `t_n` CHANGE COLUMN `ident` `ident` MEDIUMTEXT BINARY CHARACTER SET UTF8 FIRST, TABLESPACE = `ident`"},
		{"alter table t_n change column ident ident long character varying binary charset utf8 first , tablespace ident", true, "ALTER TABLE `t_n` CHANGE COLUMN `ident` `ident` MEDIUMTEXT BINARY CHARACTER SET UTF8 FIRST, TABLESPACE = `ident`"},

		// for STATS_AUTO_RECALC syntax
		{"create table t (a int) stats_auto_recalc 2;", false, ""},
		{"create table t (a int) stats_auto_recalc = 10;", false, ""},
		{"create table t (a int) stats_auto_recalc 0;", true, "CREATE TABLE `t` (`a` INT) STATS_AUTO_RECALC = 0"},
		{"create table t (a int) stats_auto_recalc default;", true, "CREATE TABLE `t` (`a` INT) STATS_AUTO_RECALC = DEFAULT"},
		{"create table t (a int) stats_auto_recalc = 0;", true, "CREATE TABLE `t` (`a` INT) STATS_AUTO_RECALC = 0"},
		{"create table t (a int) stats_auto_recalc = 1;", true, "CREATE TABLE `t` (`a` INT) STATS_AUTO_RECALC = 1"},
		{"create table t (a int) stats_auto_recalc=default;", true, "CREATE TABLE `t` (`a` INT) STATS_AUTO_RECALC = DEFAULT"},
		{"create table t (a int) stats_persistent = 1, stats_auto_recalc = 1;", true, "CREATE TABLE `t` (`a` INT) STATS_PERSISTENT = DEFAULT /* TableOptionStatsPersistent is not supported */  STATS_AUTO_RECALC = 1"},
		{"create table t (a int) stats_auto_recalc = 1, stats_sample_pages = 25;", true, "CREATE TABLE `t` (`a` INT) STATS_AUTO_RECALC = 1 STATS_SAMPLE_PAGES = 25"},
		{"alter table t modify a bigint, ENGINE=InnoDB, stats_auto_recalc = 0", true, "ALTER TABLE `t` MODIFY COLUMN `a` BIGINT, ENGINE = InnoDB, STATS_AUTO_RECALC = 0"},
		{"create table stats_auto_recalc (a int);", true, "CREATE TABLE `stats_auto_recalc` (`a` INT)"},
		{"create table stats_auto_recalc (a int) stats_auto_recalc=1;", true, "CREATE TABLE `stats_auto_recalc` (`a` INT) STATS_AUTO_RECALC = 1"},

		// for TYPE/USING syntax
		{"create table t (a int, primary key type type btree (a));", true, "CREATE TABLE `t` (`a` INT,PRIMARY KEY `type`(`a`) USING BTREE)"},
		{"create table t (a int, primary key type btree (a));", false, ""},
		{"create table t (a int, primary key using btree (a));", true, "CREATE TABLE `t` (`a` INT,PRIMARY KEY(`a`) USING BTREE)"},
		{"create table t (a int, primary key (a) type btree);", true, "CREATE TABLE `t` (`a` INT,PRIMARY KEY(`a`) USING BTREE)"},
		{"create table t (a int, primary key (a) using btree);", true, "CREATE TABLE `t` (`a` INT,PRIMARY KEY(`a`) USING BTREE)"},
		{"create table t (a int, unique index type type btree (a));", true, "CREATE TABLE `t` (`a` INT,UNIQUE `type`(`a`) USING BTREE)"},
		{"create table t (a int, unique index type using btree (a));", true, "CREATE TABLE `t` (`a` INT,UNIQUE `type`(`a`) USING BTREE)"},
		{"create table t (a int, unique index type btree (a));", false, ""},
		{"create table t (a int, unique index using btree (a));", true, "CREATE TABLE `t` (`a` INT,UNIQUE(`a`) USING BTREE)"},
		{"create table t (a int, unique index (a) using btree);", true, "CREATE TABLE `t` (`a` INT,UNIQUE(`a`) USING BTREE)"},
		{"create table t (a int, unique key (a) using btree);", true, "CREATE TABLE `t` (`a` INT,UNIQUE(`a`) USING BTREE)"},
		{"create table t (a int, index type type btree (a));", true, "CREATE TABLE `t` (`a` INT,INDEX `type`(`a`) USING BTREE)"},
		{"create table t (a int, index type btree (a));", false, ""},
		{"create table t (a int, index type using btree (a));", true, "CREATE TABLE `t` (`a` INT,INDEX `type`(`a`) USING BTREE)"},
		{"create table t (a int, index using btree (a));", true, "CREATE TABLE `t` (`a` INT,INDEX(`a`) USING BTREE)"},

		// for issue 500
		{`ALTER TABLE d_n.t_n WITHOUT VALIDATION , ADD PARTITION ( PARTITION ident VALUES LESS THAN ( MAXVALUE ) STORAGE ENGINE text_string MAX_ROWS 12 )`, true, "ALTER TABLE `d_n`.`t_n` WITHOUT VALIDATION, ADD PARTITION (PARTITION `ident` VALUES LESS THAN (MAXVALUE) ENGINE = text_string MAX_ROWS = 12)"},
		{`ALTER TABLE d_n.t_n WITH VALIDATION , ADD PARTITION NO_WRITE_TO_BINLOG (PARTITION ident VALUES LESS THAN MAXVALUE STORAGE ENGINE = text_string, PARTITION ident VALUES LESS THAN ( MAXVALUE ) (SUBPARTITION text_string MIN_ROWS 11))`, true, "ALTER TABLE `d_n`.`t_n` WITH VALIDATION, ADD PARTITION NO_WRITE_TO_BINLOG (PARTITION `ident` VALUES LESS THAN (MAXVALUE) ENGINE = text_string, PARTITION `ident` VALUES LESS THAN (MAXVALUE) (SUBPARTITION `text_string` MIN_ROWS = 11))"},
		// for test VALUE IN
		{`ALTER TABLE d_n.t_n WITHOUT VALIDATION , ADD PARTITION ( PARTITION ident VALUES IN ( DEFAULT ) STORAGE ENGINE text_string MAX_ROWS 12 )`, true, "ALTER TABLE `d_n`.`t_n` WITHOUT VALIDATION, ADD PARTITION (PARTITION `ident` DEFAULT ENGINE = text_string MAX_ROWS = 12)"},
		{`ALTER TABLE d_n.t_n WITH VALIDATION , ADD PARTITION NO_WRITE_TO_BINLOG ( PARTITION ident VALUES IN ( DEFAULT ) STORAGE ENGINE text_string MAX_ROWS 12 )`, true, "ALTER TABLE `d_n`.`t_n` WITH VALIDATION, ADD PARTITION NO_WRITE_TO_BINLOG (PARTITION `ident` DEFAULT ENGINE = text_string MAX_ROWS = 12)"},
		{`ALTER TABLE d_n.t_n ADD PARTITION ( PARTITION ident VALUES IN ( DEFAULT ), partition ptext values in ('default') )`, true, "ALTER TABLE `d_n`.`t_n` ADD PARTITION (PARTITION `ident` DEFAULT, PARTITION `ptext` VALUES IN (_UTF8MB4'default'))"},
		{`ALTER TABLE d_n.t_n WITH VALIDATION , ADD PARTITION NO_WRITE_TO_BINLOG (PARTITION ident VALUES LESS THAN MAXVALUE STORAGE ENGINE = text_string, PARTITION ident VALUES IN ( DEFAULT ) (SUBPARTITION text_string MIN_ROWS 11))`, true, "ALTER TABLE `d_n`.`t_n` WITH VALIDATION, ADD PARTITION NO_WRITE_TO_BINLOG (PARTITION `ident` VALUES LESS THAN (MAXVALUE) ENGINE = text_string, PARTITION `ident` DEFAULT (SUBPARTITION `text_string` MIN_ROWS = 11))"},
		{`ALTER TABLE d_n.t_n ADD PARTITION (PARTITION ident VALUES IN ( DEFAULT ))`, true, "ALTER TABLE `d_n`.`t_n` ADD PARTITION (PARTITION `ident` DEFAULT)"},
		{`ALTER TABLE d_n.t_n ADD PARTITION (PARTITION ident VALUES IN (1, default ))`, true, "ALTER TABLE `d_n`.`t_n` ADD PARTITION (PARTITION `ident` VALUES IN (1, DEFAULT))"},
		// for issue 501
		{"ALTER TABLE t IMPORT TABLESPACE;", true, "ALTER TABLE `t` IMPORT TABLESPACE"},
		{"ALTER TABLE t DISCARD TABLESPACE;", true, "ALTER TABLE `t` DISCARD TABLESPACE"},
		{"ALTER TABLE db.t IMPORT TABLESPACE;", true, "ALTER TABLE `db`.`t` IMPORT TABLESPACE"},
		{"ALTER TABLE db.t DISCARD TABLESPACE;", true, "ALTER TABLE `db`.`t` DISCARD TABLESPACE"},

		// for CONSTRAINT syntax, see issue 413
		{"ALTER TABLE t ADD ( CHECK ( true ) )", true, "ALTER TABLE `t` ADD COLUMN (CHECK(TRUE) ENFORCED)"},
		{"ALTER TABLE t ADD ( CONSTRAINT CHECK ( true ) )", true, "ALTER TABLE `t` ADD COLUMN (CHECK(TRUE) ENFORCED)"},
		{"ALTER TABLE t ADD COLUMN ( CONSTRAINT ident CHECK ( 1>2 ) NOT ENFORCED )", true, "ALTER TABLE `t` ADD COLUMN (CONSTRAINT `ident` CHECK(1>2) NOT ENFORCED)"},
		{"alter table t add column (b int, constraint c unique key (b))", true, "ALTER TABLE `t` ADD COLUMN (`b` INT, UNIQUE `c`(`b`))"},
		{"ALTER TABLE t ADD COLUMN ( CONSTRAINT CHECK ( true ) )", true, "ALTER TABLE `t` ADD COLUMN (CHECK(TRUE) ENFORCED)"},
		{"ALTER TABLE t ADD COLUMN ( CONSTRAINT CHECK ( true ) ENFORCED , CHECK ( true ) )", true, "ALTER TABLE `t` ADD COLUMN (CHECK(TRUE) ENFORCED, CHECK(TRUE) ENFORCED)"},
		{"ALTER TABLE t ADD COLUMN (a1 int, CONSTRAINT b1 CHECK (a1>0))", true, "ALTER TABLE `t` ADD COLUMN (`a1` INT, CONSTRAINT `b1` CHECK(`a1`>0) ENFORCED)"},
		{"ALTER TABLE t ADD COLUMN (a1 int, a2 int, CONSTRAINT b1 CHECK (a1>0), CONSTRAINT b2 CHECK (a2<10))", true, "ALTER TABLE `t` ADD COLUMN (`a1` INT, `a2` INT, CONSTRAINT `b1` CHECK(`a1`>0) ENFORCED, CONSTRAINT `b2` CHECK(`a2`<10) ENFORCED)"},
		{"ALTER TABLE `t` ADD COLUMN (`a1` INT, PRIMARY KEY (`a1`))", true, "ALTER TABLE `t` ADD COLUMN (`a1` INT, PRIMARY KEY(`a1`))"},
		{"ALTER TABLE t ADD (a1 int, CONSTRAINT PRIMARY KEY (a1))", true, "ALTER TABLE `t` ADD COLUMN (`a1` INT, PRIMARY KEY(`a1`))"},
		{"ALTER TABLE t ADD (a1 int, a2 int, PRIMARY KEY (a1), UNIQUE (a2))", true, "ALTER TABLE `t` ADD COLUMN (`a1` INT, `a2` INT, PRIMARY KEY(`a1`), UNIQUE(`a2`))"},
		{"ALTER TABLE t ADD (a1 int, a2 int, PRIMARY KEY (a1), CONSTRAINT b2 UNIQUE (a2))", true, "ALTER TABLE `t` ADD COLUMN (`a1` INT, `a2` INT, PRIMARY KEY(`a1`), UNIQUE `b2`(`a2`))"},
		{"ALTER TABLE ident ADD ( CONSTRAINT FOREIGN KEY ident ( EXECUTE ( 123 ) ) REFERENCES t ( a ) MATCH SIMPLE ON DELETE CASCADE ON UPDATE SET NULL )", true, "ALTER TABLE `ident` ADD COLUMN (CONSTRAINT `ident` FOREIGN KEY (`EXECUTE`(123)) REFERENCES `t`(`a`) MATCH SIMPLE ON DELETE CASCADE ON UPDATE SET NULL)"},
		// for CONSTRAINT cont'd, the following tests are for another aspect of the incompatibility
		{"ALTER TABLE t ADD COLUMN a DATE CHECK ( a > 0 ) FIRST", true, "ALTER TABLE `t` ADD COLUMN `a` DATE CHECK(`a`>0) ENFORCED FIRST"},
		{"ALTER TABLE t ADD a1 int CONSTRAINT ident CHECK ( a1 > 1 ) REFERENCES b ON DELETE CASCADE ON UPDATE CASCADE;", true, "ALTER TABLE `t` ADD COLUMN `a1` INT CONSTRAINT `ident` CHECK(`a1`>1) ENFORCED REFERENCES `b` ON DELETE CASCADE ON UPDATE CASCADE"},
		{"ALTER TABLE t ADD COLUMN a DATE CONSTRAINT CHECK ( a > 0 ) FIRST", true, "ALTER TABLE `t` ADD COLUMN `a` DATE CHECK(`a`>0) ENFORCED FIRST"},
		{"ALTER TABLE t ADD a TINYBLOB CONSTRAINT ident CHECK ( 1>2 ) REFERENCES b ON DELETE CASCADE ON UPDATE CASCADE", true, "ALTER TABLE `t` ADD COLUMN `a` TINYBLOB CONSTRAINT `ident` CHECK(1>2) ENFORCED REFERENCES `b` ON DELETE CASCADE ON UPDATE CASCADE"},
		{"ALTER TABLE t ADD a2 int CONSTRAINT ident CHECK (a2 > 1) ENFORCED", true, "ALTER TABLE `t` ADD COLUMN `a2` INT CONSTRAINT `ident` CHECK(`a2`>1) ENFORCED"},
		{"ALTER TABLE t ADD a2 int CONSTRAINT ident CHECK (a2 > 1) NOT ENFORCED", true, "ALTER TABLE `t` ADD COLUMN `a2` INT CONSTRAINT `ident` CHECK(`a2`>1) NOT ENFORCED"},
		{"ALTER TABLE t ADD a2 int CONSTRAINT ident primary key REFERENCES b ON DELETE CASCADE ON UPDATE CASCADE;", false, ""},
		{"ALTER TABLE t ADD a2 int CONSTRAINT ident primary key (a2))", false, ""},
		{"ALTER TABLE t ADD a2 int CONSTRAINT ident unique key (a2))", false, ""},

		{"ALTER TABLE t SET TIFLASH REPLICA 2 LOCATION LABELS 'a','b'", true, "ALTER TABLE `t` SET TIFLASH REPLICA 2 LOCATION LABELS 'a', 'b'"},
		{"ALTER TABLE t SET TIFLASH REPLICA 0", true, "ALTER TABLE `t` SET TIFLASH REPLICA 0"},
		{"ALTER DATABASE t SET TIFLASH REPLICA 2 LOCATION LABELS 'a','b'", true, "ALTER DATABASE `t` SET TIFLASH REPLICA 2 LOCATION LABELS 'a', 'b'"},
		{"ALTER DATABASE t SET TIFLASH REPLICA 0", true, "ALTER DATABASE `t` SET TIFLASH REPLICA 0"},
		{"ALTER DATABASE t SET TIFLASH REPLICA 1 SET TIFLASH REPLICA 2 LOCATION LABELS 'a','b'", true, "ALTER DATABASE `t` SET TIFLASH REPLICA 1 SET TIFLASH REPLICA 2 LOCATION LABELS 'a', 'b'"},
		{"ALTER DATABASE t SET TIFLASH REPLICA 1 SET TIFLASH REPLICA 2", true, "ALTER DATABASE `t` SET TIFLASH REPLICA 1 SET TIFLASH REPLICA 2"},
		{"ALTER DATABASE t SET TIFLASH REPLICA 1 LOCATION LABELS 'a','b' SET TIFLASH REPLICA 2", true, "ALTER DATABASE `t` SET TIFLASH REPLICA 1 LOCATION LABELS 'a', 'b' SET TIFLASH REPLICA 2"},
		{"ALTER DATABASE t SET TIFLASH REPLICA 1 LOCATION LABELS 'a','b' SET TIFLASH REPLICA 2 LOCATION LABELS 'a', 'b'", true, "ALTER DATABASE `t` SET TIFLASH REPLICA 1 LOCATION LABELS 'a', 'b' SET TIFLASH REPLICA 2 LOCATION LABELS 'a', 'b'"},

		// for issue 537
		{"CREATE TABLE IF NOT EXISTS table_ident (a SQL_TSI_YEAR(4), b SQL_TSI_YEAR);", true, "CREATE TABLE IF NOT EXISTS `table_ident` (`a` YEAR(4),`b` YEAR)"},
		{`CREATE TABLE IF NOT EXISTS table_ident (ident1 BOOL COMMENT "text_string" unique, ident2 SQL_TSI_YEAR(4) ZEROFILL);`, true, "CREATE TABLE IF NOT EXISTS `table_ident` (`ident1` TINYINT(1) COMMENT 'text_string' UNIQUE KEY,`ident2` YEAR(4))"},
		{"create table t (y sql_tsi_year(4), y1 sql_tsi_year)", true, "CREATE TABLE `t` (`y` YEAR(4),`y1` YEAR)"},
		{"create table t (y sql_tsi_year(4) unsigned zerofill zerofill, y1 sql_tsi_year signed unsigned zerofill)", true, "CREATE TABLE `t` (`y` YEAR(4),`y1` YEAR)"},

		// for issue 549
		{"insert into t set a = default", true, "INSERT INTO `t` SET `a`=DEFAULT"},
		{"insert into t set a := default", true, "INSERT INTO `t` SET `a`=DEFAULT"},
		{"replace t set a = default", true, "REPLACE INTO `t` SET `a`=DEFAULT"},
		{"update t set a = default", true, "UPDATE `t` SET `a`=DEFAULT"},
		{"insert into t set a = default on duplicate key update a = default", true, "INSERT INTO `t` SET `a`=DEFAULT ON DUPLICATE KEY UPDATE `a`=DEFAULT"},

		// for issue 529
		{"create table t (a text byte ascii)", false, ""},
		{"create table t (a text byte charset latin1)", false, ""},
		{"create table t (a longtext ascii)", true, "CREATE TABLE `t` (`a` LONGTEXT CHARACTER SET LATIN1)"},
		{"create table t (a mediumtext ascii)", true, "CREATE TABLE `t` (`a` MEDIUMTEXT CHARACTER SET LATIN1)"},
		{"create table t (a tinytext ascii)", true, "CREATE TABLE `t` (`a` TINYTEXT CHARACTER SET LATIN1)"},
		{"create table t (a text byte)", true, "CREATE TABLE `t` (`a` BLOB)"},
		{"create table t (a long byte, b text ascii)", true, "CREATE TABLE `t` (`a` MEDIUMBLOB,`b` TEXT CHARACTER SET LATIN1)"},
		{"create table t (a text ascii, b mediumtext ascii, c int)", true, "CREATE TABLE `t` (`a` TEXT CHARACTER SET LATIN1,`b` MEDIUMTEXT CHARACTER SET LATIN1,`c` INT)"},
		{"create table t (a int, b text ascii, c mediumtext ascii)", true, "CREATE TABLE `t` (`a` INT,`b` TEXT CHARACTER SET LATIN1,`c` MEDIUMTEXT CHARACTER SET LATIN1)"},
		{"create table t (a long ascii, b long ascii)", true, "CREATE TABLE `t` (`a` MEDIUMTEXT CHARACTER SET LATIN1,`b` MEDIUMTEXT CHARACTER SET LATIN1)"},
		{"create table t (a long character set utf8mb4, b long charset utf8mb4, c long char set utf8mb4)", true, "CREATE TABLE `t` (`a` MEDIUMTEXT CHARACTER SET UTF8MB4,`b` MEDIUMTEXT CHARACTER SET UTF8MB4,`c` MEDIUMTEXT CHARACTER SET UTF8MB4)"},

		{"create table t (a int STORAGE MEMORY, b varchar(255) STORAGE MEMORY)", true, "CREATE TABLE `t` (`a` INT STORAGE MEMORY,`b` VARCHAR(255) STORAGE MEMORY)"},
		{"create table t (a int storage DISK, b varchar(255) STORAGE DEFAULT)", true, "CREATE TABLE `t` (`a` INT STORAGE DISK,`b` VARCHAR(255) STORAGE DEFAULT)"},
		{"create table t (a int STORAGE DEFAULT, b varchar(255) STORAGE DISK)", true, "CREATE TABLE `t` (`a` INT STORAGE DEFAULT,`b` VARCHAR(255) STORAGE DISK)"},

		// for issue 555
		{"create table t (a fixed(6, 3), b fixed key)", true, "CREATE TABLE `t` (`a` DECIMAL(6,3),`b` DECIMAL PRIMARY KEY)"},
		{"create table t (a numeric, b fixed(6))", true, "CREATE TABLE `t` (`a` DECIMAL,`b` DECIMAL(6))"},
		{"create table t (a fixed(65, 30) zerofill, b numeric, c fixed(65) unsigned zerofill)", true, "CREATE TABLE `t` (`a` DECIMAL(65,30) UNSIGNED ZEROFILL,`b` DECIMAL,`c` DECIMAL(65) UNSIGNED ZEROFILL)"},

		// create table with expression index
		{"create table a(a int, key(lower(a)));", false, ""},
		{"create table a(a int, key(a+1));", false, ""},
		{"create table a(a int, key(a, a+1));", false, ""},
		{"create table a(a int, b int, key((a+1), (b+1)));", true, "CREATE TABLE `a` (`a` INT,`b` INT,INDEX((`a`+1), (`b`+1)))"},
		{"create table a(a int, b int, key(a, (b+1)));", true, "CREATE TABLE `a` (`a` INT,`b` INT,INDEX(`a`, (`b`+1)))"},
		{"create table a(a int, b int, key((a+1), b));", true, "CREATE TABLE `a` (`a` INT,`b` INT,INDEX((`a`+1), `b`))"},
		{"create table a(a int, b int, key((a + 1) desc));", true, "CREATE TABLE `a` (`a` INT,`b` INT,INDEX((`a`+1) DESC))"},

		// for create sequence
		{"create sequence sequence", true, "CREATE SEQUENCE `sequence`"},
		{"create sequence seq", true, "CREATE SEQUENCE `seq`"},
		{"create sequence if not exists seq", true, "CREATE SEQUENCE IF NOT EXISTS `seq`"},
		{"create sequence seq", true, "CREATE SEQUENCE `seq`"},
		{"create sequence seq", true, "CREATE SEQUENCE `seq`"},
		{"create sequence if not exists seq", true, "CREATE SEQUENCE IF NOT EXISTS `seq`"},
		{"create sequence if not exists seq", true, "CREATE SEQUENCE IF NOT EXISTS `seq`"},
		{"create sequence if not exists seq increment", false, ""},
		{"create sequence if not exists seq increment 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` INCREMENT BY 1"},
		{"create sequence if not exists seq increment = 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` INCREMENT BY 1"},
		{"create sequence if not exists seq increment by 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` INCREMENT BY 1"},
		{"create sequence if not exists seq minvalue", false, ""},
		{"create sequence if not exists seq minvalue 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` MINVALUE 1"},
		{"create sequence if not exists seq minvalue = 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` MINVALUE 1"},
		{"create sequence if not exists seq no", false, ""},
		{"create sequence if not exists seq nominvalue", true, "CREATE SEQUENCE IF NOT EXISTS `seq` NO MINVALUE"},
		{"create sequence if not exists seq no minvalue", true, "CREATE SEQUENCE IF NOT EXISTS `seq` NO MINVALUE"},
		{"create sequence if not exists seq maxvalue", false, ""},
		{"create sequence if not exists seq maxvalue 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` MAXVALUE 1"},
		{"create sequence if not exists seq maxvalue = 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` MAXVALUE 1"},
		{"create sequence if not exists seq no", false, ""},
		{"create sequence if not exists seq nomaxvalue", true, "CREATE SEQUENCE IF NOT EXISTS `seq` NO MAXVALUE"},
		{"create sequence if not exists seq no maxvalue", true, "CREATE SEQUENCE IF NOT EXISTS `seq` NO MAXVALUE"},
		{"create sequence if not exists seq start", false, ""},
		{"create sequence if not exists seq start with", false, ""},
		{"create sequence if not exists seq start =", false, ""},
		{"create sequence if not exists seq start with", false, ""},
		{"create sequence if not exists seq start 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` START WITH 1"},
		{"create sequence if not exists seq start = 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` START WITH 1"},
		{"create sequence if not exists seq start with 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` START WITH 1"},
		{"create sequence if not exists seq cache", false, ""},
		{"create sequence if not exists seq cache 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` CACHE 1"},
		{"create sequence if not exists seq cache = 1", true, "CREATE SEQUENCE IF NOT EXISTS `seq` CACHE 1"},
		{"create sequence if not exists seq nocache", true, "CREATE SEQUENCE IF NOT EXISTS `seq` NOCACHE"},
		{"create sequence if not exists seq no cache", true, "CREATE SEQUENCE IF NOT EXISTS `seq` NOCACHE"},
		{"create sequence if not exists seq cycle", true, "CREATE SEQUENCE IF NOT EXISTS `seq` CYCLE"},
		{"create sequence if not exists seq nocycle", true, "CREATE SEQUENCE IF NOT EXISTS `seq` NOCYCLE"},
		{"create sequence if not exists seq no cycle", true, "CREATE SEQUENCE IF NOT EXISTS `seq` NOCYCLE"},
		{"create sequence seq increment 1 start with 0 minvalue 0 maxvalue 1000", true, "CREATE SEQUENCE `seq` INCREMENT BY 1 START WITH 0 MINVALUE 0 MAXVALUE 1000"},
		{"create sequence seq increment 1 start with 0 minvalue 0 maxvalue 1000", true, "CREATE SEQUENCE `seq` INCREMENT BY 1 START WITH 0 MINVALUE 0 MAXVALUE 1000"},
		// TODO : support or replace if need : care for it will conflict on temporary.
		{"create sequence seq increment 10 start with 0 minvalue 0 maxvalue 1000", true, "CREATE SEQUENCE `seq` INCREMENT BY 10 START WITH 0 MINVALUE 0 MAXVALUE 1000"},
		{"create sequence if not exists seq cache 1 increment 1 start with -1 minvalue 0 maxvalue 1000", true, "CREATE SEQUENCE IF NOT EXISTS `seq` CACHE 1 INCREMENT BY 1 START WITH -1 MINVALUE 0 MAXVALUE 1000"},
		{"create sequence sEq start with 0 minvalue 0 maxvalue 1000", true, "CREATE SEQUENCE `sEq` START WITH 0 MINVALUE 0 MAXVALUE 1000"},
		{"create sequence if not exists seq increment 1 start with 0 minvalue -2 maxvalue 1000", true, "CREATE SEQUENCE IF NOT EXISTS `seq` INCREMENT BY 1 START WITH 0 MINVALUE -2 MAXVALUE 1000"},
		{"create sequence seq increment -1 start with -1 minvalue -1 maxvalue -1000 cache = 10 nocycle", true, "CREATE SEQUENCE `seq` INCREMENT BY -1 START WITH -1 MINVALUE -1 MAXVALUE -1000 CACHE 10 NOCYCLE"},

		// test sequence is not a reserved keyword
		{"create table sequence (a int)", true, "CREATE TABLE `sequence` (`a` INT)"},
		{"create table t (sequence int)", true, "CREATE TABLE `t` (`sequence` INT)"},

		// test drop sequence
		{"drop sequence", false, ""},
		{"drop sequence seq", true, "DROP SEQUENCE `seq`"},
		{"drop sequence if exists seq", true, "DROP SEQUENCE IF EXISTS `seq`"},
		{"drop sequence seq", true, "DROP SEQUENCE `seq`"},
		{"drop sequence if exists seq", true, "DROP SEQUENCE IF EXISTS `seq`"},
		{"drop sequence if exists seq, seq2, seq3", true, "DROP SEQUENCE IF EXISTS `seq`, `seq2`, `seq3`"},
		{"drop sequence seq seq2", false, ""},
		{"drop sequence seq, seq2", true, "DROP SEQUENCE `seq`, `seq2`"},

		// for auto_random
		{"create table t (a bigint auto_random(3) primary key, b varchar(255))", true, "CREATE TABLE `t` (`a` BIGINT AUTO_RANDOM(3) PRIMARY KEY,`b` VARCHAR(255))"},
		{"create table t (a bigint auto_random primary key, b varchar(255))", true, "CREATE TABLE `t` (`a` BIGINT AUTO_RANDOM PRIMARY KEY,`b` VARCHAR(255))"},
		{"create table t (a bigint primary key auto_random(4), b varchar(255))", true, "CREATE TABLE `t` (`a` BIGINT PRIMARY KEY AUTO_RANDOM(4),`b` VARCHAR(255))"},
		{"create table t (a bigint primary key auto_random(3) primary key unique, b varchar(255))", true, "CREATE TABLE `t` (`a` BIGINT PRIMARY KEY AUTO_RANDOM(3) PRIMARY KEY UNIQUE KEY,`b` VARCHAR(255))"},
		{"create table t (a bigint auto_random(5, 53) primary key, b varchar(255))", true, "CREATE TABLE `t` (`a` BIGINT AUTO_RANDOM(5, 53) PRIMARY KEY,`b` VARCHAR(255))"},
		{"create table t (a bigint auto_random(15, 32) primary key, b varchar(255))", true, "CREATE TABLE `t` (`a` BIGINT AUTO_RANDOM(15, 32) PRIMARY KEY,`b` VARCHAR(255))"},

		// for auto_id_cache
		{"create table t (a int) auto_id_cache=1", true, "CREATE TABLE `t` (`a` INT) AUTO_ID_CACHE = 1"},
		{"create table t (a int auto_increment key) auto_id_cache 10", true, "CREATE TABLE `t` (`a` INT AUTO_INCREMENT PRIMARY KEY) AUTO_ID_CACHE = 10"},
		{"create table t (a bigint, b varchar(255)) auto_id_cache 50", true, "CREATE TABLE `t` (`a` BIGINT,`b` VARCHAR(255)) AUTO_ID_CACHE = 50"},

		// for auto_random_id
		{"create table t (a bigint auto_random(3) primary key) auto_random_base = 10", true, "CREATE TABLE `t` (`a` BIGINT AUTO_RANDOM(3) PRIMARY KEY) AUTO_RANDOM_BASE = 10"},
		{"create table t (a bigint primary key auto_random(4), b varchar(100)) auto_random_base 200", true, "CREATE TABLE `t` (`a` BIGINT PRIMARY KEY AUTO_RANDOM(4),`b` VARCHAR(100)) AUTO_RANDOM_BASE = 200"},
		{"alter table t auto_random_base = 50", true, "ALTER TABLE `t` AUTO_RANDOM_BASE = 50"},
		{"alter table t auto_increment 30, auto_random_base 40", true, "ALTER TABLE `t` AUTO_INCREMENT = 30, AUTO_RANDOM_BASE = 40"},
		{"alter table t force auto_random_base = 50", true, "ALTER TABLE `t` FORCE AUTO_RANDOM_BASE = 50"},
		{"alter table t auto_increment 30, force auto_random_base 40", true, "ALTER TABLE `t` AUTO_INCREMENT = 30, FORCE AUTO_RANDOM_BASE = 40"},

		// for alter sequence
		{"alter sequence seq", false, ""},
		{"alter sequence seq comment=\"haha\"", false, ""},
		{"alter sequence seq start = 1", true, "ALTER SEQUENCE `seq` START WITH 1"},
		{"alter sequence seq start with 1 increment by 1", true, "ALTER SEQUENCE `seq` START WITH 1 INCREMENT BY 1"},
		{"alter sequence seq start with 1 increment by 2 minvalue 0 maxvalue 100", true, "ALTER SEQUENCE `seq` START WITH 1 INCREMENT BY 2 MINVALUE 0 MAXVALUE 100"},
		{"alter sequence seq increment -1 start with -1 minvalue -1 maxvalue -1000 cache = 10 nocycle", true, "ALTER SEQUENCE `seq` INCREMENT BY -1 START WITH -1 MINVALUE -1 MAXVALUE -1000 CACHE 10 NOCYCLE"},
		{"alter sequence if exists seq2 increment = 2", true, "ALTER SEQUENCE IF EXISTS `seq2` INCREMENT BY 2"},
		{"alter sequence seq restart", true, "ALTER SEQUENCE `seq` RESTART"},
		{"alter sequence seq start with 3 restart with 5", true, "ALTER SEQUENCE `seq` START WITH 3 RESTART WITH 5"},
		{"alter sequence seq restart = 5", true, "ALTER SEQUENCE `seq` RESTART WITH 5"},
		{"create sequence seq restart = 5", false, ""},

		// for issue 18149
		{"create table t (a int, index ``(a))", true, "CREATE TABLE `t` (`a` INT,INDEX ``(`a`))"},

		// for clustered index
		{"create table t (a int, b varchar(255), primary key(b, a) clustered)", true, "CREATE TABLE `t` (`a` INT,`b` VARCHAR(255),PRIMARY KEY(`b`, `a`) CLUSTERED)"},
		{"create table t (a int, b varchar(255), primary key(b, a) nonclustered)", true, "CREATE TABLE `t` (`a` INT,`b` VARCHAR(255),PRIMARY KEY(`b`, `a`) NONCLUSTERED)"},
		{"create table t (a int primary key nonclustered, b varchar(255))", true, "CREATE TABLE `t` (`a` INT PRIMARY KEY NONCLUSTERED,`b` VARCHAR(255))"},
		{"create table t (a int, b varchar(255) primary key clustered)", true, "CREATE TABLE `t` (`a` INT,`b` VARCHAR(255) PRIMARY KEY CLUSTERED)"},
		{"create table t (a int, b varchar(255) default 'a' primary key clustered)", true, "CREATE TABLE `t` (`a` INT,`b` VARCHAR(255) DEFAULT _UTF8MB4'a' PRIMARY KEY CLUSTERED)"},
		{"create table t (a int, b varchar(255) primary key nonclustered, primary key(b, a) nonclustered)", true, "CREATE TABLE `t` (`a` INT,`b` VARCHAR(255) PRIMARY KEY NONCLUSTERED,PRIMARY KEY(`b`, `a`) NONCLUSTERED)"},
		{"create table t (a int, b varchar(255), primary key(b, a) using RTREE nonclustered)", true, "CREATE TABLE `t` (`a` INT,`b` VARCHAR(255),PRIMARY KEY(`b`, `a`) NONCLUSTERED USING RTREE)"},
		{"create table t (a int, b varchar(255), primary key(b, a) using RTREE clustered nonclustered)", true, "CREATE TABLE `t` (`a` INT,`b` VARCHAR(255),PRIMARY KEY(`b`, `a`) NONCLUSTERED USING RTREE)"},
		{"create table t (a int, b varchar(255), primary key(b, a) using RTREE nonclustered clustered)", true, "CREATE TABLE `t` (`a` INT,`b` VARCHAR(255),PRIMARY KEY(`b`, `a`) CLUSTERED USING RTREE)"},
		{"create table t (a int, b varchar(255) clustered primary key)", false, ""},
		{"create table t (a int, b varchar(255) primary key nonclustered clustered)", false, ""},
		{"alter table t add primary key (`a`, `b`) clustered", true, "ALTER TABLE `t` ADD PRIMARY KEY(`a`, `b`) CLUSTERED"},
		{"alter table t add primary key (`a`, `b`) nonclustered", true, "ALTER TABLE `t` ADD PRIMARY KEY(`a`, `b`) NONCLUSTERED"},

		// for create table with vector index
		{"create table t(a int, b vector(3), vector index(b) USING HNSW);", true, "CREATE TABLE `t` (`a` INT,`b` VECTOR(3),VECTOR INDEX(`b`) USING HNSW)"},
		{"create table t(a int, b vector(3), vector index(a, b) USING HNSW);", true, "CREATE TABLE `t` (`a` INT,`b` VECTOR(3),VECTOR INDEX(`a`, `b`) USING HNSW)"},
		{"create table t(a int, b vector(3), vector index((VEC_COSINE_DISTANCE(b))));", true, "CREATE TABLE `t` (`a` INT,`b` VECTOR(3),VECTOR INDEX((VEC_COSINE_DISTANCE(`b`))))"},
		{"create table t(a int, b vector(3), vector index((VEC_COSINE_DISTANCE(b))) USING HASH);", true, "CREATE TABLE `t` (`a` INT,`b` VECTOR(3),VECTOR INDEX((VEC_COSINE_DISTANCE(`b`))) USING HASH)"},
		{"create table t(a int, b vector(3), vector index(a, (VEC_COSINE_DISTANCE(b))) USING HNSW);", true, "CREATE TABLE `t` (`a` INT,`b` VECTOR(3),VECTOR INDEX(`a`, (VEC_COSINE_DISTANCE(`b`))) USING HNSW)"},
		{"create table t(a int, b vector(3), vector index((VEC_COSINE_DISTANCE(b)), a) USING HNSW);", true, "CREATE TABLE `t` (`a` INT,`b` VECTOR(3),VECTOR INDEX((VEC_COSINE_DISTANCE(`b`)), `a`) USING HNSW)"},
		{"create table t(a int, b vector(3), vector index(VEC_COSINE_DISTANCE(b)) USING HNSW);", false, ""},
		{"create table t(a int, b vector(3), vector key((VEC_COSINE_DISTANCE(b))) TYPE HNSW);", false, ""},
		{"create table t(a int, b vector(3), vector index((b+1)) USING HNSW);", true, "CREATE TABLE `t` (`a` INT,`b` VECTOR(3),VECTOR INDEX((`b`+1)) USING HNSW)"},
		{"create table t(a int, b vector(3), vector index((VEC_COSINE_DISTANCE(a, b))) USING HNSW);", true, "CREATE TABLE `t` (`a` INT,`b` VECTOR(3),VECTOR INDEX((VEC_COSINE_DISTANCE(`a`, `b`))) USING HNSW)"},
		{"create table t(a int, b vector(3), vector index((VEC_COSINE_DISTANCE(b))) USING HNSW);", true, "CREATE TABLE `t` (`a` INT,`b` VECTOR(3),VECTOR INDEX((VEC_COSINE_DISTANCE(`b`))) USING HNSW)"},

		// for drop placement policy
		{"drop placement policy x", true, "DROP PLACEMENT POLICY `x`"},
		{"drop placement policy x, y", false, ""},
		{"drop placement policy if exists x", true, "DROP PLACEMENT POLICY IF EXISTS `x`"},
		{"drop placement policy if exists x, y", false, ""},
		// for show create placement policy
		{"show create placement policy x", true, "SHOW CREATE PLACEMENT POLICY `x`"},
		{"show create placement policy if exists x", false, ""},
		{"show create placement policy x, y", false, ""},
		{"show create placement policy `placement`", true, "SHOW CREATE PLACEMENT POLICY `placement`"},
		// for create placement policy
		{"create placement policy x primary_region='us'", true, "CREATE PLACEMENT POLICY `x` PRIMARY_REGION = 'us'"},
		{"create placement policy x region='us, 3'", false, ""},
		{"create placement policy x followers=3", true, "CREATE PLACEMENT POLICY `x` FOLLOWERS = 3"},
		{"create placement policy x followers=0", false, ""},
		{"create placement policy x voters=3", true, "CREATE PLACEMENT POLICY `x` VOTERS = 3"},
		{"create placement policy x learners=3", true, "CREATE PLACEMENT POLICY `x` LEARNERS = 3"},
		{"create placement policy x schedule='even'", true, "CREATE PLACEMENT POLICY `x` SCHEDULE = 'even'"},
		{"create placement policy x constraints='ww'", true, "CREATE PLACEMENT POLICY `x` CONSTRAINTS = 'ww'"},
		{"create placement policy x leader_constraints='ww'", true, "CREATE PLACEMENT POLICY `x` LEADER_CONSTRAINTS = 'ww'"},
		{"create placement policy x follower_constraints='ww'", true, "CREATE PLACEMENT POLICY `x` FOLLOWER_CONSTRAINTS = 'ww'"},
		{"create placement policy x voter_constraints='ww'", true, "CREATE PLACEMENT POLICY `x` VOTER_CONSTRAINTS = 'ww'"},
		{"create placement policy x learner_constraints='ww'", true, "CREATE PLACEMENT POLICY `x` LEARNER_CONSTRAINTS = 'ww'"},
		{"create placement policy x primary_region='cn' regions='us' schedule='even'", true, "CREATE PLACEMENT POLICY `x` PRIMARY_REGION = 'cn' REGIONS = 'us' SCHEDULE = 'even'"},
		{"create placement policy x primary_region='cn', leader_constraints='ww', leader_constraints='yy'", true, "CREATE PLACEMENT POLICY `x` PRIMARY_REGION = 'cn' LEADER_CONSTRAINTS = 'ww' LEADER_CONSTRAINTS = 'yy'"},
		{"create placement policy if not exists x regions = 'us', follower_constraints='yy'", true, "CREATE PLACEMENT POLICY IF NOT EXISTS `x` REGIONS = 'us' FOLLOWER_CONSTRAINTS = 'yy'"},
		{"create or replace placement policy x regions='us'", true, "CREATE OR REPLACE PLACEMENT POLICY `x` REGIONS = 'us'"},
		{"create placement policy x placement policy y", false, ""},

		{"alter placement policy x primary_region='us'", true, "ALTER PLACEMENT POLICY `x` PRIMARY_REGION = 'us'"},
		{"alter placement policy x region='us, 3'", false, ""},
		{"alter placement policy x followers=3", true, "ALTER PLACEMENT POLICY `x` FOLLOWERS = 3"},
		{"alter placement policy x voters=3", true, "ALTER PLACEMENT POLICY `x` VOTERS = 3"},
		{"alter placement policy x learners=3", true, "ALTER PLACEMENT POLICY `x` LEARNERS = 3"},
		{"alter placement policy x schedule='even'", true, "ALTER PLACEMENT POLICY `x` SCHEDULE = 'even'"},
		{"alter placement policy x constraints='ww'", true, "ALTER PLACEMENT POLICY `x` CONSTRAINTS = 'ww'"},
		{"alter placement policy x leader_constraints='ww'", true, "ALTER PLACEMENT POLICY `x` LEADER_CONSTRAINTS = 'ww'"},
		{"alter placement policy x follower_constraints='ww'", true, "ALTER PLACEMENT POLICY `x` FOLLOWER_CONSTRAINTS = 'ww'"},
		{"alter placement policy x voter_constraints='ww'", true, "ALTER PLACEMENT POLICY `x` VOTER_CONSTRAINTS = 'ww'"},
		{"alter placement policy x learner_constraints='ww'", true, "ALTER PLACEMENT POLICY `x` LEARNER_CONSTRAINTS = 'ww'"},
		{"alter placement policy x primary_region='cn' regions='us' schedule='even'", true, "ALTER PLACEMENT POLICY `x` PRIMARY_REGION = 'cn' REGIONS = 'us' SCHEDULE = 'even'"},
		{"alter placement policy x primary_region='cn', leader_constraints='ww', leader_constraints='yy'", true, "ALTER PLACEMENT POLICY `x` PRIMARY_REGION = 'cn' LEADER_CONSTRAINTS = 'ww' LEADER_CONSTRAINTS = 'yy'"},
		{"alter placement policy if exists x regions = 'us', follower_constraints='yy'", true, "ALTER PLACEMENT POLICY IF EXISTS `x` REGIONS = 'us' FOLLOWER_CONSTRAINTS = 'yy'"},
		{"alter placement policy x placement policy y", false, ""},

		// for create resource group
		{"create resource group x cpu ='8c'", false, ""},
		{"create resource group x region ='us, 3'", false, ""},
		{"create resource group x cpu='8c', io_read_bandwidth='2GB/s', io_write_bandwidth='200MB/s'", false, ""},
		{"create resource group x ru_per_sec=2000", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 2000"},
		{"create resource group x ru_per_sec=200000", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 200000"},
		{"create resource group x ru_per_sec=UNLIMITED", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED"},
		{"create resource group x ru_per_sec=unlimited", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED"},
		{"create resource group x ru_per_sec='check'", false, ""},
		{"create resource group x followers=0", false, ""},
		{"create resource group x burstable=true", false, ""},
		{"create resource group x burstable=false", false, ""},
		{"create resource group x burstable=disable", false, ""},
		{"create resource group x ru_per_sec=1000, burstable", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BURSTABLE = MODERATED"},
		{"create resource group x burstable, ru_per_sec=2000", true, "CREATE RESOURCE GROUP `x` BURSTABLE = MODERATED, RU_PER_SEC = 2000"},
		{"create resource group x ru_per_sec=3000 burstable", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 3000, BURSTABLE = MODERATED"},
		{"create resource group x burstable ru_per_sec=4000", true, "CREATE RESOURCE GROUP `x` BURSTABLE = MODERATED, RU_PER_SEC = 4000"},
		{"create resource group x BURSTABLE = UNLIMITED ru_per_sec=4000", true, "CREATE RESOURCE GROUP `x` BURSTABLE = UNLIMITED, RU_PER_SEC = 4000"},
		{"create resource group x BURSTABLE = MODERATED ru_per_sec=4000", true, "CREATE RESOURCE GROUP `x` BURSTABLE = MODERATED, RU_PER_SEC = 4000"},
		{"create resource group x BURSTABLE = OFF ru_per_sec=4000", true, "CREATE RESOURCE GROUP `x` BURSTABLE = OFF, RU_PER_SEC = 4000"},
		{"create resource group x ru_per_sec=20, priority=LOW, burstable", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 20, PRIORITY = LOW, BURSTABLE = MODERATED"},
		{"create resource group default ru_per_sec=20, priority=LOW, burstable", true, "CREATE RESOURCE GROUP `default` RU_PER_SEC = 20, PRIORITY = LOW, BURSTABLE = MODERATED"},
		{"create resource group default ru_per_sec=UNLIMITED, priority=LOW, burstable", true, "CREATE RESOURCE GROUP `default` RU_PER_SEC = UNLIMITED, PRIORITY = LOW, BURSTABLE = MODERATED"},
		{"create resource group x ru_per_sec=1000", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000"},
		{"create resource group x ru_per_sec=1000 burstable=unlimited", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BURSTABLE = UNLIMITED"},
		{"create resource group x ru_per_sec=1000 burstable=off", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BURSTABLE = OFF"},
		{"create resource group x ru_per_sec=1000 burstable=moderated", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BURSTABLE = MODERATED"},
		{"create resource group x burstable=unlimited, ru_per_sec=2000", true, "CREATE RESOURCE GROUP `x` BURSTABLE = UNLIMITED, RU_PER_SEC = 2000"},
		{"create resource group x burstable=off, ru_per_sec=2000", true, "CREATE RESOURCE GROUP `x` BURSTABLE = OFF, RU_PER_SEC = 2000"},
		{"create resource group x burstable=moderated, ru_per_sec=2000", true, "CREATE RESOURCE GROUP `x` BURSTABLE = MODERATED, RU_PER_SEC = 2000"},
		{"create resource group x ru_per_sec=1000 ,burstable=unlimited", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BURSTABLE = UNLIMITED"},
		{"create resource group x ru_per_sec=1000 ,burstable=off", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BURSTABLE = OFF"},
		{"create resource group x ru_per_sec=1000 ,burstable=moderated", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BURSTABLE = MODERATED"},
		{"create resource group x ru_per_sec=1000 , priority=LOW,burstable=unlimited", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, PRIORITY = LOW, BURSTABLE = UNLIMITED"},
		{"create resource group x ru_per_sec=1000 , priority=LOW,burstable=off", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, PRIORITY = LOW, BURSTABLE = OFF"},
		{"create resource group x ru_per_sec=1000 , priority=LOW,burstable=moderated", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, PRIORITY = LOW, BURSTABLE = MODERATED"},
		{"create resource group x ru_per_sec=UNLIMITED , priority=LOW,burstable=unlimited", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED, PRIORITY = LOW, BURSTABLE = UNLIMITED"},
		{"create resource group x ru_per_sec=UNLIMITED , priority=LOW,burstable=off", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED, PRIORITY = LOW, BURSTABLE = OFF"},
		{"create resource group x ru_per_sec=UNLIMITED , priority=LOW,burstable=moderated", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED, PRIORITY = LOW, BURSTABLE = MODERATED"},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10s' ACTION DRYRUN)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' ACTION = DRYRUN)"},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10m' ACTION COOLDOWN)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10m' ACTION = COOLDOWN)"},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT=(ACTION KILL EXEC_ELAPSED='10m')", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (ACTION = KILL EXEC_ELAPSED = '10m')"},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10s' WATCH=SIMILAR DURATION '10m' ACTION COOLDOWN)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' WATCH = SIMILAR DURATION = '10m' ACTION = COOLDOWN)"},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT (EXEC_ELAPSED \"10s\" ACTION COOLDOWN WATCH EXACT DURATION='10m')", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' ACTION = COOLDOWN WATCH = EXACT DURATION = '10m')"},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT (EXEC_ELAPSED '9s' ACTION COOLDOWN WATCH EXACT)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '9s' ACTION = COOLDOWN WATCH = EXACT DURATION = UNLIMITED)"},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT (EXEC_ELAPSED '8s' ACTION COOLDOWN WATCH EXACT DURATION = UNLIMITED)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '8s' ACTION = COOLDOWN WATCH = EXACT DURATION = UNLIMITED)"},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT (EXEC_ELAPSED '7s' ACTION COOLDOWN WATCH EXACT DURATION UNLIMITED)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '7s' ACTION = COOLDOWN WATCH = EXACT DURATION = UNLIMITED)"},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT (EXEC_ELAPSED '7s' ACTION COOLDOWN WATCH EXACT DURATION 'UNLIMITED')", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '7s' ACTION = COOLDOWN WATCH = EXACT DURATION = UNLIMITED)"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10s' RU 100 ACTION DRYRUN)", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' RU = 100 ACTION = DRYRUN)"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10s' PROCESSED_KEYS 100 ACTION DRYRUN)", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' PROCESSED_KEYS = 100 ACTION = DRYRUN)"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10s' WATCH SIMILAR DURATION '10m' ACTION COOLDOWN)", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' WATCH = SIMILAR DURATION = '10m' ACTION = COOLDOWN)"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10s' ACTION COOLDOWN WATCH EXACT DURATION '10m')", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' ACTION = COOLDOWN WATCH = EXACT DURATION = '10m')"},
		{"create resource group x ru_per_sec=1000 background = (task_types='')", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BACKGROUND = (TASK_TYPES = '')"},
		{"create resource group x ru_per_sec=1000 background = (UTILIZATION_LIMIT=50)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BACKGROUND = (UTILIZATION_LIMIT = 50)"},
		{"create resource group x ru_per_sec=1000 background = (UTILIZATION_LIMIT=\"NAN\")", false, ""},
		{"create resource group x ru_per_sec=1000 background (task_types='br,lightning')", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BACKGROUND = (TASK_TYPES = 'br,lightning')"},
		{"create resource group x ru_per_sec=1000 background (task_types='br,lightning',utilization_limit=50)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, BACKGROUND = (TASK_TYPES = 'br,lightning', UTILIZATION_LIMIT = 50)"},
		{`create resource group x ru_per_sec=1000 QUERY_LIMIT (EXEC_ELAPSED "10s" ACTION COOLDOWN WATCH EXACT DURATION='10m')  background (task_types 'br,lightning')`, true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' ACTION = COOLDOWN WATCH = EXACT DURATION = '10m'), BACKGROUND = (TASK_TYPES = 'br,lightning')"},
		{`create resource group x ru_per_sec=1000 QUERY_LIMIT (EXEC_ELAPSED "10s" ACTION COOLDOWN WATCH PLAN DURATION='10m')  background (task_types 'br,lightning')`, true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' ACTION = COOLDOWN WATCH = PLAN DURATION = '10m'), BACKGROUND = (TASK_TYPES = 'br,lightning')"},
		{`create resource group x ru_per_sec=1000 QUERY_LIMIT (EXEC_ELAPSED "10s" ACTION COOLDOWN WATCH PLAN DURATION='10m')  background (task_types 'br,lightning', utilization_limit 10)`, true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' ACTION = COOLDOWN WATCH = PLAN DURATION = '10m'), BACKGROUND = (TASK_TYPES = 'br,lightning', UTILIZATION_LIMIT = 10)"},
		{`create resource group x ru_per_sec=UNLIMITED QUERY_LIMIT (EXEC_ELAPSED "10s" ACTION COOLDOWN WATCH PLAN DURATION='10m')  background (task_types 'br,lightning')`, true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED, QUERY_LIMIT = (EXEC_ELAPSED = '10s' ACTION = COOLDOWN WATCH = PLAN DURATION = '10m'), BACKGROUND = (TASK_TYPES = 'br,lightning')"},
		// This case is expected in parser test but not in actual ddl job.
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (EXEC_ELAPSED '10s')", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s')"},
		{"create resource group x ru_per_sec=1000 QUERY=(EXEC_ELAPSED '10s')", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT=EXEC_ELAPSED '10s'", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (EXEC_ELAPSED '10s'", false, ""},
		{"create resource group x ru_per_sec=1000 LIMIT=(EXEC_ELAPSED '10s')", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (EXEC_ELAPSED '10s' ACTION DRYRUN ACTION KILL)", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (PROCESSED_KEYS=100)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (PROCESSED_KEYS = 100)"},
		{"create resource group x ru_per_sec=1000 QUERY=(PROCESSED_KEYS 100)", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT=PROCESSED_KEYS 100", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (PROCESSED_KEYS 100", false, ""},
		{"create resource group x ru_per_sec=1000 LIMIT=(PROCESSED_KEYS 100)", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (PROCESSED_KEYS 100 ACTION DRYRUN ACTION KILL)", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (RU=100)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (RU = 100)"},
		{"create resource group x ru_per_sec=1000 QUERY=(RU 100)", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT=RU 100", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (RU 100", false, ""},
		{"create resource group x ru_per_sec=1000 LIMIT=(RU 100)", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (RU 100 ACTION DRYRUN ACTION KILL)", false, ""},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (EXEC_ELAPSED='10s' PROCESSED_KEYS=100)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' PROCESSED_KEYS = 100)"},
		{"create resource group x ru_per_sec=1000 QUERY_LIMIT = (EXEC_ELAPSED='10s', PROCESSED_KEYS=100, RU=100)", true, "CREATE RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' PROCESSED_KEYS = 100 RU = 100)"},

		{"alter resource group x cpu ='8c'", false, ""},
		{"alter resource group x region ='us, 3'", false, ""},
		{"alter resource group x burstable=true", false, ""},
		{"alter resource group x burstable=false", false, ""},
		{"alter resource group x burstable=disable", false, ""},
		{"alter resource group default priority = high", true, "ALTER RESOURCE GROUP `default` PRIORITY = HIGH"},
		{"alter resource group x cpu='8c', io_read_bandwidth='2GB/s', io_write_bandwidth='200MB/s'", false, ""},
		{"alter resource group x ru_per_sec=1000", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000"},
		{"alter resource group x ru_per_sec=2000, BURSTABLE", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 2000, BURSTABLE = MODERATED"},
		{"alter resource group x ru_per_sec=UNLIMITED", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED"},
		{"alter resource group x ru_per_sec=UNLIMITED, BURSTABLE", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED, BURSTABLE = MODERATED"},
		{"alter resource group x ru_per_sec=unlimited, BURSTABLE", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED, BURSTABLE = MODERATED"},
		{"alter resource group x ru_per_sec='check', BURSTABLE", false, ""},
		{"alter resource group x BURSTABLE, ru_per_sec=3000", true, "ALTER RESOURCE GROUP `x` BURSTABLE = MODERATED, RU_PER_SEC = 3000"},
		{"alter resource group x BURSTABLE ru_per_sec=4000", true, "ALTER RESOURCE GROUP `x` BURSTABLE = MODERATED, RU_PER_SEC = 4000"},
		{"alter resource group x ru_per_sec=2000, BURSTABLE=unlimited", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 2000, BURSTABLE = UNLIMITED"},
		{"alter resource group x ru_per_sec=2000, BURSTABLE=moderated", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 2000, BURSTABLE = MODERATED"},
		{"alter resource group x ru_per_sec=2000, BURSTABLE=off", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 2000, BURSTABLE = OFF"},
		{"alter resource group x ru_per_sec=UNLIMITED", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED"},
		{"alter resource group x ru_per_sec=UNLIMITED, BURSTABLE=unlimited", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED, BURSTABLE = UNLIMITED"},
		{"alter resource group x ru_per_sec=UNLIMITED, BURSTABLE=moderated", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED, BURSTABLE = MODERATED"},
		{"alter resource group x ru_per_sec=unlimited, BURSTABLE=off", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED, BURSTABLE = OFF"},
		{"alter resource group x ru_per_sec='check', BURSTABLE", false, ""},
		{"alter resource group x ru_per_sec=2000, BURSTABLE=yes", false, ""},
		{"alter resource group x BURSTABLE=unlimited, ru_per_sec=3000", true, "ALTER RESOURCE GROUP `x` BURSTABLE = UNLIMITED, RU_PER_SEC = 3000"},
		{"alter resource group x BURSTABLE=moderated, ru_per_sec=3000", true, "ALTER RESOURCE GROUP `x` BURSTABLE = MODERATED, RU_PER_SEC = 3000"},
		{"alter resource group x BURSTABLE=off, ru_per_sec=3000", true, "ALTER RESOURCE GROUP `x` BURSTABLE = OFF, RU_PER_SEC = 3000"},
		{"alter resource group x BURSTABLE=unlimited ru_per_sec=4000", true, "ALTER RESOURCE GROUP `x` BURSTABLE = UNLIMITED, RU_PER_SEC = 4000"},
		{"alter resource group x BURSTABLE=moderated ru_per_sec=4000", true, "ALTER RESOURCE GROUP `x` BURSTABLE = MODERATED, RU_PER_SEC = 4000"},
		{"alter resource group x BURSTABLE=off ru_per_sec=4000", true, "ALTER RESOURCE GROUP `x` BURSTABLE = OFF, RU_PER_SEC = 4000"},
		// This case is expected in parser test but not in actual ddl job.
		// Todo: support patch setting(not cover all).
		{"alter resource group x BURSTABLE", true, "ALTER RESOURCE GROUP `x` BURSTABLE = MODERATED"},
		{"alter resource group x ru_per_sec=200000 BURSTABLE", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 200000, BURSTABLE = MODERATED"},
		{"alter resource group x followers=0", false, ""},
		{"alter resource group x ru_per_sec=20 priority=MID BURSTABLE", false, ""},
		{"alter resource group x BURSTABLE=unlimited", true, "ALTER RESOURCE GROUP `x` BURSTABLE = UNLIMITED"},
		{"alter resource group x BURSTABLE=moderated", true, "ALTER RESOURCE GROUP `x` BURSTABLE = MODERATED"},
		{"alter resource group x BURSTABLE=off", true, "ALTER RESOURCE GROUP `x` BURSTABLE = OFF"},
		{"alter resource group x ru_per_sec=200000 BURSTABLE=unlimited", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 200000, BURSTABLE = UNLIMITED"},
		{"alter resource group x ru_per_sec=200000 BURSTABLE=moderated", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 200000, BURSTABLE = MODERATED"},
		{"alter resource group x ru_per_sec=200000 BURSTABLE=off", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 200000, BURSTABLE = OFF"},
		{"alter resource group x followers=0", false, ""},
		{"alter resource group x ru_per_sec=20 priority=MID", false, ""},
		{"alter resource group x ru_per_sec=20 priority=HIGH BURSTABLE=unlimited", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 20, PRIORITY = HIGH, BURSTABLE = UNLIMITED"},
		{"alter resource group x ru_per_sec=20 priority=HIGH BURSTABLE=moderated", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 20, PRIORITY = HIGH, BURSTABLE = MODERATED"},
		{"alter resource group x ru_per_sec=20 priority=HIGH BURSTABLE=off", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 20, PRIORITY = HIGH, BURSTABLE = OFF"},

		{"alter resource group x QUERY_LIMIT=NULL", true, "ALTER RESOURCE GROUP `x` QUERY_LIMIT = NULL"},
		{"alter resource group x QUERY_LIMIT=()", true, "ALTER RESOURCE GROUP `x` QUERY_LIMIT = NULL"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10s' ACTION DRYRUN)", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' ACTION = DRYRUN)"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT=()", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = NULL"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10m' ACTION COOLDOWN)", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10m' ACTION = COOLDOWN)"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT=( ACTION KILL EXEC_ELAPSED '10m')", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (ACTION = KILL EXEC_ELAPSED = '10m')"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10s' WATCH SIMILAR DURATION '10m' ACTION COOLDOWN)", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' WATCH = SIMILAR DURATION = '10m' ACTION = COOLDOWN)"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT=(EXEC_ELAPSED '10s' ACTION COOLDOWN WATCH EXACT DURATION '10m')", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s' ACTION = COOLDOWN WATCH = EXACT DURATION = '10m')"},
		{"alter resource group x ru_per_sec=UNLIMITED QUERY_LIMIT=(EXEC_ELAPSED '10s' ACTION COOLDOWN WATCH EXACT DURATION '10m')", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = UNLIMITED, QUERY_LIMIT = (EXEC_ELAPSED = '10s' ACTION = COOLDOWN WATCH = EXACT DURATION = '10m')"},
		// This case is expected in parser test but not in actual ddl job.
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT = (EXEC_ELAPSED '10s')", true, "ALTER RESOURCE GROUP `x` RU_PER_SEC = 1000, QUERY_LIMIT = (EXEC_ELAPSED = '10s')"},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT EXEC_ELAPSED '10s'", false, ""},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT = (EXEC_ELAPSED '10s' ACTION DRYRUN ACTION KILL)", false, ""},
		{"alter resource group x ru_per_sec=1000 QUERY_LIMIT = (EXEC_ELAPSED '10s' ACTION DRYRUN WATCH SIMILAR DURATION '10m' ACTION COOLDOWN)", false, ""},
		{"alter resource group x background=()", true, "ALTER RESOURCE GROUP `x` BACKGROUND = NULL"},
		{"alter resource group x background NULL", true, "ALTER RESOURCE GROUP `x` BACKGROUND = NULL"},
		{"alter resource group default priority=low background = ( task_types \"ttl\" )", true, "ALTER RESOURCE GROUP `default` PRIORITY = LOW, BACKGROUND = (TASK_TYPES = 'ttl')"},
		{"alter resource group default burstable=unlimited background ( task_types = 'a,b,c' )", true, "ALTER RESOURCE GROUP `default` BURSTABLE = UNLIMITED, BACKGROUND = (TASK_TYPES = 'a,b,c')"},
		{"alter resource group default burstable=moderated background ( task_types = 'a,b,c' )", true, "ALTER RESOURCE GROUP `default` BURSTABLE = MODERATED, BACKGROUND = (TASK_TYPES = 'a,b,c')"},
		{"alter resource group default burstable=off background ( task_types = 'a,b,c' )", true, "ALTER RESOURCE GROUP `default` BURSTABLE = OFF, BACKGROUND = (TASK_TYPES = 'a,b,c')"},
		{"alter resource group default burstable=unlimited background ( utilization_limit = 20 )", true, "ALTER RESOURCE GROUP `default` BURSTABLE = UNLIMITED, BACKGROUND = (UTILIZATION_LIMIT = 20)"},
		{"alter resource group default burstable=moderated background ( utilization_limit = 20 )", true, "ALTER RESOURCE GROUP `default` BURSTABLE = MODERATED, BACKGROUND = (UTILIZATION_LIMIT = 20)"},
		{"alter resource group default burstable=off background ( utilization_limit = 20 )", true, "ALTER RESOURCE GROUP `default` BURSTABLE = OFF, BACKGROUND = (UTILIZATION_LIMIT = 20)"},
		{"alter resource group default burstable=unlimited background ( task_types = 'a,b,c', utilization_limit = 20 )", true, "ALTER RESOURCE GROUP `default` BURSTABLE = UNLIMITED, BACKGROUND = (TASK_TYPES = 'a,b,c', UTILIZATION_LIMIT = 20)"},
		{"alter resource group default burstable=moderated background ( task_types = 'a,b,c', utilization_limit = 20 )", true, "ALTER RESOURCE GROUP `default` BURSTABLE = MODERATED, BACKGROUND = (TASK_TYPES = 'a,b,c', UTILIZATION_LIMIT = 20)"},
		{"alter resource group default burstable=off background ( task_types = 'a,b,c', utilization_limit = 20 )", true, "ALTER RESOURCE GROUP `default` BURSTABLE = OFF, BACKGROUND = (TASK_TYPES = 'a,b,c', UTILIZATION_LIMIT = 20)"},
		{"alter resource group default burstable=unlimited background ( utilization_limit = 'abc' )", false, ""},
		{"alter resource group default burstable=moderated background ( utilization_limit = 'abc' )", false, ""},
		{"alter resource group default burstable=off background ( utilization_limit = 'abc' )", false, ""},

		{"drop resource group x;", true, "DROP RESOURCE GROUP `x`"},
		{"drop resource group DEFAULT;", true, "DROP RESOURCE GROUP `DEFAULT`"},
		{"drop resource group if exists x;", true, "DROP RESOURCE GROUP IF EXISTS `x`"},
		{"drop resource group x,y", false, ""},
		{"drop resource group if exists x,y", false, ""},

		{"set resource group x;", true, "SET RESOURCE GROUP `x`"},
		{"set resource group ``;", true, "SET RESOURCE GROUP ``"},
		{"set resource group `default`;", true, "SET RESOURCE GROUP `default`"},
		{"set resource group default;", true, "SET RESOURCE GROUP `default`"},
		{"set resource group x y", false, ""},

		{"CREATE ROLE `RESOURCE`", true, "CREATE ROLE `RESOURCE`@`%`"},
		{"CREATE ROLE RESOURCE", false, ""},

		// for table stats options
		// 1. create table with options
		{"CREATE TABLE t (a int) STATS_BUCKETS=1", true, "CREATE TABLE `t` (`a` INT) STATS_BUCKETS = 1"},
		{"CREATE TABLE t (a int) STATS_BUCKETS='abc'", false, ""},
		{"CREATE TABLE t (a int) STATS_BUCKETS=", false, ""},
		{"CREATE TABLE t (a int) STATS_TOPN=1", true, "CREATE TABLE `t` (`a` INT) STATS_TOPN = 1"},
		{"CREATE TABLE t (a int) STATS_TOPN='abc'", false, ""},
		{"CREATE TABLE t (a int) STATS_AUTO_RECALC=1", true, "CREATE TABLE `t` (`a` INT) STATS_AUTO_RECALC = 1"},
		{"CREATE TABLE t (a int) STATS_AUTO_RECALC='abc'", false, ""},
		{"CREATE TABLE t(a int) STATS_SAMPLE_RATE=0.1", true, "CREATE TABLE `t` (`a` INT) STATS_SAMPLE_RATE = 0.1"},
		{"CREATE TABLE t (a int) STATS_SAMPLE_RATE='abc'", false, ""},
		{"CREATE TABLE t (a int) STATS_COL_CHOICE='all'", true, "CREATE TABLE `t` (`a` INT) STATS_COL_CHOICE = 'all'"},
		{"CREATE TABLE t (a int) STATS_COL_CHOICE='list'", true, "CREATE TABLE `t` (`a` INT) STATS_COL_CHOICE = 'list'"},
		{"CREATE TABLE t (a int) STATS_COL_CHOICE=1", false, ""},
		{"CREATE TABLE t (a int, b int) STATS_COL_LIST='a,b'", true, "CREATE TABLE `t` (`a` INT,`b` INT) STATS_COL_LIST = 'a,b'"},
		{"CREATE TABLE t (a int, b int) STATS_COL_LIST=1", false, ""},
		{"CREATE TABLE t (a int) STATS_BUCKETS=1,STATS_TOPN=1", true, "CREATE TABLE `t` (`a` INT) STATS_BUCKETS = 1 STATS_TOPN = 1"},
		// 2. create partition table with options
		{"CREATE TABLE t (a int) STATS_BUCKETS=1,STATS_TOPN=1 PARTITION BY RANGE (a) (PARTITION p1 VALUES LESS THAN (200))", true, "CREATE TABLE `t` (`a` INT) STATS_BUCKETS = 1 STATS_TOPN = 1 PARTITION BY RANGE (`a`) (PARTITION `p1` VALUES LESS THAN (200))"},
		// 3. alter table with options
		{"ALTER TABLE t STATS_OPTIONS='str'", true, "ALTER TABLE `t` STATS_OPTIONS='str'"},
		{"ALTER TABLE t STATS_OPTIONS='str1,str2'", true, "ALTER TABLE `t` STATS_OPTIONS='str1,str2'"},
		{"ALTER TABLE t STATS_OPTIONS=\"str1,str2\"", true, "ALTER TABLE `t` STATS_OPTIONS='str1,str2'"},
		{"ALTER TABLE t STATS_OPTIONS 'str1,str2'", true, "ALTER TABLE `t` STATS_OPTIONS='str1,str2'"},
		{"ALTER TABLE t STATS_OPTIONS \"str1,str2\"", true, "ALTER TABLE `t` STATS_OPTIONS='str1,str2'"},
		{"ALTER TABLE t STATS_OPTIONS=DEFAULT", true, "ALTER TABLE `t` STATS_OPTIONS=DEFAULT"},
		{"ALTER TABLE t STATS_OPTIONS=default", true, "ALTER TABLE `t` STATS_OPTIONS=DEFAULT"},
		{"ALTER TABLE t STATS_OPTIONS=DeFaUlT", true, "ALTER TABLE `t` STATS_OPTIONS=DEFAULT"},
		{"ALTER TABLE t STATS_OPTIONS", false, ""},

		// Restore INSERT_METHOD table option
		{"CREATE TABLE t (a int) INSERT_METHOD=FIRST", true, "CREATE TABLE `t` (`a` INT) INSERT_METHOD = FIRST"},
	}
	RunTest(t, table, false)
}

func TestHintError(t *testing.T) {
	p := parser.New()
	stmt, warns, err := p.Parse("select /*+ tidb_unknown(T1,t2) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	require.Len(t, warns, 1)
	require.Equal(t, `[parser:8061]Optimizer hint tidb_unknown is not supported by TiDB and is ignored`, warns[0].Error())
	require.Len(t, stmt[0].(*ast.SelectStmt).TableHints, 0)
	stmt, warns, err = p.Parse("select /*+ TIDB_INLJ(t1, T2) tidb_unknown(T1,t2, 1) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.Len(t, stmt[0].(*ast.SelectStmt).TableHints, 1)
	require.NoError(t, err)
	require.Len(t, warns, 1)
	require.Equal(t, `[parser:8061]Optimizer hint tidb_unknown is not supported by TiDB and is ignored`, warns[0].Error())
	_, _, err = p.Parse("select c1, c2 from /*+ tidb_unknow(T1,t2) */ t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err) // Hints are ignored after the "FROM" keyword!
	_, _, err = p.Parse("select1 /*+ TIDB_INLJ(t1, T2) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.EqualError(t, err, "line 1 column 7 near \"select1 /*+ TIDB_INLJ(t1, T2) */ c1, c2 from t1, t2 where t1.c1 = t2.c1\" ")
	_, _, err = p.Parse("select /*+ TIDB_INLJ(t1, T2) */ c1, c2 fromt t1, t2 where t1.c1 = t2.c1", "", "")
	require.EqualError(t, err, "line 1 column 47 near \"t1, t2 where t1.c1 = t2.c1\" ")
	_, _, err = p.Parse("SELECT 1 FROM DUAL WHERE 1 IN (SELECT /*+ DEBUG_HINT3 */ 1)", "", "")
	require.NoError(t, err)
	stmt, _, err = p.Parse("insert into t select /*+ memory_quota(1 MB) */ * from t;", "", "")
	require.NoError(t, err)
	require.Len(t, stmt[0].(*ast.InsertStmt).TableHints, 0)
	require.Len(t, stmt[0].(*ast.InsertStmt).Select.(*ast.SelectStmt).TableHints, 1)
	stmt, _, err = p.Parse("insert /*+ memory_quota(1 MB) */ into t select * from t;", "", "")
	require.NoError(t, err)
	require.Len(t, stmt[0].(*ast.InsertStmt).TableHints, 1)

	_, warns, err = p.Parse("SELECT id FROM tbl WHERE id = 0 FOR UPDATE /*+ xyz */", "", "")
	require.NoError(t, err)
	require.Len(t, warns, 1)
	require.Regexp(t, `near '/\*\+' at line 1$`, warns[0].Error())

	_, warns, err = p.Parse("create global binding for select /*+ max_execution_time(1) */ 1 using select /*+ max_execution_time(1) */ 1;\n", "", "")
	require.NoError(t, err)
	require.Len(t, warns, 0)
}

func TestErrorMsg(t *testing.T) {
	p := parser.New()
	_, _, err := p.Parse("select1 1", "", "")
	require.EqualError(t, err, "line 1 column 7 near \"select1 1\" ")
	_, _, err = p.Parse("select 1 from1 dual", "", "")
	require.EqualError(t, err, "line 1 column 19 near \"dual\" ")
	_, _, err = p.Parse("select * from t1 join t2 from t1.a = t2.a;", "", "")
	require.EqualError(t, err, "line 1 column 29 near \"from t1.a = t2.a;\" ")
	_, _, err = p.Parse("select * from t1 join t2 one t1.a = t2.a;", "", "")
	require.EqualError(t, err, "line 1 column 31 near \"t1.a = t2.a;\" ")
	_, _, err = p.Parse("select * from t1 join t2 on t1.a >>> t2.a;", "", "")
	require.EqualError(t, err, "line 1 column 36 near \"> t2.a;\" ")

	_, _, err = p.Parse("create table t(f_year year(5))ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;", "", "")
	require.EqualError(t, err, "[parser:1818]Supports only YEAR or YEAR(4) column")

	_, _, err = p.Parse("create table ``.t (id int);", "", "")
	require.EqualError(t, err, "[parser:1102]Incorrect database name ''")

	_, _, err = p.Parse("create table ` `.t (id int);", "", "")
	require.EqualError(t, err, "[parser:1102]Incorrect database name ' '")

	_, _, err = p.Parse("select ifnull(a,0) & ifnull(a,0) like '55' ESCAPE '\\\\a' from t;", "", "")
	require.EqualError(t, err, "[parser:1210]Incorrect arguments to ESCAPE")

	_, _, err = p.Parse("load data infile 'aaa' into table aaa FIELDS  Enclosed by '\\\\b';", "", "")
	require.EqualError(t, err, "[parser:1083]Field separator argument is not what is expected; check the manual")

	_, _, err = p.Parse("load data infile 'aaa' into table aaa FIELDS  Escaped by '\\\\b';", "", "")
	require.EqualError(t, err, "[parser:1083]Field separator argument is not what is expected; check the manual")

	_, _, err = p.Parse("load data infile 'aaa' into table aaa FIELDS  Enclosed by '\\\\b' Escaped by '\\\\b' ;", "", "")
	require.EqualError(t, err, "[parser:1083]Field separator argument is not what is expected; check the manual")

	_, _, err = p.Parse("ALTER DATABASE `` CHARACTER SET = ''", "", "")
	require.EqualError(t, err, "[parser:1115]Unknown character set: ''")

	_, _, err = p.Parse("ALTER DATABASE t CHARACTER SET = ''", "", "")
	require.EqualError(t, err, "[parser:1115]Unknown character set: ''")

	_, _, err = p.Parse("ALTER SCHEMA t CHARACTER SET = 'SOME_INVALID_CHARSET'", "", "")
	require.EqualError(t, err, "[parser:1115]Unknown character set: 'SOME_INVALID_CHARSET'")

	_, _, err = p.Parse("ALTER DATABASE t COLLATE = ''", "", "")
	require.EqualError(t, err, "[ddl:1273]Unknown collation: ''")

	_, _, err = p.Parse("ALTER SCHEMA t COLLATE = 'SOME_INVALID_COLLATION'", "", "")
	require.EqualError(t, err, "[ddl:1273]Unknown collation: 'SOME_INVALID_COLLATION'")

	_, _, err = p.Parse("ALTER DATABASE CHARSET = 'utf8mb4' COLLATE = 'utf8_bin'", "", "")
	require.EqualError(t, err, "line 1 column 24 near \"= 'utf8mb4' COLLATE = 'utf8_bin'\" ")

	_, _, err = p.Parse("ALTER DATABASE t ENCRYPTION = ''", "", "")
	require.EqualError(t, err, "[parser:1525]Incorrect argument (should be Y or N) value: ''")

	_, _, err = p.Parse("ALTER DATABASE", "", "")
	require.EqualError(t, err, "line 1 column 14 near \"\" ")

	_, _, err = p.Parse("ALTER SCHEMA `ANY_DB_NAME`", "", "")
	require.EqualError(t, err, "line 1 column 26 near \"\" ")

	_, _, err = p.Parse("alter table t partition by range FIELDS(a)", "", "")
	require.EqualError(t, err, "[ddl:1492]For RANGE partitions each partition must be defined")

	_, _, err = p.Parse("alter table t partition by list FIELDS(a)", "", "")
	require.EqualError(t, err, "[ddl:1492]For LIST partitions each partition must be defined")

	_, _, err = p.Parse("alter table t partition by list FIELDS(a)", "", "")
	require.EqualError(t, err, "[ddl:1492]For LIST partitions each partition must be defined")

	_, _, err = p.Parse("alter table t partition by list FIELDS(a,b,c)", "", "")
	require.EqualError(t, err, "[ddl:1492]For LIST partitions each partition must be defined")

	_, _, err = p.Parse("alter table t lock = first", "", "")
	require.EqualError(t, err, "[parser:1801]Unknown LOCK type 'first'")

	_, _, err = p.Parse("alter table t lock = start", "", "")
	require.EqualError(t, err, "[parser:1801]Unknown LOCK type 'start'")

	_, _, err = p.Parse("alter table t lock = commit", "", "")
	require.EqualError(t, err, "[parser:1801]Unknown LOCK type 'commit'")

	_, _, err = p.Parse("alter table t lock = binlog", "", "")
	require.EqualError(t, err, "[parser:1801]Unknown LOCK type 'binlog'")

	_, _, err = p.Parse("alter table t lock = randomStr123", "", "")
	require.EqualError(t, err, "[parser:1801]Unknown LOCK type 'randomStr123'")

	_, _, err = p.Parse("create table t (a longtext unicode)", "", "")
	require.EqualError(t, err, "[parser:1115]Unknown character set: 'ucs2'")

	_, _, err = p.Parse("create table t (a long byte, b text unicode)", "", "")
	require.EqualError(t, err, "[parser:1115]Unknown character set: 'ucs2'")

	_, _, err = p.Parse("create table t (a long ascii, b long unicode)", "", "")
	require.EqualError(t, err, "[parser:1115]Unknown character set: 'ucs2'")

	_, _, err = p.Parse("create table t (a text unicode, b mediumtext ascii, c int)", "", "")
	require.EqualError(t, err, "[parser:1115]Unknown character set: 'ucs2'")

	_, _, err = p.Parse("select 1 collate some_unknown_collation", "", "")
	require.EqualError(t, err, "[ddl:1273]Unknown collation: 'some_unknown_collation'")
}

func TestOptimizerHints(t *testing.T) {
	p := parser.New()
	// Test USE_INDEX
	stmt, _, err := p.Parse("select /*+ USE_INDEX(T1,T2), use_index(t3,t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt := stmt[0].(*ast.SelectStmt)

	hints := selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "use_index", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 1)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Len(t, hints[0].Indexes, 1)
	require.Equal(t, "t2", hints[0].Indexes[0].L)

	require.Equal(t, "use_index", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 1)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Len(t, hints[1].Indexes, 1)
	require.Equal(t, "t4", hints[1].Indexes[0].L)

	// Test FORCE_INDEX
	stmt, _, err = p.Parse("select /*+ FORCE_INDEX(T1,T2), force_index(t3,t4) RESOURCE_GROUP(rg1)*/ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 3)
	require.Equal(t, "force_index", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 1)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Len(t, hints[0].Indexes, 1)
	require.Equal(t, "t2", hints[0].Indexes[0].L)

	require.Equal(t, "force_index", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 1)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Len(t, hints[1].Indexes, 1)
	require.Equal(t, "t4", hints[1].Indexes[0].L)

	require.Equal(t, "resource_group", hints[2].HintName.L)
	require.Equal(t, hints[2].HintData, "rg1")

	// Test IGNORE_INDEX
	stmt, _, err = p.Parse("select /*+ IGNORE_INDEX(T1,T2), ignore_index(t3,t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "ignore_index", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 1)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Len(t, hints[0].Indexes, 1)
	require.Equal(t, "t2", hints[0].Indexes[0].L)

	require.Equal(t, "ignore_index", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 1)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Len(t, hints[1].Indexes, 1)
	require.Equal(t, "t4", hints[1].Indexes[0].L)

	// Test ORDER_INDEX
	stmt, _, err = p.Parse("select /*+ ORDER_INDEX(T1,T2), order_index(t3,t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "order_index", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 1)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Len(t, hints[0].Indexes, 1)
	require.Equal(t, "t2", hints[0].Indexes[0].L)

	require.Equal(t, "order_index", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 1)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Len(t, hints[1].Indexes, 1)
	require.Equal(t, "t4", hints[1].Indexes[0].L)

	// Test NO_ORDER_INDEX
	stmt, _, err = p.Parse("select /*+ NO_ORDER_INDEX(T1,T2), no_order_index(t3,t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "no_order_index", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 1)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Len(t, hints[0].Indexes, 1)
	require.Equal(t, "t2", hints[0].Indexes[0].L)

	require.Equal(t, "no_order_index", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 1)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Len(t, hints[1].Indexes, 1)
	require.Equal(t, "t4", hints[1].Indexes[0].L)

	// Test TIDB_SMJ
	stmt, _, err = p.Parse("select /*+ TIDB_SMJ(T1,t2), tidb_smj(T3,t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "tidb_smj", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)

	require.Equal(t, "tidb_smj", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t4", hints[1].Tables[1].TableName.L)

	// Test MERGE_JOIN
	stmt, _, err = p.Parse("select /*+ MERGE_JOIN(t1, T2), merge_join(t3, t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "merge_join", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)

	require.Equal(t, "merge_join", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t4", hints[1].Tables[1].TableName.L)

	// TEST BROADCAST_JOIN
	stmt, _, err = p.Parse("select /*+ BROADCAST_JOIN(t1, T2), broadcast_join(t3, t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "broadcast_join", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)

	require.Equal(t, "broadcast_join", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t4", hints[1].Tables[1].TableName.L)

	// Test TIDB_INLJ
	stmt, _, err = p.Parse("select /*+ TIDB_INLJ(t1, T2), tidb_inlj(t3, t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "tidb_inlj", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)

	require.Equal(t, "tidb_inlj", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t4", hints[1].Tables[1].TableName.L)

	// Test INL_JOIN
	stmt, _, err = p.Parse("select /*+ INL_JOIN(t1, T2), inl_join(t3, t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "inl_join", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)

	require.Equal(t, "inl_join", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t4", hints[1].Tables[1].TableName.L)

	// Test INL_HASH_JOIN
	stmt, _, err = p.Parse("select /*+ INL_HASH_JOIN(t1, T2), inl_hash_join(t3, t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "inl_hash_join", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)

	require.Equal(t, "inl_hash_join", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t4", hints[1].Tables[1].TableName.L)

	// Test INL_MERGE_JOIN
	stmt, _, err = p.Parse("select /*+ INL_MERGE_JOIN(t1, T2), inl_merge_join(t3, t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "inl_merge_join", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)

	require.Equal(t, "inl_merge_join", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t4", hints[1].Tables[1].TableName.L)

	// Test TIDB_HJ
	stmt, _, err = p.Parse("select /*+ TIDB_HJ(t1, T2), tidb_hj(t3, t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "tidb_hj", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)

	require.Equal(t, "tidb_hj", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t4", hints[1].Tables[1].TableName.L)

	// Test HASH_JOIN
	stmt, _, err = p.Parse("select /*+ HASH_JOIN(t1, T2), hash_join(t3, t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "hash_join", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)

	require.Equal(t, "hash_join", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t4", hints[1].Tables[1].TableName.L)

	// Test HASH_JOIN_BUILD and HASH_JOIN_PROBE
	stmt, _, err = p.Parse("select /*+ hash_join_build(t1), hash_join_probe(t4) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "hash_join_build", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 1)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)

	require.Equal(t, "hash_join_probe", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 1)
	require.Equal(t, "t4", hints[1].Tables[0].TableName.L)

	// Test HASH_JOIN with SWAP_JOIN_INPUTS/NO_SWAP_JOIN_INPUTS
	// t1 for build, t4 for probe
	stmt, _, err = p.Parse("select /*+ HASH_JOIN(t1, T2), hash_join(t3, t4), SWAP_JOIN_INPUTS(t1), NO_SWAP_JOIN_INPUTS(t4)  */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 4)
	require.Equal(t, "hash_join", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)

	require.Equal(t, "hash_join", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t4", hints[1].Tables[1].TableName.L)

	require.Equal(t, "swap_join_inputs", hints[2].HintName.L)
	require.Len(t, hints[2].Tables, 1)
	require.Equal(t, "t1", hints[2].Tables[0].TableName.L)

	require.Equal(t, "no_swap_join_inputs", hints[3].HintName.L)
	require.Len(t, hints[3].Tables, 1)
	require.Equal(t, "t4", hints[3].Tables[0].TableName.L)

	// Test MAX_EXECUTION_TIME
	queries := []string{
		"SELECT /*+ MAX_EXECUTION_TIME(1000) */ * FROM t1 INNER JOIN t2 where t1.c1 = t2.c1",
		"SELECT /*+ MAX_EXECUTION_TIME(1000) */ 1",
		"SELECT /*+ MAX_EXECUTION_TIME(1000) */ SLEEP(20)",
		"SELECT /*+ MAX_EXECUTION_TIME(1000) */ 1 FROM DUAL",
	}
	for i, query := range queries {
		stmt, _, err = p.Parse(query, "", "")
		require.NoError(t, err)
		selectStmt = stmt[0].(*ast.SelectStmt)
		hints = selectStmt.TableHints
		require.Len(t, hints, 1)
		require.Equal(t, "max_execution_time", hints[0].HintName.L, "case", i)
		require.Equal(t, uint64(1000), hints[0].HintData.(uint64))
	}

	// Test NTH_PLAN
	queries = []string{
		"SELECT /*+ NTH_PLAN(10) */ * FROM t1 INNER JOIN t2 where t1.c1 = t2.c1",
		"SELECT /*+ NTH_PLAN(10) */ 1",
		"SELECT /*+ NTH_PLAN(10) */ SLEEP(20)",
		"SELECT /*+ NTH_PLAN(10) */ 1 FROM DUAL",
	}
	for i, query := range queries {
		stmt, _, err = p.Parse(query, "", "")
		require.NoError(t, err)
		selectStmt = stmt[0].(*ast.SelectStmt)
		hints = selectStmt.TableHints
		require.Len(t, hints, 1)
		require.Equal(t, "nth_plan", hints[0].HintName.L, "case", i)
		require.Equal(t, int64(10), hints[0].HintData.(int64))
	}

	// Test USE_INDEX_MERGE
	stmt, _, err = p.Parse("select /*+ USE_INDEX_MERGE(t1, c1), use_index_merge(t2, c1), use_index_merge(t3, c1, primary, c2) */ c1, c2 from t1, t2, t3 where t1.c1 = t2.c1 and t3.c2 = t1.c2", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 3)
	require.Equal(t, "use_index_merge", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 1)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Len(t, hints[0].Indexes, 1)
	require.Equal(t, "c1", hints[0].Indexes[0].L)

	require.Equal(t, "use_index_merge", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 1)
	require.Equal(t, "t2", hints[1].Tables[0].TableName.L)
	require.Len(t, hints[1].Indexes, 1)
	require.Equal(t, "c1", hints[1].Indexes[0].L)

	require.Equal(t, "use_index_merge", hints[2].HintName.L)
	require.Len(t, hints[2].Tables, 1)
	require.Equal(t, "t3", hints[2].Tables[0].TableName.L)
	require.Len(t, hints[2].Indexes, 3)
	require.Equal(t, "c1", hints[2].Indexes[0].L)
	require.Equal(t, "primary", hints[2].Indexes[1].L)
	require.Equal(t, "c2", hints[2].Indexes[2].L)

	// Test READ_FROM_STORAGE
	stmt, _, err = p.Parse("select /*+ READ_FROM_STORAGE(tiflash[t1, t2], tikv[t3]) */ c1, c2 from t1, t2, t1 t3 where t1.c1 = t2.c1 and t2.c1 = t3.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "read_from_storage", hints[0].HintName.L)
	require.Equal(t, "tiflash", hints[0].HintData.(ast.CIStr).L)
	require.Len(t, hints[0].Tables, 2)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)
	require.Equal(t, "t2", hints[0].Tables[1].TableName.L)
	require.Equal(t, "read_from_storage", hints[1].HintName.L)
	require.Equal(t, "tikv", hints[1].HintData.(ast.CIStr).L)
	require.Len(t, hints[1].Tables, 1)
	require.Equal(t, "t3", hints[1].Tables[0].TableName.L)

	// Test USE_TOJA
	stmt, _, err = p.Parse("select /*+ USE_TOJA(true), use_toja(false) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "use_toja", hints[0].HintName.L)
	require.True(t, hints[0].HintData.(bool))

	require.Equal(t, "use_toja", hints[1].HintName.L)
	require.False(t, hints[1].HintData.(bool))

	// Test IGNORE_PLAN_CACHE
	stmt, _, err = p.Parse("select /*+ IGNORE_PLAN_CACHE(), ignore_plan_cache() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)
	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "ignore_plan_cache", hints[0].HintName.L)
	require.Equal(t, "ignore_plan_cache", hints[1].HintName.L)

	stmt, _, err = p.Parse("delete /*+ IGNORE_PLAN_CACHE(), ignore_plan_cache() */ from t where a = 1", "", "")
	require.NoError(t, err)
	deleteStmt := stmt[0].(*ast.DeleteStmt)
	hints = deleteStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "ignore_plan_cache", hints[0].HintName.L)
	require.Equal(t, "ignore_plan_cache", hints[1].HintName.L)

	stmt, _, err = p.Parse("update /*+  IGNORE_PLAN_CACHE(), ignore_plan_cache() */ t set a = 1 where a = 10", "", "")
	require.NoError(t, err)
	updateStmt := stmt[0].(*ast.UpdateStmt)
	hints = updateStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "ignore_plan_cache", hints[0].HintName.L)
	require.Equal(t, "ignore_plan_cache", hints[1].HintName.L)

	// Test USE_CASCADES
	stmt, _, err = p.Parse("select /*+ USE_CASCADES(true), use_cascades(false) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "use_cascades", hints[0].HintName.L)
	require.True(t, hints[0].HintData.(bool))

	require.Equal(t, "use_cascades", hints[1].HintName.L)
	require.False(t, hints[1].HintData.(bool))

	// Test USE_PLAN_CACHE
	stmt, _, err = p.Parse("select /*+ USE_PLAN_CACHE(), use_plan_cache() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "use_plan_cache", hints[0].HintName.L)
	require.Equal(t, "use_plan_cache", hints[1].HintName.L)

	// Test QUERY_TYPE
	stmt, _, err = p.Parse("select /*+ QUERY_TYPE(OLAP), query_type(OLTP) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "query_type", hints[0].HintName.L)
	require.Equal(t, "olap", hints[0].HintData.(ast.CIStr).L)
	require.Equal(t, "query_type", hints[1].HintName.L)
	require.Equal(t, "oltp", hints[1].HintData.(ast.CIStr).L)

	// Test MEMORY_QUOTA
	stmt, _, err = p.Parse("select /*+ MEMORY_QUOTA(1 MB), memory_quota(1 GB) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "memory_quota", hints[0].HintName.L)
	require.Equal(t, int64(1024*1024), hints[0].HintData.(int64))
	require.Equal(t, "memory_quota", hints[1].HintName.L)
	require.Equal(t, int64(1024*1024*1024), hints[1].HintData.(int64))

	_, _, err = p.Parse("select /*+ MEMORY_QUOTA(18446744073709551612 MB), memory_quota(8689934592 GB) */ 1", "", "")
	require.NoError(t, err)

	// Test HASH_AGG
	stmt, _, err = p.Parse("select /*+ HASH_AGG(), hash_agg() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "hash_agg", hints[0].HintName.L)
	require.Equal(t, "hash_agg", hints[1].HintName.L)

	// Test MPPAgg
	stmt, _, err = p.Parse("select /*+ MPP_1PHASE_AGG(), mpp_1phase_agg() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "mpp_1phase_agg", hints[0].HintName.L)
	require.Equal(t, "mpp_1phase_agg", hints[1].HintName.L)

	stmt, _, err = p.Parse("select /*+ MPP_2PHASE_AGG(), mpp_2phase_agg() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "mpp_2phase_agg", hints[0].HintName.L)
	require.Equal(t, "mpp_2phase_agg", hints[1].HintName.L)

	// Test ShuffleJoin
	stmt, _, err = p.Parse("select /*+ SHUFFLE_JOIN(t1, t2), shuffle_join(t1, t2) */ * from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "shuffle_join", hints[0].HintName.L)
	require.Equal(t, "shuffle_join", hints[1].HintName.L)

	// Test STREAM_AGG
	stmt, _, err = p.Parse("select /*+ STREAM_AGG(), stream_agg() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "stream_agg", hints[0].HintName.L)
	require.Equal(t, "stream_agg", hints[1].HintName.L)

	// Test AGG_TO_COP
	stmt, _, err = p.Parse("select /*+ AGG_TO_COP(), agg_to_cop() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "agg_to_cop", hints[0].HintName.L)
	require.Equal(t, "agg_to_cop", hints[1].HintName.L)

	// Test NO_INDEX_MERGE
	stmt, _, err = p.Parse("select /*+ NO_INDEX_MERGE(), no_index_merge() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "no_index_merge", hints[0].HintName.L)
	require.Equal(t, "no_index_merge", hints[1].HintName.L)

	// Test READ_CONSISTENT_REPLICA
	stmt, _, err = p.Parse("select /*+ READ_CONSISTENT_REPLICA(), read_consistent_replica() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "read_consistent_replica", hints[0].HintName.L)
	require.Equal(t, "read_consistent_replica", hints[1].HintName.L)

	// Test LIMIT_TO_COP
	stmt, _, err = p.Parse("select /*+ LIMIT_TO_COP(), limit_to_cop() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "limit_to_cop", hints[0].HintName.L)
	require.Equal(t, "limit_to_cop", hints[1].HintName.L)

	// Test CTE MERGE
	stmt, _, err = p.Parse("with cte(x) as (select * from t1) select /*+ MERGE(), merge() */ * from cte;", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "merge", hints[0].HintName.L)
	require.Equal(t, "merge", hints[1].HintName.L)

	// Test STRAIGHT_JOIN
	stmt, _, err = p.Parse("select /*+ STRAIGHT_JOIN(), straight_join() */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "straight_join", hints[0].HintName.L)
	require.Equal(t, "straight_join", hints[1].HintName.L)

	// Test LEADING
	stmt, _, err = p.Parse("select /*+ LEADING(T1), LEADING(t2, t3), LEADING(T4, t5, t6) */ c1, c2 from t1, t2 where t1.c1 = t2.c1", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 3)
	require.Equal(t, "leading", hints[0].HintName.L)
	require.Len(t, hints[0].Tables, 1)
	require.Equal(t, "t1", hints[0].Tables[0].TableName.L)

	require.Equal(t, "leading", hints[1].HintName.L)
	require.Len(t, hints[1].Tables, 2)
	require.Equal(t, "t2", hints[1].Tables[0].TableName.L)
	require.Equal(t, "t3", hints[1].Tables[1].TableName.L)

	require.Equal(t, "leading", hints[2].HintName.L)
	require.Len(t, hints[2].Tables, 3)
	require.Equal(t, "t4", hints[2].Tables[0].TableName.L)
	require.Equal(t, "t5", hints[2].Tables[1].TableName.L)
	require.Equal(t, "t6", hints[2].Tables[2].TableName.L)

	// Test NO_HASH_JOIN
	stmt, _, err = p.Parse("select /*+ NO_HASH_JOIN(t1, t2), NO_HASH_JOIN(t3) */ * from t1, t2, t3", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "no_hash_join", hints[0].HintName.L)
	require.Equal(t, hints[0].Tables[0].TableName.L, "t1")
	require.Equal(t, hints[0].Tables[1].TableName.L, "t2")

	require.Equal(t, "no_hash_join", hints[1].HintName.L)
	require.Equal(t, hints[1].Tables[0].TableName.L, "t3")

	// Test NO_MERGE_JOIN
	stmt, _, err = p.Parse("select /*+ NO_MERGE_JOIN(t1), NO_MERGE_JOIN(t3) */ * from t1, t2, t3", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "no_merge_join", hints[0].HintName.L)
	require.Equal(t, hints[0].Tables[0].TableName.L, "t1")

	require.Equal(t, "no_merge_join", hints[1].HintName.L)
	require.Equal(t, hints[1].Tables[0].TableName.L, "t3")

	// Test INDEX_JOIN
	stmt, _, err = p.Parse("select /*+ INDEX_JOIN(t1), INDEX_JOIN(t3) */ * from t1, t2, t3", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "index_join", hints[0].HintName.L)
	require.Equal(t, hints[0].Tables[0].TableName.L, "t1")

	require.Equal(t, "index_join", hints[1].HintName.L)
	require.Equal(t, hints[1].Tables[0].TableName.L, "t3")

	// Test NO_INDEX_JOIN
	stmt, _, err = p.Parse("select /*+ NO_INDEX_JOIN(t1), NO_INDEX_JOIN(t3) */ * from t1, t2, t3", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "no_index_join", hints[0].HintName.L)
	require.Equal(t, hints[0].Tables[0].TableName.L, "t1")

	require.Equal(t, "no_index_join", hints[1].HintName.L)
	require.Equal(t, hints[1].Tables[0].TableName.L, "t3")

	// Test INDEX_HASH_JOIN
	stmt, _, err = p.Parse("select /*+ INDEX_HASH_JOIN(t1), INDEX_HASH_JOIN(t3) */ * from t1, t2, t3", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "index_hash_join", hints[0].HintName.L)
	require.Equal(t, hints[0].Tables[0].TableName.L, "t1")

	require.Equal(t, "index_hash_join", hints[1].HintName.L)
	require.Equal(t, hints[1].Tables[0].TableName.L, "t3")

	// Test NO_INDEX_HASH_JOIN
	stmt, _, err = p.Parse("select /*+ NO_INDEX_HASH_JOIN(t1), NO_INDEX_HASH_JOIN(t3) */ * from t1, t2, t3", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "no_index_hash_join", hints[0].HintName.L)
	require.Equal(t, hints[0].Tables[0].TableName.L, "t1")

	require.Equal(t, "no_index_hash_join", hints[1].HintName.L)
	require.Equal(t, hints[1].Tables[0].TableName.L, "t3")

	// Test INDEX_MERGE_JOIN
	stmt, _, err = p.Parse("select /*+ INDEX_MERGE_JOIN(t1), INDEX_MERGE_JOIN(t3) */ * from t1, t2, t3", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "index_merge_join", hints[0].HintName.L)
	require.Equal(t, hints[0].Tables[0].TableName.L, "t1")

	require.Equal(t, "index_merge_join", hints[1].HintName.L)
	require.Equal(t, hints[1].Tables[0].TableName.L, "t3")

	// Test NO_INDEX_MERGE_JOIN
	stmt, _, err = p.Parse("select /*+ NO_INDEX_MERGE_JOIN(t1), NO_INDEX_MERGE_JOIN(t3) */ * from t1, t2, t3", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "no_index_merge_join", hints[0].HintName.L)
	require.Equal(t, hints[0].Tables[0].TableName.L, "t1")

	require.Equal(t, "no_index_merge_join", hints[1].HintName.L)
	require.Equal(t, hints[1].Tables[0].TableName.L, "t3")

	// Test HYPO_INDEX
	stmt, _, err = p.Parse("select /*+ HYPO_INDEX(t1, a), HYPO_INDEX(t3, a, b, c) */ * from t1, t2, t3", "", "")
	require.NoError(t, err)
	selectStmt = stmt[0].(*ast.SelectStmt)

	hints = selectStmt.TableHints
	require.Len(t, hints, 2)
	require.Equal(t, "hypo_index", hints[0].HintName.L)
	require.Equal(t, hints[0].Tables[0].TableName.L, "t1")

	require.Equal(t, "hypo_index", hints[1].HintName.L)
	require.Equal(t, hints[1].Tables[0].TableName.L, "t3")
}

func TestType(t *testing.T) {
	table := []testCase{
		// for time fsp
		{"CREATE TABLE t( c1 TIME(2), c2 DATETIME(2), c3 TIMESTAMP(2) );", true, "CREATE TABLE `t` (`c1` TIME(2),`c2` DATETIME(2),`c3` TIMESTAMP(2))"},

		// for hexadecimal
		{"select x'0a', X'11', 0x11", true, "SELECT x'0a',x'11',x'11'"},
		{"select x'13181C76734725455A'", true, "SELECT x'13181c76734725455a'"},
		{"select x'0xaa'", false, ""},
		{"select 0X11", false, ""},
		{"select 0x4920616D2061206C6F6E672068657820737472696E67", true, "SELECT x'4920616d2061206c6f6e672068657820737472696e67'"},

		// for bit
		{"select 0b01, 0b0, b'11', B'11'", true, "SELECT b'1',b'0',b'11',b'11'"},
		// 0B01 and 0b21 are identifiers, the following two statement could parse.
		// {"select 0B01", false, ""},
		// {"select 0b21", false, ""},

		// for enum and set type
		{"create table t (c1 enum('a', 'b'), c2 set('a', 'b'))", true, "CREATE TABLE `t` (`c1` ENUM('a','b'),`c2` SET('a','b'))"},
		{"create table t (c1 enum('a  ', 'b\t'), c2 set('a  ', 'b\t'))", true, "CREATE TABLE `t` (`c1` ENUM('a','b\t'),`c2` SET('a','b\t'))"},
		{"create table t (c1 enum('a', 'b') binary, c2 set('a', 'b') binary)", true, "CREATE TABLE `t` (`c1` ENUM('a','b') BINARY,`c2` SET('a','b') BINARY)"},
		{"create table t (c1 enum(0x61, 'b'), c2 set(0x61, 'b'))", true, "CREATE TABLE `t` (`c1` ENUM('a','b'),`c2` SET('a','b'))"},
		{"create table t (c1 enum(0b01100001, 'b'), c2 set(0b01100001, 'b'))", true, "CREATE TABLE `t` (`c1` ENUM('a','b'),`c2` SET('a','b'))"},
		{"create table t (c1 enum)", false, ""},
		{"create table t (c1 set)", false, ""},

		// for blob and text field length
		{"create table t (c1 blob(1024), c2 text(1024))", true, "CREATE TABLE `t` (`c1` BLOB(1024),`c2` TEXT(1024))"},

		// for year
		{"create table t (y year(4), y1 year)", true, "CREATE TABLE `t` (`y` YEAR(4),`y1` YEAR)"},
		{"create table t (y year(4) unsigned zerofill zerofill, y1 year signed unsigned zerofill)", true, "CREATE TABLE `t` (`y` YEAR(4),`y1` YEAR)"},

		// for national
		{"create table t (c1 national char(2), c2 national varchar(2))", true, "CREATE TABLE `t` (`c1` CHAR(2),`c2` VARCHAR(2))"},

		// for json type
		{`create table t (a JSON);`, true, "CREATE TABLE `t` (`a` JSON)"},
	}
	RunTest(t, table, false)
}

func TestPrivilege(t *testing.T) {
	table := []testCase{
		// for create user
		{`CREATE USER 'ttt' REQUIRE X509;`, true, "CREATE USER `ttt`@`%` REQUIRE X509"},
		{`CREATE USER 'ttt' REQUIRE SSL;`, true, "CREATE USER `ttt`@`%` REQUIRE SSL"},
		{`CREATE USER 'ttt' REQUIRE NONE;`, true, "CREATE USER `ttt`@`%` REQUIRE NONE"},
		{`CREATE USER 'ttt' REQUIRE ISSUER '/C=SE/ST=Stockholm/L=Stockholm/O=MySQL/CN=CA/emailAddress=ca@example.com' AND CIPHER 'EDH-RSA-DES-CBC3-SHA';`, true, "CREATE USER `ttt`@`%` REQUIRE ISSUER '/C=SE/ST=Stockholm/L=Stockholm/O=MySQL/CN=CA/emailAddress=ca@example.com' AND CIPHER 'EDH-RSA-DES-CBC3-SHA'"},
		{`CREATE USER 'ttt' REQUIRE ISSUER '/C=SE/ST=Stockholm/L=Stockholm/O=MySQL/CN=CA/emailAddress=ca@example.com' CIPHER 'EDH-RSA-DES-CBC3-SHA' SUBJECT '/C=SE/ST=Stockholm/L=Stockholm/O=MySQL/CN=CA/emailAddress=ca@example.com';`, true, "CREATE USER `ttt`@`%` REQUIRE ISSUER '/C=SE/ST=Stockholm/L=Stockholm/O=MySQL/CN=CA/emailAddress=ca@example.com' AND CIPHER 'EDH-RSA-DES-CBC3-SHA' AND SUBJECT '/C=SE/ST=Stockholm/L=Stockholm/O=MySQL/CN=CA/emailAddress=ca@example.com'"},
		{`CREATE USER 'ttt' REQUIRE SAN 'DNS:mysql-user, URI:spiffe://example.org/myservice'`, true, "CREATE USER `ttt`@`%` REQUIRE SAN 'DNS:mysql-user, URI:spiffe://example.org/myservice'"},
		{`CREATE USER 'ttt' WITH MAX_QUERIES_PER_HOUR 2;`, true, "CREATE USER `ttt`@`%` WITH MAX_QUERIES_PER_HOUR 2"},
		{`CREATE USER 'ttt'@'localhost' REQUIRE NONE WITH MAX_QUERIES_PER_HOUR 1 MAX_UPDATES_PER_HOUR 10 PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK;`, true, "CREATE USER `ttt`@`localhost` REQUIRE NONE WITH MAX_QUERIES_PER_HOUR 1 MAX_UPDATES_PER_HOUR 10 PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK"},
		{`CREATE USER 'u1'@'%' IDENTIFIED WITH 'mysql_native_password' AS '' REQUIRE NONE PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK ;`, true, "CREATE USER `u1`@`%` IDENTIFIED WITH 'mysql_native_password' AS '' REQUIRE NONE PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK"},
		{`CREATE USER 'test'`, true, "CREATE USER `test`@`%`"},
		{`CREATE USER test`, true, "CREATE USER `test`@`%`"},
		{"CREATE USER `test`", true, "CREATE USER `test`@`%`"},
		{"CREATE USER test-user", false, ""},
		{"CREATE USER test.user", false, ""},
		{"CREATE USER 'test-user'", true, "CREATE USER `test-user`@`%`"},
		{"CREATE USER `test-user`", true, "CREATE USER `test-user`@`%`"},
		{"CREATE USER test.user", false, ""},
		{"CREATE USER 'test.user'", true, "CREATE USER `test.user`@`%`"},
		{"CREATE USER `test.user`", true, "CREATE USER `test.user`@`%`"},
		{"CREATE USER uesr1@LOCALhost", true, "CREATE USER `uesr1`@`localhost`"},
		{"CREATE USER `uesr1`@localhost", true, "CREATE USER `uesr1`@`localhost`"},
		{"CREATE USER uesr1@`localhost`", true, "CREATE USER `uesr1`@`localhost`"},
		{"CREATE USER `uesr1`@`localhost`", true, "CREATE USER `uesr1`@`localhost`"},
		{"CREATE USER 'uesr1'@localhost", true, "CREATE USER `uesr1`@`localhost`"},
		{"CREATE USER uesr1@'localhost'", true, "CREATE USER `uesr1`@`localhost`"},
		{"CREATE USER 'uesr1'@'localhost'", true, "CREATE USER `uesr1`@`localhost`"},
		{"CREATE USER 'uesr1'@`localhost`", true, "CREATE USER `uesr1`@`localhost`"},
		{"CREATE USER `uesr1`@'localhost'", true, "CREATE USER `uesr1`@`localhost`"},
		{"create user 'test@localhost' password expire;", true, "CREATE USER `test@localhost`@`%` PASSWORD EXPIRE"},
		{"create user 'test@localhost' password expire never;", true, "CREATE USER `test@localhost`@`%` PASSWORD EXPIRE NEVER"},
		{"create user 'test@localhost' password expire default;", true, "CREATE USER `test@localhost`@`%` PASSWORD EXPIRE DEFAULT"},
		{"create user 'test@localhost' password expire interval 3 day;", true, "CREATE USER `test@localhost`@`%` PASSWORD EXPIRE INTERVAL 3 DAY"},
		{"create user 'test@localhost' identified by 'password' failed_login_attempts 3 password_lock_time 3;", true, "CREATE USER `test@localhost`@`%` IDENTIFIED BY 'password' FAILED_LOGIN_ATTEMPTS 3 PASSWORD_LOCK_TIME 3"},
		{"create user 'test@localhost' identified by 'password' failed_login_attempts 3 password_lock_time unbounded;", true, "CREATE USER `test@localhost`@`%` IDENTIFIED BY 'password' FAILED_LOGIN_ATTEMPTS 3 PASSWORD_LOCK_TIME UNBOUNDED"},
		{"create user 'test@localhost' identified by 'password' failed_login_attempts 3;", true, "CREATE USER `test@localhost`@`%` IDENTIFIED BY 'password' FAILED_LOGIN_ATTEMPTS 3"},
		{"create user 'test@localhost' identified by 'password' password_lock_time 3;", true, "CREATE USER `test@localhost`@`%` IDENTIFIED BY 'password' PASSWORD_LOCK_TIME 3"},
		{"create user 'test@localhost' identified by 'password' password_lock_time unbounded;", true, "CREATE USER `test@localhost`@`%` IDENTIFIED BY 'password' PASSWORD_LOCK_TIME UNBOUNDED"},
		{"CREATE USER 'sha_test'@'localhost' IDENTIFIED WITH 'caching_sha2_password' BY 'sha_test'", true, "CREATE USER `sha_test`@`localhost` IDENTIFIED WITH 'caching_sha2_password' BY 'sha_test'"},
		{"CREATE USER 'sha_test3'@'localhost' IDENTIFIED WITH 'caching_sha2_password' AS 0x24412430303524255B03496C662C1055127B3B654A2F04207D01485276703644704B76303247474564416A516662346C5868646D32764C6B514F43585A473779565947514F34", true, "CREATE USER `sha_test3`@`localhost` IDENTIFIED WITH 'caching_sha2_password' AS '$A$005$%[\x03Ilf,\x10U\x12{;eJ/\x04 }\x01HRvp6DpKv02GGEdAjQfb4lXhdm2vLkQOCXZG7yVYGQO4'"},
		{"CREATE USER 'sha_test4'@'localhost' IDENTIFIED WITH 'caching_sha2_password' AS '$A$005$%[\x03Ilf,\x10U\x12{;eJ/\x04 }\x01HRvp6DpKv02GGEdAjQfb4lXhdm2vLkQOCXZG7yVYGQO4'", true, "CREATE USER `sha_test4`@`localhost` IDENTIFIED WITH 'caching_sha2_password' AS '$A$005$%[\x03Ilf,\x10U\x12{;eJ/\x04 }\x01HRvp6DpKv02GGEdAjQfb4lXhdm2vLkQOCXZG7yVYGQO4'"},
		{"CREATE USER `user@pingcap.com`@'localhost' IDENTIFIED WITH 'tidb_auth_token' REQUIRE token_issuer 'issuer-abc' ATTRIBUTE '{\"email\": \"user@pingcap.com\"}'", true, "CREATE USER `user@pingcap.com`@`localhost` IDENTIFIED WITH 'tidb_auth_token' REQUIRE TOKEN_ISSUER 'issuer-abc' ATTRIBUTE '{\"email\": \"user@pingcap.com\"}'"},
		{"CREATE USER 'nopwd_native'@'localhost' IDENTIFIED WITH 'mysql_native_password'", true, "CREATE USER `nopwd_native`@`localhost` IDENTIFIED WITH 'mysql_native_password'"},
		{"CREATE USER 'nopwd_sha'@'localhost' IDENTIFIED WITH 'caching_sha2_password'", true, "CREATE USER `nopwd_sha`@`localhost` IDENTIFIED WITH 'caching_sha2_password'"},
		{"CREATE ROLE `test-role`, `role1`@'localhost'", true, "CREATE ROLE `test-role`@`%`, `role1`@`localhost`"},
		{"CREATE ROLE `test-role`", true, "CREATE ROLE `test-role`@`%`"},
		{"CREATE ROLE role1", true, "CREATE ROLE `role1`@`%`"},
		{"CREATE ROLE `role1`@'localhost'", true, "CREATE ROLE `role1`@`localhost`"},
		{"create user 'bug19354014user'@'%' identified WITH mysql_native_password", true, "CREATE USER `bug19354014user`@`%` IDENTIFIED WITH 'mysql_native_password'"},
		{"create user 'bug19354014user'@'%' identified WITH mysql_native_password by 'new-password'", true, "CREATE USER `bug19354014user`@`%` IDENTIFIED WITH 'mysql_native_password' BY 'new-password'"},
		{"create user 'bug19354014user'@'%' identified WITH mysql_native_password as 'hashstring'", true, "CREATE USER `bug19354014user`@`%` IDENTIFIED WITH 'mysql_native_password' AS 'hashstring'"},
		{`CREATE USER IF NOT EXISTS 'root'@'localhost' IDENTIFIED BY 'new-password'`, true, "CREATE USER IF NOT EXISTS `root`@`localhost` IDENTIFIED BY 'new-password'"},
		{`CREATE USER 'root'@'localhost' IDENTIFIED BY 'new-password'`, true, "CREATE USER `root`@`localhost` IDENTIFIED BY 'new-password'"},
		{`CREATE USER 'root'@'localhost' IDENTIFIED BY PASSWORD 'hashstring'`, true, "CREATE USER `root`@`localhost` IDENTIFIED WITH 'mysql_native_password' AS 'hashstring'"},
		{`CREATE USER 'root'@'localhost' IDENTIFIED BY 'new-password', 'root'@'127.0.0.1' IDENTIFIED BY PASSWORD 'hashstring'`, true, "CREATE USER `root`@`localhost` IDENTIFIED BY 'new-password', `root`@`127.0.0.1` IDENTIFIED WITH 'mysql_native_password' AS 'hashstring'"},
		{`CREATE USER 'root'@'127.0.0.1' IDENTIFIED BY 'hashstring' RESOURCE GROUP rg1`, true, "CREATE USER `root`@`127.0.0.1` IDENTIFIED BY 'hashstring' RESOURCE GROUP `rg1`"},
		{`ALTER USER IF EXISTS 'root'@'localhost' IDENTIFIED BY 'new-password'`, true, "ALTER USER IF EXISTS `root`@`localhost` IDENTIFIED BY 'new-password'"},
		{`ALTER USER 'root'@'localhost' IDENTIFIED BY 'new-password'`, true, "ALTER USER `root`@`localhost` IDENTIFIED BY 'new-password'"},
		{`ALTER USER 'root'@'localhost' RESOURCE GROUP rg2`, true, "ALTER USER `root`@`localhost` RESOURCE GROUP `rg2`"},
		{`ALTER USER 'root'@'localhost' IDENTIFIED BY PASSWORD 'hashstring'`, true, "ALTER USER `root`@`localhost` IDENTIFIED WITH 'mysql_native_password' AS 'hashstring'"},
		{`ALTER USER 'root'@'localhost' IDENTIFIED BY 'new-password', 'root'@'127.0.0.1' IDENTIFIED BY PASSWORD 'hashstring'`, true, "ALTER USER `root`@`localhost` IDENTIFIED BY 'new-password', `root`@`127.0.0.1` IDENTIFIED WITH 'mysql_native_password' AS 'hashstring'"},
		{`ALTER USER USER() IDENTIFIED BY 'new-password'`, true, "ALTER USER USER() IDENTIFIED BY 'new-password'"},
		{`ALTER USER IF EXISTS USER() IDENTIFIED BY 'new-password'`, true, "ALTER USER IF EXISTS USER() IDENTIFIED BY 'new-password'"},
		{"alter user 'test@localhost' password expire;", true, "ALTER USER `test@localhost`@`%` PASSWORD EXPIRE"},
		{"alter user 'test@localhost' password expire never;", true, "ALTER USER `test@localhost`@`%` PASSWORD EXPIRE NEVER"},
		{"alter user 'test@localhost' password expire default;", true, "ALTER USER `test@localhost`@`%` PASSWORD EXPIRE DEFAULT"},
		{"alter user 'test@localhost' password expire interval 3 day;", true, "ALTER USER `test@localhost`@`%` PASSWORD EXPIRE INTERVAL 3 DAY"},
		{"ALTER USER 'ttt' REQUIRE X509;", true, "ALTER USER `ttt`@`%` REQUIRE X509"},
		{"ALTER USER 'ttt' REQUIRE SSL;", true, "ALTER USER `ttt`@`%` REQUIRE SSL"},
		{"ALTER USER 'ttt' REQUIRE NONE;", true, "ALTER USER `ttt`@`%` REQUIRE NONE"},
		{"ALTER USER 'ttt' REQUIRE ISSUER '/C=SE/ST=Stockholm/L=Stockholm/O=MySQL/CN=CA/emailAddress=ca@example.com' AND CIPHER 'EDH-RSA-DES-CBC3-SHA';", true, "ALTER USER `ttt`@`%` REQUIRE ISSUER '/C=SE/ST=Stockholm/L=Stockholm/O=MySQL/CN=CA/emailAddress=ca@example.com' AND CIPHER 'EDH-RSA-DES-CBC3-SHA'"},
		{"ALTER USER 'ttt' WITH MAX_QUERIES_PER_HOUR 2;", true, "ALTER USER `ttt`@`%` WITH MAX_QUERIES_PER_HOUR 2"},
		{"ALTER USER 'ttt' WITH MAX_UPDATES_PER_HOUR 2;", true, "ALTER USER `ttt`@`%` WITH MAX_UPDATES_PER_HOUR 2"},
		{"ALTER USER 'ttt' WITH MAX_CONNECTIONS_PER_HOUR 2;", true, "ALTER USER `ttt`@`%` WITH MAX_CONNECTIONS_PER_HOUR 2"},
		{"ALTER USER 'ttt' WITH MAX_USER_CONNECTIONS 2;", true, "ALTER USER `ttt`@`%` WITH MAX_USER_CONNECTIONS 2"},
		{"ALTER USER 'ttt'@'localhost' REQUIRE NONE WITH MAX_QUERIES_PER_HOUR 1 MAX_UPDATES_PER_HOUR 10 PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK;", true, "ALTER USER `ttt`@`localhost` REQUIRE NONE WITH MAX_QUERIES_PER_HOUR 1 MAX_UPDATES_PER_HOUR 10 PASSWORD EXPIRE DEFAULT ACCOUNT UNLOCK"},
		{`DROP USER 'root'@'localhost', 'root1'@'localhost'`, true, "DROP USER `root`@`localhost`, `root1`@`localhost`"},
		{`DROP USER IF EXISTS 'root'@'localhost'`, true, "DROP USER IF EXISTS `root`@`localhost`"},
		{`RENAME USER 'root'@'localhost' TO 'root'@'%'`, true, "RENAME USER `root`@`localhost` TO `root`@`%`"},
		{`RENAME USER 'fred' TO 'barry'`, true, "RENAME USER `fred`@`%` TO `barry`@`%`"},
		{`RENAME USER u1 to u2, u3 to u4`, true, "RENAME USER `u1`@`%` TO `u2`@`%`, `u3`@`%` TO `u4`@`%`"},
		{`DROP ROLE 'role'@'localhost', 'role1'@'localhost'`, true, "DROP ROLE `role`@`localhost`, `role1`@`localhost`"},
		{`DROP ROLE 'administrator', 'developer';`, true, "DROP ROLE `administrator`@`%`, `developer`@`%`"},
		{`DROP ROLE IF EXISTS 'role'@'localhost'`, true, "DROP ROLE IF EXISTS `role`@`localhost`"},

		// for grant statement
		{"GRANT ALL ON db1.* TO 'jeffrey'@'localhost' REQUIRE X509;", true, "GRANT ALL ON `db1`.* TO `jeffrey`@`localhost` REQUIRE X509"},
		{"GRANT ALL ON db1.* TO 'jeffrey'@'LOCALhost' REQUIRE SSL;", true, "GRANT ALL ON `db1`.* TO `jeffrey`@`localhost` REQUIRE SSL"},
		{"GRANT ALL ON db1.* TO 'jeffrey'@'localhost' REQUIRE NONE;", true, "GRANT ALL ON `db1`.* TO `jeffrey`@`localhost` REQUIRE NONE"},
		{"GRANT ALL ON db1.* TO 'jeffrey'@'localhost' REQUIRE ISSUER '/C=SE/ST=Stockholm/L=Stockholm/O=MySQL/CN=CA/emailAddress=ca@example.com' AND CIPHER 'EDH-RSA-DES-CBC3-SHA';", true, "GRANT ALL ON `db1`.* TO `jeffrey`@`localhost` REQUIRE ISSUER '/C=SE/ST=Stockholm/L=Stockholm/O=MySQL/CN=CA/emailAddress=ca@example.com' AND CIPHER 'EDH-RSA-DES-CBC3-SHA'"},
		{"GRANT ALL ON db1.* TO 'jeffrey'@'localhost';", true, "GRANT ALL ON `db1`.* TO `jeffrey`@`localhost`"},
		{"GRANT ALL ON TABLE db1.* TO 'jeffrey'@'localhost';", true, "GRANT ALL ON TABLE `db1`.* TO `jeffrey`@`localhost`"},
		{"GRANT ALL ON db1.* TO 'jeffrey'@'localhost' WITH GRANT OPTION;", true, "GRANT ALL ON `db1`.* TO `jeffrey`@`localhost` WITH GRANT OPTION"},
		{"GRANT SELECT ON db2.invoice TO 'jeffrey'@'localhost';", true, "GRANT SELECT ON `db2`.`invoice` TO `jeffrey`@`localhost`"},
		{"GRANT ALL ON *.* TO 'someuser'@'somehost';", true, "GRANT ALL ON *.* TO `someuser`@`somehost`"},
		{"GRANT ALL ON *.* TO 'SOMEuser'@'SOMEhost';", true, "GRANT ALL ON *.* TO `SOMEuser`@`somehost`"},
		{"GRANT SELECT, INSERT ON *.* TO 'someuser'@'somehost';", true, "GRANT SELECT, INSERT ON *.* TO `someuser`@`somehost`"},
		{"GRANT ALL ON mydb.* TO 'someuser'@'somehost';", true, "GRANT ALL ON `mydb`.* TO `someuser`@`somehost`"},
		{"GRANT SELECT, INSERT ON mydb.* TO 'someuser'@'somehost';", true, "GRANT SELECT, INSERT ON `mydb`.* TO `someuser`@`somehost`"},
		{"GRANT ALL ON mydb.mytbl TO 'someuser'@'somehost';", true, "GRANT ALL ON `mydb`.`mytbl` TO `someuser`@`somehost`"},
		{"GRANT SELECT, INSERT ON mydb.mytbl TO 'someuser'@'somehost';", true, "GRANT SELECT, INSERT ON `mydb`.`mytbl` TO `someuser`@`somehost`"},
		{"GRANT SELECT (col1), INSERT (col1,col2) ON mydb.mytbl TO 'someuser'@'somehost';", true, "GRANT SELECT (`col1`), INSERT (`col1`,`col2`) ON `mydb`.`mytbl` TO `someuser`@`somehost`"},
		{"grant all privileges on zabbix.* to 'zabbix'@'localhost' identified by 'password';", true, "GRANT ALL ON `zabbix`.* TO `zabbix`@`localhost` IDENTIFIED BY 'password'"},
		{"GRANT SELECT ON test.* to 'test'", true, "GRANT SELECT ON `test`.* TO `test`@`%`"}, // For issue 2654.
		{"grant PROCESS,usage, REPLICATION SLAVE, REPLICATION CLIENT on *.* to 'xxxxxxxxxx'@'%' identified by password 'xxxxxxxxxxxxxxxxxxxxxxxxxxxx'", true, "GRANT PROCESS, USAGE, REPLICATION SLAVE, REPLICATION CLIENT ON *.* TO `xxxxxxxxxx`@`%` IDENTIFIED WITH 'mysql_native_password' AS 'xxxxxxxxxxxxxxxxxxxxxxxxxxxx'"},
		{"/* rds internal mark */ GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, REFERENCES, RELOAD, PROCESS, INDEX, ALTER, CREATE TEMPORARY TABLES, LOCK TABLES,      EXECUTE, REPLICATION SLAVE, REPLICATION CLIENT, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, CREATE USER, EVENT,      TRIGGER on *.* to 'root2'@'%' identified by password '*sdsadsdsadssadsadsadsadsada' with grant option", true, "GRANT SELECT, INSERT, UPDATE, DELETE, CREATE, DROP, REFERENCES, RELOAD, PROCESS, INDEX, ALTER, CREATE TEMPORARY TABLES, LOCK TABLES, EXECUTE, REPLICATION SLAVE, REPLICATION CLIENT, CREATE VIEW, SHOW VIEW, CREATE ROUTINE, ALTER ROUTINE, CREATE USER, EVENT, TRIGGER ON *.* TO `root2`@`%` IDENTIFIED WITH 'mysql_native_password' AS '*sdsadsdsadssadsadsadsadsada' WITH GRANT OPTION"},
		{"GRANT 'role1', 'role2' TO 'user1'@'LOCalhost', 'user2'@'LOcalhost';", true, "GRANT `role1`@`%`, `role2`@`%` TO `user1`@`localhost`, `user2`@`localhost`"},
		{"GRANT 'u1' TO 'u1';", true, "GRANT `u1`@`%` TO `u1`@`%`"},
		{"GRANT 'app_read'@'%','app_write'@'%' TO 'rw_user1'@'localhost'", true, "GRANT `app_read`@`%`, `app_write`@`%` TO `rw_user1`@`localhost`"},
		{"GRANT 'app_developer' TO 'dev1'@'localhost';", true, "GRANT `app_developer`@`%` TO `dev1`@`localhost`"},
		{"GRANT SHUTDOWN ON *.* TO 'dev1'@'localhost';", true, "GRANT SHUTDOWN ON *.* TO `dev1`@`localhost`"},
		{"GRANT CONFIG ON *.* TO 'dev1'@'localhost';", true, "GRANT CONFIG ON *.* TO `dev1`@`localhost`"},
		{"GRANT CREATE ON *.* TO 'dev1'@'localhost';", true, "GRANT CREATE ON *.* TO `dev1`@`localhost`"},
		{"GRANT CREATE TABLESPACE ON *.* TO 'dev1'@'localhost';", true, "GRANT CREATE TABLESPACE ON *.* TO `dev1`@`localhost`"},
		{"GRANT EXECUTE ON FUNCTION db1.anomaly_score TO 'user1'@'domain-or-ip-address1'", true, "GRANT EXECUTE ON FUNCTION `db1`.`anomaly_score` TO `user1`@`domain-or-ip-address1`"},
		{"GRANT EXECUTE ON PROCEDURE mydb.myproc TO 'someuser'@'somehost'", true, "GRANT EXECUTE ON PROCEDURE `mydb`.`myproc` TO `someuser`@`somehost`"},
		{"GRANT APPLICATION_PASSWORD_ADMIN,AUDIT_ADMIN ON *.* TO 'root'@'localhost'", true, "GRANT APPLICATION_PASSWORD_ADMIN, AUDIT_ADMIN ON *.* TO `root`@`localhost`"},
		{"GRANT LOAD FROM S3, SELECT INTO S3, INVOKE LAMBDA, INVOKE SAGEMAKER, INVOKE COMPREHEND ON *.* TO 'root'@'localhost'", true, "GRANT LOAD FROM S3, SELECT INTO S3, INVOKE LAMBDA, INVOKE SAGEMAKER, INVOKE COMPREHEND ON *.* TO `root`@`localhost`"},
		{"GRANT PROXY ON 'localuser'@'localhost' TO 'externaluser'@'somehost'", true, "GRANT PROXY ON `localuser`@`localhost` TO `externaluser`@`somehost`"},
		{"GRANT PROXY ON ''@'' TO 'root'@'localhost' WITH GRANT OPTION", true, "GRANT PROXY ON ``@`` TO `root`@`localhost` WITH GRANT OPTION"},
		{"GRANT PROXY ON 'proxied_user' TO 'proxy_user1', 'proxy_user2'", true, "GRANT PROXY ON `proxied_user`@`%` TO `proxy_user1`@`%`, `proxy_user2`@`%`"},
		{"grant grant option on *.* to u1", true, "GRANT GRANT OPTION ON *.* TO `u1`@`%`"}, // not typical syntax, but supported

		// for revoke statement
		{"REVOKE ALL ON db1.* FROM 'jeffrey'@'LOCalhost';", true, "REVOKE ALL ON `db1`.* FROM `jeffrey`@`localhost`"},
		{"REVOKE SELECT ON db2.invoice FROM 'jeffrey'@'localhost';", true, "REVOKE SELECT ON `db2`.`invoice` FROM `jeffrey`@`localhost`"},
		{"REVOKE ALL ON *.* FROM 'someuser'@'somehost';", true, "REVOKE ALL ON *.* FROM `someuser`@`somehost`"},
		{"REVOKE SELECT, INSERT ON *.* FROM 'someuser'@'somehost';", true, "REVOKE SELECT, INSERT ON *.* FROM `someuser`@`somehost`"},
		{"REVOKE ALL ON mydb.* FROM 'someuser'@'somehost';", true, "REVOKE ALL ON `mydb`.* FROM `someuser`@`somehost`"},
		{"REVOKE SELECT, INSERT ON mydb.* FROM 'someuser'@'somehost';", true, "REVOKE SELECT, INSERT ON `mydb`.* FROM `someuser`@`somehost`"},
		{"REVOKE ALL ON mydb.mytbl FROM 'someuser'@'somehost';", true, "REVOKE ALL ON `mydb`.`mytbl` FROM `someuser`@`somehost`"},
		{"REVOKE SELECT, INSERT ON mydb.mytbl FROM 'someuser'@'somehost';", true, "REVOKE SELECT, INSERT ON `mydb`.`mytbl` FROM `someuser`@`somehost`"},
		{"REVOKE SELECT (col1), INSERT (col1,col2) ON mydb.mytbl FROM 'someuser'@'somehost';", true, "REVOKE SELECT (`col1`), INSERT (`col1`,`col2`) ON `mydb`.`mytbl` FROM `someuser`@`somehost`"},
		{"REVOKE all privileges on zabbix.* FROM 'zabbix'@'localhost' identified by 'password';", true, "REVOKE ALL ON `zabbix`.* FROM `zabbix`@`localhost` IDENTIFIED BY 'password'"},
		{"REVOKE 'role1', 'role2' FROM 'user1'@'localhost', 'user2'@'localhost';", true, "REVOKE `role1`@`%`, `role2`@`%` FROM `user1`@`localhost`, `user2`@`localhost`"},
		{"REVOKE SHUTDOWN ON *.* FROM 'dev1'@'localhost';", true, "REVOKE SHUTDOWN ON *.* FROM `dev1`@`localhost`"},
		{"REVOKE CONFIG ON *.* FROM 'dev1'@'localhost';", true, "REVOKE CONFIG ON *.* FROM `dev1`@`localhost`"},
		{"REVOKE EXECUTE ON FUNCTION db.func FROM 'user'@'localhost'", true, "REVOKE EXECUTE ON FUNCTION `db`.`func` FROM `user`@`localhost`"},
		{"REVOKE EXECUTE ON PROCEDURE db.func FROM 'user'@'localhost'", true, "REVOKE EXECUTE ON PROCEDURE `db`.`func` FROM `user`@`localhost`"},
		{"REVOKE APPLICATION_PASSWORD_ADMIN,AUDIT_ADMIN ON *.* FROM 'root'@'localhost'", true, "REVOKE APPLICATION_PASSWORD_ADMIN, AUDIT_ADMIN ON *.* FROM `root`@`localhost`"},
		{"revoke all privileges, grant option from u1", true, "REVOKE ALL, GRANT OPTION ON *.* FROM `u1`@`%`"},                             // special case syntax
		{"revoke all privileges, grant option from u1, u2, u3", true, "REVOKE ALL, GRANT OPTION ON *.* FROM `u1`@`%`, `u2`@`%`, `u3`@`%`"}, // special case syntax
	}
	RunTest(t, table, false)
}

func TestComment(t *testing.T) {
	table := []testCase{
		{"create table t (c int comment 'comment')", true, "CREATE TABLE `t` (`c` INT COMMENT 'comment')"},
		{"create table t (c int) comment = 'comment'", true, "CREATE TABLE `t` (`c` INT) COMMENT = 'comment'"},
		{"create table t (c int) comment 'comment'", true, "CREATE TABLE `t` (`c` INT) COMMENT = 'comment'"},
		{"create table t (c int) comment comment", false, ""},
		{"create table t (comment text)", true, "CREATE TABLE `t` (`comment` TEXT)"},
		{"START TRANSACTION /*!40108 WITH CONSISTENT SNAPSHOT */", true, "START TRANSACTION"},
		// for comment in query
		{"/*comment*/ /*comment*/ select c /* this is a comment */ from t;", true, "SELECT `c` FROM `t`"},
		// for unclosed comment
		{"delete from t where a = 7 or 1=1/*' and b = 'p'", false, ""},

		{"create table t (ssl int)", false, ""},
		{"create table t (require int)", false, ""},
		{"create table t (account int)", true, "CREATE TABLE `t` (`account` INT)"},
		{"create table t (expire int)", true, "CREATE TABLE `t` (`expire` INT)"},
		{"create table t (cipher int)", true, "CREATE TABLE `t` (`cipher` INT)"},
		{"create table t (issuer int)", true, "CREATE TABLE `t` (`issuer` INT)"},
		{"create table t (never int)", true, "CREATE TABLE `t` (`never` INT)"},
		{"create table t (subject int)", true, "CREATE TABLE `t` (`subject` INT)"},
		{"create table t (x509 int)", true, "CREATE TABLE `t` (`x509` INT)"},

		// COMMENT/ATTRIBUTE in CREATE/ALTER USER
		{"create user commentUser COMMENT '123456' '{\"name\": \"Tom\", \"age\", 19}", false, ""},
		{"alter user commentUser COMMENT '123456' '{\"name\": \"Tom\", \"age\", 19}", false, ""},
		{"create user commentUser COMMENT '123456'", true, "CREATE USER `commentUser`@`%` COMMENT '123456'"},
		{"alter user commentUser COMMENT '123456'", true, "ALTER USER `commentUser`@`%` COMMENT '123456'"},
		{"create user commentUser ATTRIBUTE '{\"name\": \"Tom\", \"age\", 19}'", true, "CREATE USER `commentUser`@`%` ATTRIBUTE '{\"name\": \"Tom\", \"age\", 19}'"},
		{"alter user commentUser ATTRIBUTE '{\"name\": \"Tom\", \"age\", 19}'", true, "ALTER USER `commentUser`@`%` ATTRIBUTE '{\"name\": \"Tom\", \"age\", 19}'"},
	}
	RunTest(t, table, false)
}

func TestParserErrMsg(t *testing.T) {
	commentMsgCases := []testErrMsgCase{
		{"delete from t where a = 7 or 1=1/*' and b = 'p'", errors.New("near '/*' and b = 'p'' at line 1")},
		{"delete from t where a = 7 or\n 1=1/*' and b = 'p'", errors.New("near '/*' and b = 'p'' at line 2")},
		{"select 1/*", errors.New("near '/*' at line 1")},
		{"select 1/* comment */", nil},
	}
	funcCallMsgCases := []testErrMsgCase{
		{"select a.b()", nil},
		{"SELECT foo.bar('baz');", nil},
	}
	RunErrMsgTest(t, commentMsgCases)
	RunErrMsgTest(t, funcCallMsgCases)
}

type subqueryChecker struct {
	text string
	t    *testing.T
}

// Enter implements ast.Visitor interface.
func (sc *subqueryChecker) Enter(inNode ast.Node) (outNode ast.Node, skipChildren bool) {
	if expr, ok := inNode.(*ast.SubqueryExpr); ok {
		require.Equal(sc.t, sc.text, expr.Query.Text())
		return inNode, true
	}
	return inNode, false
}

// Leave implements ast.Visitor interface.
func (sc *subqueryChecker) Leave(inNode ast.Node) (node ast.Node, ok bool) {
	return inNode, true
}

func TestSubquery(t *testing.T) {
	table := []testCase{
		// for compare subquery
		{"SELECT 1 > (select 1)", true, "SELECT 1>(SELECT 1)"},
		{"SELECT 1 > ANY (select 1)", true, "SELECT 1>ANY (SELECT 1)"},
		{"SELECT 1 > ALL (select 1)", true, "SELECT 1>ALL (SELECT 1)"},
		{"SELECT 1 > SOME (select 1)", true, "SELECT 1>ANY (SELECT 1)"},

		// for exists subquery
		{"SELECT EXISTS select 1", false, ""},
		{"SELECT EXISTS (select 1)", true, "SELECT EXISTS (SELECT 1)"},
		{"SELECT + EXISTS (select 1)", true, "SELECT +EXISTS (SELECT 1)"},
		{"SELECT - EXISTS (select 1)", true, "SELECT -EXISTS (SELECT 1)"},
		{"SELECT NOT EXISTS (select 1)", true, "SELECT NOT EXISTS (SELECT 1)"},
		{"SELECT + NOT EXISTS (select 1)", false, ""},
		{"SELECT - NOT EXISTS (select 1)", false, ""},
		{"SELECT * FROM t where t.a in (select a from t limit 1, 10)", true, "SELECT * FROM `t` WHERE `t`.`a` IN (SELECT `a` FROM `t` LIMIT 1,10)"},
		{"SELECT * FROM t where t.a in ((select a from t limit 1, 10))", true, "SELECT * FROM `t` WHERE `t`.`a` IN ((SELECT `a` FROM `t` LIMIT 1,10))"},
		{"SELECT * FROM t where t.a in ((select a from t limit 1, 10), 1)", true, "SELECT * FROM `t` WHERE `t`.`a` IN ((SELECT `a` FROM `t` LIMIT 1,10),1)"},
		{"select * from ((select a from t) t1 join t t2) join t3", true, "SELECT * FROM ((SELECT `a` FROM `t`) AS `t1` JOIN `t` AS `t2`) JOIN `t3`"},
		{"SELECT t1.a AS a FROM ((SELECT a FROM t) AS t1)", true, "SELECT `t1`.`a` AS `a` FROM (SELECT `a` FROM `t`) AS `t1`"},
		{"select count(*) from (select a, b from x1 union all select a, b from x3 union all (select x1.a, x3.b from (select * from x3 union all select * from x2) x3 left join x1 on x3.a = x1.b))", true, "SELECT COUNT(1) FROM (SELECT `a`,`b` FROM `x1` UNION ALL SELECT `a`,`b` FROM `x3` UNION ALL (SELECT `x1`.`a`,`x3`.`b` FROM (SELECT * FROM `x3` UNION ALL SELECT * FROM `x2`) AS `x3` LEFT JOIN `x1` ON `x3`.`a`=`x1`.`b`))"},
		{"(SELECT 1 a,3 b) UNION (SELECT 2,1) ORDER BY (SELECT 2)", true, "(SELECT 1 AS `a`,3 AS `b`) UNION (SELECT 2,1) ORDER BY (SELECT 2)"},
		{"((select * from t1)) union (select * from t1)", true, "(SELECT * FROM `t1`) UNION (SELECT * FROM `t1`)"},
		{"(((select * from t1))) union (select * from t1)", true, "(SELECT * FROM `t1`) UNION (SELECT * FROM `t1`)"},
		{"select * from (((select * from t1)) union (select * from t1) union (select * from t1)) a", true, "SELECT * FROM ((SELECT * FROM `t1`) UNION (SELECT * FROM `t1`) UNION (SELECT * FROM `t1`)) AS `a`"},
		{"SELECT COUNT(*) FROM plan_executions WHERE (EXISTS((SELECT * FROM triggers WHERE plan_executions.trigger_id=triggers.id AND triggers.type='CRON')))", true, "SELECT COUNT(1) FROM `plan_executions` WHERE (EXISTS (SELECT * FROM `triggers` WHERE `plan_executions`.`trigger_id`=`triggers`.`id` AND `triggers`.`type`=_UTF8MB4'CRON'))"},
		{"select exists((select 1));", true, "SELECT EXISTS (SELECT 1)"},
		{"select * from ((SELECT 1 a,3 b) UNION (SELECT 2,1) ORDER BY (SELECT 2)) t order by a,b", true, "SELECT * FROM ((SELECT 1 AS `a`,3 AS `b`) UNION (SELECT 2,1) ORDER BY (SELECT 2)) AS `t` ORDER BY `a`,`b`"},
		{"select (select * from t1 where a != t.a union all (select * from t2 where a != t.a) order by a limit 1) from t1 t", true, "SELECT (SELECT * FROM `t1` WHERE `a`!=`t`.`a` UNION ALL (SELECT * FROM `t2` WHERE `a`!=`t`.`a`) ORDER BY `a` LIMIT 1) FROM `t1` AS `t`"},
		{"(WITH v0 AS (SELECT TRUE) (SELECT 'abc' EXCEPT (SELECT TRUE)))", true, "WITH `v0` AS (SELECT TRUE) (SELECT _UTF8MB4'abc' EXCEPT (SELECT TRUE))"},
	}
	RunTest(t, table, false)

	tests := []struct {
		input string
		text  string
	}{
		{"SELECT 1 > (select 1)", "select 1"},
		{"SELECT 1 > (select 1 union select 2)", "select 1 union select 2"},
	}
	p := parser.New()
	for _, tbl := range tests {
		stmt, err := p.ParseOneStmt(tbl.input, "", "")
		require.NoError(t, err)
		stmt.Accept(&subqueryChecker{
			text: tbl.text,
			t:    t,
		})
	}
}

func TestSetOperator(t *testing.T) {
	table := []testCase{
		// union and union all
		{"select c1 from t1 union select c2 from t2", true, "SELECT `c1` FROM `t1` UNION SELECT `c2` FROM `t2`"},
		{"select c1 from t1 union (select c2 from t2)", true, "SELECT `c1` FROM `t1` UNION (SELECT `c2` FROM `t2`)"},
		{"select c1 from t1 union (select c2 from t2) order by c1", true, "SELECT `c1` FROM `t1` UNION (SELECT `c2` FROM `t2`) ORDER BY `c1`"},
		{"select c1 from t1 union select c2 from t2 order by c2", true, "SELECT `c1` FROM `t1` UNION SELECT `c2` FROM `t2` ORDER BY `c2`"},
		{"select c1 from t1 union (select c2 from t2) limit 1", true, "SELECT `c1` FROM `t1` UNION (SELECT `c2` FROM `t2`) LIMIT 1"},
		{"select c1 from t1 union (select c2 from t2) limit 1, 1", true, "SELECT `c1` FROM `t1` UNION (SELECT `c2` FROM `t2`) LIMIT 1,1"},
		{"select c1 from t1 union (select c2 from t2) order by c1 limit 1", true, "SELECT `c1` FROM `t1` UNION (SELECT `c2` FROM `t2`) ORDER BY `c1` LIMIT 1"},
		{"(select c1 from t1) union distinct select c2 from t2", true, "(SELECT `c1` FROM `t1`) UNION SELECT `c2` FROM `t2`"},
		{"(select c1 from t1) union distinctrow select c2 from t2", true, "(SELECT `c1` FROM `t1`) UNION SELECT `c2` FROM `t2`"},
		{"(select c1 from t1) union all select c2 from t2", true, "(SELECT `c1` FROM `t1`) UNION ALL SELECT `c2` FROM `t2`"},
		{"(select c1 from t1) union distinct all select c2 from t2", false, ""},
		{"(select c1 from t1) union distinctrow all select c2 from t2", false, ""},
		{"(select c1 from t1) union (select c2 from t2) order by c1 union select c3 from t3", false, ""},
		{"(select c1 from t1) union (select c2 from t2) limit 1 union select c3 from t3", false, ""},
		{"(select c1 from t1) union select c2 from t2 union (select c3 from t3) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) UNION SELECT `c2` FROM `t2` UNION (SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"select (select 1 union select 1) as a", true, "SELECT (SELECT 1 UNION SELECT 1) AS `a`"},
		{"select * from (select 1 union select 2) as a", true, "SELECT * FROM (SELECT 1 UNION SELECT 2) AS `a`"},
		{"insert into t select c1 from t1 union select c2 from t2", true, "INSERT INTO `t` SELECT `c1` FROM `t1` UNION SELECT `c2` FROM `t2`"},
		{"insert into t (c) select c1 from t1 union select c2 from t2", true, "INSERT INTO `t` (`c`) SELECT `c1` FROM `t1` UNION SELECT `c2` FROM `t2`"},
		{"select 2 as a from dual union select 1 as b from dual order by a", true, "SELECT 2 AS `a` UNION SELECT 1 AS `b` ORDER BY `a`"},
		{"table t1 union table t2", true, "TABLE `t1` UNION TABLE `t2`"},
		{"table t1 union (table t2)", true, "TABLE `t1` UNION (TABLE `t2`)"},
		{"table t1 union select * from t2", true, "TABLE `t1` UNION SELECT * FROM `t2`"},
		{"select * from t1 union table t2", true, "SELECT * FROM `t1` UNION TABLE `t2`"},
		{"table t1 union (select c2 from t2) order by c1 limit 1", true, "TABLE `t1` UNION (SELECT `c2` FROM `t2`) ORDER BY `c1` LIMIT 1"},
		{"select c1 from t1 union (table t2) order by c1 limit 1", true, "SELECT `c1` FROM `t1` UNION (TABLE `t2`) ORDER BY `c1` LIMIT 1"},
		{"(select c1 from t1) union table t2 union (select c3 from t3) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) UNION TABLE `t2` UNION (SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"(table t1) union select c2 from t2 union (table t3) order by c1 limit 1", true, "(TABLE `t1`) UNION SELECT `c2` FROM `t2` UNION (TABLE `t3`) ORDER BY `c1` LIMIT 1"},
		{"values row(1,-2,3), row(5,7,9) union values row(1,-2,3), row(5,7,9)", true, "VALUES ROW(1,-2,3), ROW(5,7,9) UNION VALUES ROW(1,-2,3), ROW(5,7,9)"},
		{"values row(1,-2,3), row(5,7,9) union (values row(1,-2,3), row(5,7,9))", true, "VALUES ROW(1,-2,3), ROW(5,7,9) UNION (VALUES ROW(1,-2,3), ROW(5,7,9))"},
		{"values row(1,-2,3), row(5,7,9) union select * from t", true, "VALUES ROW(1,-2,3), ROW(5,7,9) UNION SELECT * FROM `t`"},
		{"values row(1,-2,3), row(5,7,9) union table t", true, "VALUES ROW(1,-2,3), ROW(5,7,9) UNION TABLE `t`"},
		{"select * from t union values row(1,-2,3), row(5,7,9)", true, "SELECT * FROM `t` UNION VALUES ROW(1,-2,3), ROW(5,7,9)"},
		{"table t union values row(1,-2,3), row(5,7,9)", true, "TABLE `t` UNION VALUES ROW(1,-2,3), ROW(5,7,9)"},
		// except
		{"select c1 from t1 except select c2 from t2", true, "SELECT `c1` FROM `t1` EXCEPT SELECT `c2` FROM `t2`"},
		{"select c1 from t1 except (select c2 from t2)", true, "SELECT `c1` FROM `t1` EXCEPT (SELECT `c2` FROM `t2`)"},
		{"select c1 from t1 except (select c2 from t2) order by c1", true, "SELECT `c1` FROM `t1` EXCEPT (SELECT `c2` FROM `t2`) ORDER BY `c1`"},
		{"select c1 from t1 except select c2 from t2 order by c2", true, "SELECT `c1` FROM `t1` EXCEPT SELECT `c2` FROM `t2` ORDER BY `c2`"},
		{"select c1 from t1 except (select c2 from t2) limit 1", true, "SELECT `c1` FROM `t1` EXCEPT (SELECT `c2` FROM `t2`) LIMIT 1"},
		{"select c1 from t1 except (select c2 from t2) limit 1, 1", true, "SELECT `c1` FROM `t1` EXCEPT (SELECT `c2` FROM `t2`) LIMIT 1,1"},
		{"select c1 from t1 except (select c2 from t2) order by c1 limit 1", true, "SELECT `c1` FROM `t1` EXCEPT (SELECT `c2` FROM `t2`) ORDER BY `c1` LIMIT 1"},
		{"(select c1 from t1) except (select c2 from t2) order by c1 except select c3 from t3", false, ""},
		{"(select c1 from t1) except (select c2 from t2) limit 1 except select c3 from t3", false, ""},
		{"(select c1 from t1) except select c2 from t2 except (select c3 from t3) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) EXCEPT SELECT `c2` FROM `t2` EXCEPT (SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"select (select 1 except select 1) as a", true, "SELECT (SELECT 1 EXCEPT SELECT 1) AS `a`"},
		{"select * from (select 1 except select 2) as a", true, "SELECT * FROM (SELECT 1 EXCEPT SELECT 2) AS `a`"},
		{"insert into t select c1 from t1 except select c2 from t2", true, "INSERT INTO `t` SELECT `c1` FROM `t1` EXCEPT SELECT `c2` FROM `t2`"},
		{"insert into t (c) select c1 from t1 except select c2 from t2", true, "INSERT INTO `t` (`c`) SELECT `c1` FROM `t1` EXCEPT SELECT `c2` FROM `t2`"},
		{"select 2 as a from dual except select 1 as b from dual order by a", true, "SELECT 2 AS `a` EXCEPT SELECT 1 AS `b` ORDER BY `a`"},
		{"table t1 except table t2", true, "TABLE `t1` EXCEPT TABLE `t2`"},
		{"table t1 except (table t2)", true, "TABLE `t1` EXCEPT (TABLE `t2`)"},
		{"table t1 except select * from t2", true, "TABLE `t1` EXCEPT SELECT * FROM `t2`"},
		{"select * from t1 except table t2", true, "SELECT * FROM `t1` EXCEPT TABLE `t2`"},
		{"table t1 except (select c2 from t2) order by c1 limit 1", true, "TABLE `t1` EXCEPT (SELECT `c2` FROM `t2`) ORDER BY `c1` LIMIT 1"},
		{"select c1 from t1 except (table t2) order by c1 limit 1", true, "SELECT `c1` FROM `t1` EXCEPT (TABLE `t2`) ORDER BY `c1` LIMIT 1"},
		{"(select c1 from t1) except table t2 except (select c3 from t3) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) EXCEPT TABLE `t2` EXCEPT (SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"(table t1) except select c2 from t2 except (table t3) order by c1 limit 1", true, "(TABLE `t1`) EXCEPT SELECT `c2` FROM `t2` EXCEPT (TABLE `t3`) ORDER BY `c1` LIMIT 1"},
		{"values row(1,-2,3), row(5,7,9) except values row(1,-2,3), row(5,7,9)", true, "VALUES ROW(1,-2,3), ROW(5,7,9) EXCEPT VALUES ROW(1,-2,3), ROW(5,7,9)"},
		{"values row(1,-2,3), row(5,7,9) except (values row(1,-2,3), row(5,7,9))", true, "VALUES ROW(1,-2,3), ROW(5,7,9) EXCEPT (VALUES ROW(1,-2,3), ROW(5,7,9))"},
		{"values row(1,-2,3), row(5,7,9) except select * from t", true, "VALUES ROW(1,-2,3), ROW(5,7,9) EXCEPT SELECT * FROM `t`"},
		{"values row(1,-2,3), row(5,7,9) except table t", true, "VALUES ROW(1,-2,3), ROW(5,7,9) EXCEPT TABLE `t`"},
		{"select * from t except values row(1,-2,3), row(5,7,9)", true, "SELECT * FROM `t` EXCEPT VALUES ROW(1,-2,3), ROW(5,7,9)"},
		{"table t except values row(1,-2,3), row(5,7,9)", true, "TABLE `t` EXCEPT VALUES ROW(1,-2,3), ROW(5,7,9)"},
		// intersect
		{"select c1 from t1 intersect select c2 from t2", true, "SELECT `c1` FROM `t1` INTERSECT SELECT `c2` FROM `t2`"},
		{"select c1 from t1 intersect (select c2 from t2)", true, "SELECT `c1` FROM `t1` INTERSECT (SELECT `c2` FROM `t2`)"},
		{"select c1 from t1 intersect (select c2 from t2) order by c1", true, "SELECT `c1` FROM `t1` INTERSECT (SELECT `c2` FROM `t2`) ORDER BY `c1`"},
		{"select c1 from t1 intersect select c2 from t2 order by c2", true, "SELECT `c1` FROM `t1` INTERSECT SELECT `c2` FROM `t2` ORDER BY `c2`"},
		{"select c1 from t1 intersect (select c2 from t2) limit 1", true, "SELECT `c1` FROM `t1` INTERSECT (SELECT `c2` FROM `t2`) LIMIT 1"},
		{"select c1 from t1 intersect (select c2 from t2) limit 1, 1", true, "SELECT `c1` FROM `t1` INTERSECT (SELECT `c2` FROM `t2`) LIMIT 1,1"},
		{"select c1 from t1 intersect (select c2 from t2) order by c1 limit 1", true, "SELECT `c1` FROM `t1` INTERSECT (SELECT `c2` FROM `t2`) ORDER BY `c1` LIMIT 1"},
		{"(select c1 from t1) intersect (select c2 from t2) order by c1 intersect select c3 from t3", false, ""},
		{"(select c1 from t1) intersect (select c2 from t2) limit 1 intersect select c3 from t3", false, ""},
		{"(select c1 from t1) intersect select c2 from t2 intersect (select c3 from t3) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) INTERSECT SELECT `c2` FROM `t2` INTERSECT (SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"select (select 1 intersect select 1) as a", true, "SELECT (SELECT 1 INTERSECT SELECT 1) AS `a`"},
		{"select * from (select 1 intersect select 2) as a", true, "SELECT * FROM (SELECT 1 INTERSECT SELECT 2) AS `a`"},
		{"insert into t select c1 from t1 intersect select c2 from t2", true, "INSERT INTO `t` SELECT `c1` FROM `t1` INTERSECT SELECT `c2` FROM `t2`"},
		{"insert into t (c) select c1 from t1 intersect select c2 from t2", true, "INSERT INTO `t` (`c`) SELECT `c1` FROM `t1` INTERSECT SELECT `c2` FROM `t2`"},
		{"select 2 as a from dual intersect select 1 as b from dual order by a", true, "SELECT 2 AS `a` INTERSECT SELECT 1 AS `b` ORDER BY `a`"},
		{"table t1 intersect table t2", true, "TABLE `t1` INTERSECT TABLE `t2`"},
		{"table t1 intersect (table t2)", true, "TABLE `t1` INTERSECT (TABLE `t2`)"},
		{"table t1 intersect select * from t2", true, "TABLE `t1` INTERSECT SELECT * FROM `t2`"},
		{"select * from t1 intersect table t2", true, "SELECT * FROM `t1` INTERSECT TABLE `t2`"},
		{"table t1 intersect (select c2 from t2) order by c1 limit 1", true, "TABLE `t1` INTERSECT (SELECT `c2` FROM `t2`) ORDER BY `c1` LIMIT 1"},
		{"select c1 from t1 intersect (table t2) order by c1 limit 1", true, "SELECT `c1` FROM `t1` INTERSECT (TABLE `t2`) ORDER BY `c1` LIMIT 1"},
		{"(select c1 from t1) intersect table t2 intersect (select c3 from t3) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) INTERSECT TABLE `t2` INTERSECT (SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"(table t1) intersect select c2 from t2 intersect (table t3) order by c1 limit 1", true, "(TABLE `t1`) INTERSECT SELECT `c2` FROM `t2` INTERSECT (TABLE `t3`) ORDER BY `c1` LIMIT 1"},
		{"values row(1,-2,3), row(5,7,9) intersect values row(1,-2,3), row(5,7,9)", true, "VALUES ROW(1,-2,3), ROW(5,7,9) INTERSECT VALUES ROW(1,-2,3), ROW(5,7,9)"},
		{"values row(1,-2,3), row(5,7,9) intersect (values row(1,-2,3), row(5,7,9))", true, "VALUES ROW(1,-2,3), ROW(5,7,9) INTERSECT (VALUES ROW(1,-2,3), ROW(5,7,9))"},
		{"values row(1,-2,3), row(5,7,9) intersect select * from t", true, "VALUES ROW(1,-2,3), ROW(5,7,9) INTERSECT SELECT * FROM `t`"},
		{"values row(1,-2,3), row(5,7,9) intersect table t", true, "VALUES ROW(1,-2,3), ROW(5,7,9) INTERSECT TABLE `t`"},
		{"select * from t intersect values row(1,-2,3), row(5,7,9)", true, "SELECT * FROM `t` INTERSECT VALUES ROW(1,-2,3), ROW(5,7,9)"},
		{"table t intersect values row(1,-2,3), row(5,7,9)", true, "TABLE `t` INTERSECT VALUES ROW(1,-2,3), ROW(5,7,9)"},
		// mixture of union, except and intersect
		{"(select c1 from t1) intersect select c2 from t2 union (select c3 from t3) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) INTERSECT SELECT `c2` FROM `t2` UNION (SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"(select c1 from t1) union all select c2 from t2 except (select c3 from t3) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) UNION ALL SELECT `c2` FROM `t2` EXCEPT (SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"(select c1 from t1) except select c2 from t2 intersect (select c3 from t3) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) EXCEPT SELECT `c2` FROM `t2` INTERSECT (SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"select 1 union distinct select 1 except select 1 intersect select 1", true, "SELECT 1 UNION SELECT 1 EXCEPT SELECT 1 INTERSECT SELECT 1"},
		// mixture of union, except and intersect with parentheses
		{"(select c1 from t1) intersect all (select c2 from t2 union (select c3 from t3)) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) INTERSECT ALL (SELECT `c2` FROM `t2` UNION (SELECT `c3` FROM `t3`)) ORDER BY `c1` LIMIT 1"},
		{"(select c1 from t1) union all (select c2 from t2 except select c3 from t3) order by c1 limit 1", true, "(SELECT `c1` FROM `t1`) UNION ALL (SELECT `c2` FROM `t2` EXCEPT SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"((select c1 from t1) except select c2 from t2) intersect all (select c3 from t3) order by c1 limit 1", true, "((SELECT `c1` FROM `t1`) EXCEPT SELECT `c2` FROM `t2`) INTERSECT ALL (SELECT `c3` FROM `t3`) ORDER BY `c1` LIMIT 1"},
		{"select 1 union distinct (select 1 except all select 1 intersect select 1)", true, "SELECT 1 UNION (SELECT 1 EXCEPT ALL SELECT 1 INTERSECT SELECT 1)"},

		// https://github.com/pingcap/tidb/issues/49874
		{"select * from a where PK = 0 union all (select * from b where PK = 0 union all (select * from b where PK != 0) order by pk limit 1)", true,
			"SELECT * FROM `a` WHERE `PK`=0 UNION ALL (SELECT * FROM `b` WHERE `PK`=0 UNION ALL (SELECT * FROM `b` WHERE `PK`!=0) ORDER BY `pk` LIMIT 1)"},
		{"select * from a where PK = 0 union all (select * from b where PK = 0 union all (select * from b where PK != 0) order by pk limit 1) order by pk limit 2", true,
			"SELECT * FROM `a` WHERE `PK`=0 UNION ALL (SELECT * FROM `b` WHERE `PK`=0 UNION ALL (SELECT * FROM `b` WHERE `PK`!=0) ORDER BY `pk` LIMIT 1) ORDER BY `pk` LIMIT 2"},
		{"(select * from b where pk= 0 union all (select * from b where pk !=0) order by pk limit 1) order by pk limit 2", true,
			"(SELECT * FROM `b` WHERE `pk`=0 UNION ALL (SELECT * FROM `b` WHERE `pk`!=0) ORDER BY `pk` LIMIT 1) ORDER BY `pk` LIMIT 2"},
		{"(select * from b where pk= 0 union all (select * from b where pk !=0) order by pk limit 1) order by pk", true,
			"(SELECT * FROM `b` WHERE `pk`=0 UNION ALL (SELECT * FROM `b` WHERE `pk`!=0) ORDER BY `pk` LIMIT 1) ORDER BY `pk`"},
	}
	RunTest(t, table, false)
}

func checkOrderBy(t *testing.T, s ast.Node, hasOrderBy []bool, i int) int {
	switch x := s.(type) {
	case *ast.SelectStmt:
		require.Equal(t, hasOrderBy[i], x.OrderBy != nil)
		return i + 1
	case *ast.SetOprSelectList:
		for _, sel := range x.Selects {
			i = checkOrderBy(t, sel, hasOrderBy, i)
		}
		return i
	}
	return i
}

func TestUnionOrderBy(t *testing.T) {
	p := parser.New()
	p.EnableWindowFunc(false)

	tests := []struct {
		src        string
		hasOrderBy []bool
	}{
		{"select 2 as a from dual union select 1 as b from dual order by a", []bool{false, false, true}},
		{"select 2 as a from dual union (select 1 as b from dual order by a)", []bool{false, true, false}},
		{"(select 2 as a from dual order by a) union select 1 as b from dual order by a", []bool{true, false, true}},
		{"select 1 a, 2 b from dual order by a", []bool{true}},
		{"select 1 a, 2 b from dual", []bool{false}},
	}

	for _, tbl := range tests {
		stmt, _, err := p.Parse(tbl.src, "", "")
		require.NoError(t, err)
		us, ok := stmt[0].(*ast.SetOprStmt)
		if ok {
			var i int
			for _, s := range us.SelectList.Selects {
				i = checkOrderBy(t, s, tbl.hasOrderBy, i)
			}
			require.Equal(t, tbl.hasOrderBy[i], us.OrderBy != nil)
		}
		ss, ok := stmt[0].(*ast.SelectStmt)
		if ok {
			require.Equal(t, tbl.hasOrderBy[0], ss.OrderBy != nil)
		}
	}
}

func TestLikeEscape(t *testing.T) {
	table := []testCase{
		// for like escape
		{`select "abc_" like "abc\\_" escape ''`, true, "SELECT _UTF8MB4'abc_' LIKE _UTF8MB4'abc\\_'"},
		{`select "abc_" like "abc\\_" escape '\\'`, true, "SELECT _UTF8MB4'abc_' LIKE _UTF8MB4'abc\\_'"},
		{`select "abc_" like "abc\\_" escape '||'`, false, ""},
		{`select "abc" like "escape" escape '+'`, true, "SELECT _UTF8MB4'abc' LIKE _UTF8MB4'escape' ESCAPE '+'"},
		{"select '''_' like '''_' escape ''''", true, "SELECT _UTF8MB4'''_' LIKE _UTF8MB4'''_' ESCAPE ''''"},
	}

	RunTest(t, table, false)
}

func TestLockUnlockTables(t *testing.T) {
	table := []testCase{
		{`UNLOCK TABLES;`, true, "UNLOCK TABLES"},
		{`LOCK TABLES t1 READ;`, true, "LOCK TABLES `t1` READ"},
		{`LOCK TABLES t1 READ LOCAL;`, true, "LOCK TABLES `t1` READ LOCAL"},
		{`show table status like 't'`, true, "SHOW TABLE STATUS LIKE _UTF8MB4't'"},
		{`LOCK TABLES t2 WRITE`, true, "LOCK TABLES `t2` WRITE"},
		{`LOCK TABLES t2 WRITE LOCAL;`, true, "LOCK TABLES `t2` WRITE LOCAL"},
		{`LOCK TABLES t1 WRITE, t2 READ;`, true, "LOCK TABLES `t1` WRITE, `t2` READ"},
		{`LOCK TABLES t1 WRITE LOCAL, t2 READ LOCAL;`, true, "LOCK TABLES `t1` WRITE LOCAL, `t2` READ LOCAL"},

		// for unlock table and lock table
		{`UNLOCK TABLE;`, true, "UNLOCK TABLES"},
		{`LOCK TABLE t1 READ;`, true, "LOCK TABLES `t1` READ"},
		{`LOCK TABLE t1 READ LOCAL;`, true, "LOCK TABLES `t1` READ LOCAL"},
		{`show table status like 't'`, true, "SHOW TABLE STATUS LIKE _UTF8MB4't'"},
		{`LOCK TABLE t2 WRITE`, true, "LOCK TABLES `t2` WRITE"},
		{`LOCK TABLE t2 WRITE LOCAL;`, true, "LOCK TABLES `t2` WRITE LOCAL"},
		{`LOCK TABLE t1 WRITE, t2 READ;`, true, "LOCK TABLES `t1` WRITE, `t2` READ"},

		// for cleanup table lock.
		{"ADMIN CLEANUP TABLE LOCK", false, ""},
		{"ADMIN CLEANUP TABLE LOCK t", true, "ADMIN CLEANUP TABLE LOCK `t`"},
		{"ADMIN CLEANUP TABLE LOCK t1,t2", true, "ADMIN CLEANUP TABLE LOCK `t1`, `t2`"},

		// For alter table read only/write.
		{"ALTER TABLE t READ ONLY", true, "ALTER TABLE `t` READ ONLY"},
		{"ALTER TABLE t READ WRITE", true, "ALTER TABLE `t` READ WRITE"},
	}

	RunTest(t, table, false)
}

func TestWithRollup(t *testing.T) {
	table := []testCase{
		{`select * from t group by a, b rollup`, false, ""},
		{`select * from t group by a, b with rollup`, true, "SELECT * FROM `t` GROUP BY `a`,`b` WITH ROLLUP"},
		// should be ERROR 1241 (21000): Operand should contain 1 column(s) in runtime.
		{`select * from t group by (a, b) with rollup`, true, "SELECT * FROM `t` GROUP BY ROW(`a`,`b`) WITH ROLLUP"},
		{`select * from t group by (a+b) with rollup`, true, "SELECT * FROM `t` GROUP BY (`a`+`b`) WITH ROLLUP"},
	}
	RunTest(t, table, false)
}

func TestIndexHint(t *testing.T) {
	table := []testCase{
		{`select * from t use index (primary)`, true, "SELECT * FROM `t` USE INDEX (`primary`)"},
		{"select * from t use index (`primary`)", true, "SELECT * FROM `t` USE INDEX (`primary`)"},
		{`select * from t use index ();`, true, "SELECT * FROM `t` USE INDEX ()"},
		{`select * from t use index (idx);`, true, "SELECT * FROM `t` USE INDEX (`idx`)"},
		{`select * from t use index (idx1, idx2);`, true, "SELECT * FROM `t` USE INDEX (`idx1`, `idx2`)"},
		{`select * from t ignore key (idx1)`, true, "SELECT * FROM `t` IGNORE INDEX (`idx1`)"},
		{`select * from t force index for join (idx1)`, true, "SELECT * FROM `t` FORCE INDEX FOR JOIN (`idx1`)"},
		{`select * from t use index for order by (idx1)`, true, "SELECT * FROM `t` USE INDEX FOR ORDER BY (`idx1`)"},
		{`select * from t force index for group by (idx1)`, true, "SELECT * FROM `t` FORCE INDEX FOR GROUP BY (`idx1`)"},
		{`select * from t use index for group by (idx1) use index for order by (idx2), t2`, true, "SELECT * FROM (`t` USE INDEX FOR GROUP BY (`idx1`) USE INDEX FOR ORDER BY (`idx2`)) JOIN `t2`"},
	}

	RunTest(t, table, false)
}

func TestPriority(t *testing.T) {
	table := []testCase{
		{`select high_priority * from t`, true, "SELECT HIGH_PRIORITY * FROM `t`"},
		{`select low_priority * from t`, true, "SELECT LOW_PRIORITY * FROM `t`"},
		{`select delayed * from t`, true, "SELECT DELAYED * FROM `t`"},
		{`insert high_priority into t values (1)`, true, "INSERT HIGH_PRIORITY INTO `t` VALUES (1)"},
		{`insert LOW_PRIORITY into t values (1)`, true, "INSERT LOW_PRIORITY INTO `t` VALUES (1)"},
		{`insert delayed into t values (1)`, true, "INSERT DELAYED INTO `t` VALUES (1)"},
		{`update low_priority t set a = 2`, true, "UPDATE LOW_PRIORITY `t` SET `a`=2"},
		{`update high_priority t set a = 2`, true, "UPDATE HIGH_PRIORITY `t` SET `a`=2"},
		{`update delayed t set a = 2`, true, "UPDATE DELAYED `t` SET `a`=2"},
		{`delete low_priority from t where a = 2`, true, "DELETE LOW_PRIORITY FROM `t` WHERE `a`=2"},
		{`delete high_priority from t where a = 2`, true, "DELETE HIGH_PRIORITY FROM `t` WHERE `a`=2"},
		{`delete delayed from t where a = 2`, true, "DELETE DELAYED FROM `t` WHERE `a`=2"},
		{`replace high_priority into t values (1)`, true, "REPLACE HIGH_PRIORITY INTO `t` VALUES (1)"},
		{`replace LOW_PRIORITY into t values (1)`, true, "REPLACE LOW_PRIORITY INTO `t` VALUES (1)"},
		{`replace delayed into t values (1)`, true, "REPLACE DELAYED INTO `t` VALUES (1)"},
	}
	RunTest(t, table, false)

	p := parser.New()
	stmt, _, err := p.Parse("select HIGH_PRIORITY * from t", "", "")
	require.NoError(t, err)
	sel := stmt[0].(*ast.SelectStmt)
	require.Equal(t, mysql.HighPriority, sel.SelectStmtOpts.Priority)
}

func TestSQLResult(t *testing.T) {
	table := []testCase{
		{`select SQL_BIG_RESULT c1 from t group by c1`, true, "SELECT SQL_BIG_RESULT `c1` FROM `t` GROUP BY `c1`"},
		{`select SQL_SMALL_RESULT c1 from t group by c1`, true, "SELECT SQL_SMALL_RESULT `c1` FROM `t` GROUP BY `c1`"},
		{`select SQL_BUFFER_RESULT * from t`, true, "SELECT SQL_BUFFER_RESULT * FROM `t`"},
		{`select sql_small_result sql_big_result sql_buffer_result 1`, true, "SELECT SQL_SMALL_RESULT SQL_BIG_RESULT SQL_BUFFER_RESULT 1"},
		{`select STRAIGHT_JOIN SQL_SMALL_RESULT * from t`, true, "SELECT SQL_SMALL_RESULT STRAIGHT_JOIN * FROM `t`"},
		{`select SQL_CALC_FOUND_ROWS DISTINCT * from t`, true, "SELECT SQL_CALC_FOUND_ROWS DISTINCT * FROM `t`"},
	}

	RunTest(t, table, false)
}

func TestSQLNoCache(t *testing.T) {
	table := []testCase{
		{`select SQL_NO_CACHE * from t`, false, ""},
		{`select SQL_CACHE * from t`, true, "SELECT * FROM `t`"},
		{`select * from t`, true, "SELECT * FROM `t`"},
	}

	p := parser.New()
	for _, tbl := range table {
		stmt, _, err := p.Parse(tbl.src, "", "")
		require.NoError(t, err)

		sel := stmt[0].(*ast.SelectStmt)
		require.Equal(t, tbl.ok, sel.SelectStmtOpts.SQLCache)
	}
}

func TestEscape(t *testing.T) {
	table := []testCase{
		{`select """;`, false, ""},
		{`select """";`, true, "SELECT _UTF8MB4'\"'"},
		{`select "汉字";`, true, "SELECT _UTF8MB4'汉字'"},
		{`select 'abc"def';`, true, "SELECT _UTF8MB4'abc\"def'"},
		{`select 'a\r\n';`, true, "SELECT _UTF8MB4'a\r\n'"},
		{`select "\a\r\n"`, true, "SELECT _UTF8MB4'a\r\n'"},
		{`select "\xFF"`, true, "SELECT _UTF8MB4'xFF'"},
	}
	RunTest(t, table, false)
}

func TestExplain(t *testing.T) {
	table := []testCase{
		{"explain select c1 from t1", true, "EXPLAIN FORMAT = 'row' SELECT `c1` FROM `t1`"},
		{"explain delete t1, t2 from t1 inner join t2 inner join t3 where t1.id=t2.id and t2.id=t3.id;", true, "EXPLAIN FORMAT = 'row' DELETE `t1`,`t2` FROM (`t1` JOIN `t2`) JOIN `t3` WHERE `t1`.`id`=`t2`.`id` AND `t2`.`id`=`t3`.`id`"},
		{"explain insert into t values (1), (2), (3)", true, "EXPLAIN FORMAT = 'row' INSERT INTO `t` VALUES (1),(2),(3)"},
		{"explain replace into foo values (1 || 2)", true, "EXPLAIN FORMAT = 'row' REPLACE INTO `foo` VALUES (1 OR 2)"},
		{"explain update t set id = id + 1 order by id desc;", true, "EXPLAIN FORMAT = 'row' UPDATE `t` SET `id`=`id`+1 ORDER BY `id` DESC"},
		{"explain select c1 from t1 union (select c2 from t2) limit 1, 1", true, "EXPLAIN FORMAT = 'row' SELECT `c1` FROM `t1` UNION (SELECT `c2` FROM `t2`) LIMIT 1,1"},
		{`explain format = "row" select c1 from t1 union (select c2 from t2) limit 1, 1`, true, "EXPLAIN FORMAT = 'row' SELECT `c1` FROM `t1` UNION (SELECT `c2` FROM `t2`) LIMIT 1,1"},
		{"explain format = 'brief' select * from t", true, "EXPLAIN FORMAT = 'brief' SELECT * FROM `t`"},
		{"DESC SCHE.TABL", true, "DESC `SCHE`.`TABL`"},
		{"DESC SCHE.TABL COLUM", true, "DESC `SCHE`.`TABL` `COLUM`"},
		{"DESCRIBE SCHE.TABL COLUM", true, "DESC `SCHE`.`TABL` `COLUM`"},
		{"EXPLAIN ANALYZE SELECT 1", true, "EXPLAIN ANALYZE SELECT 1"},
		{"EXPLAIN ANALYZE format=VERBOSE SELECT 1", true, "EXPLAIN ANALYZE FORMAT = 'VERBOSE' SELECT 1"},
		{"EXPLAIN ANALYZE format=TRUE_CARD_COST SELECT 1", true, "EXPLAIN ANALYZE FORMAT = 'TRUE_CARD_COST' SELECT 1"},
		{"EXPLAIN ANALYZE format='VERBOSE' SELECT 1", true, "EXPLAIN ANALYZE FORMAT = 'VERBOSE' SELECT 1"},
		{"EXPLAIN ANALYZE format='TRUE_CARD_COST' SELECT 1", true, "EXPLAIN ANALYZE FORMAT = 'TRUE_CARD_COST' SELECT 1"},
		{"EXPLAIN FORMAT = 'dot' SELECT 1", true, "EXPLAIN FORMAT = 'dot' SELECT 1"},
		{"EXPLAIN FORMAT = DOT SELECT 1", true, "EXPLAIN FORMAT = 'DOT' SELECT 1"},
		{"EXPLAIN FORMAT = 'row' SELECT 1", true, "EXPLAIN FORMAT = 'row' SELECT 1"},
		{"EXPLAIN FORMAT = 'ROW' SELECT 1", true, "EXPLAIN FORMAT = 'ROW' SELECT 1"},
		{"EXPLAIN FORMAT = 'BRIEF' SELECT 1", true, "EXPLAIN FORMAT = 'BRIEF' SELECT 1"},
		{"EXPLAIN FORMAT = BRIEF SELECT 1", true, "EXPLAIN FORMAT = 'BRIEF' SELECT 1"},
		{"EXPLAIN FORMAT = 'verbose' SELECT 1", true, "EXPLAIN FORMAT = 'verbose' SELECT 1"},
		{"EXPLAIN FORMAT = 'VERBOSE' SELECT 1", true, "EXPLAIN FORMAT = 'VERBOSE' SELECT 1"},
		{"EXPLAIN FORMAT = VERBOSE SELECT 1", true, "EXPLAIN FORMAT = 'VERBOSE' SELECT 1"},
		{"EXPLAIN SELECT 1", true, "EXPLAIN FORMAT = 'row' SELECT 1"},
		{"EXPLAIN FOR CONNECTION 1", true, "EXPLAIN FORMAT = 'row' FOR CONNECTION 1"},
		{"EXPLAIN FOR connection 42", true, "EXPLAIN FORMAT = 'row' FOR CONNECTION 42"},
		{"EXPLAIN FORMAT = 'dot' FOR CONNECTION 1", true, "EXPLAIN FORMAT = 'dot' FOR CONNECTION 1"},
		{"EXPLAIN FORMAT = DOT FOR CONNECTION 1", true, "EXPLAIN FORMAT = 'DOT' FOR CONNECTION 1"},
		{"EXPLAIN FORMAT = 'row' FOR connection 1", true, "EXPLAIN FORMAT = 'row' FOR CONNECTION 1"},
		{"EXPLAIN FORMAT = ROW FOR connection 1", true, "EXPLAIN FORMAT = 'ROW' FOR CONNECTION 1"},
		{"EXPLAIN FORMAT = TRADITIONAL FOR CONNECTION 1", true, "EXPLAIN FORMAT = 'TRADITIONAL' FOR CONNECTION 1"},
		{"EXPLAIN FORMAT = TRADITIONAL SELECT 1", true, "EXPLAIN FORMAT = 'TRADITIONAL' SELECT 1"},
		{"EXPLAIN FORMAT = BRIEF SELECT 1", true, "EXPLAIN FORMAT = 'BRIEF' SELECT 1"},
		{"EXPLAIN FORMAT = 'brief' SELECT 1", true, "EXPLAIN FORMAT = 'brief' SELECT 1"},
		{"EXPLAIN FORMAT = DOT SELECT 1", true, "EXPLAIN FORMAT = 'DOT' SELECT 1"},
		{"EXPLAIN FORMAT = 'dot' SELECT 1", true, "EXPLAIN FORMAT = 'dot' SELECT 1"},
		{"EXPLAIN FORMAT = VERBOSE SELECT 1", true, "EXPLAIN FORMAT = 'VERBOSE' SELECT 1"},
		{"EXPLAIN FORMAT = 'verbose' SELECT 1", true, "EXPLAIN FORMAT = 'verbose' SELECT 1"},
		{"EXPLAIN FORMAT = JSON FOR CONNECTION 1", true, "EXPLAIN FORMAT = 'JSON' FOR CONNECTION 1"},
		{"EXPLAIN FORMAT = JSON SELECT 1", true, "EXPLAIN FORMAT = 'JSON' SELECT 1"},
		{"EXPLAIN FORMAT = 'hint' SELECT 1", true, "EXPLAIN FORMAT = 'hint' SELECT 1"},
		{"EXPLAIN ANALYZE FORMAT = 'verbose' SELECT 1", true, "EXPLAIN ANALYZE FORMAT = 'verbose' SELECT 1"},
		{"EXPLAIN ANALYZE FORMAT = 'binary' SELECT 1", true, "EXPLAIN ANALYZE FORMAT = 'binary' SELECT 1"},
		{"EXPLAIN ALTER TABLE t1 ADD INDEX (a)", true, "EXPLAIN FORMAT = 'row' ALTER TABLE `t1` ADD INDEX(`a`)"},
		{"EXPLAIN ALTER TABLE t1 ADD a varchar(255)", true, "EXPLAIN FORMAT = 'row' ALTER TABLE `t1` ADD COLUMN `a` VARCHAR(255)"},
		{"EXPLAIN FORMAT = TIDB_JSON FOR CONNECTION 1", true, "EXPLAIN FORMAT = 'TIDB_JSON' FOR CONNECTION 1"},
		{"EXPLAIN FORMAT = tidb_json SELECT 1", true, "EXPLAIN FORMAT = 'tidb_json' SELECT 1"},
		{"EXPLAIN ANALYZE FORMAT = tidb_json SELECT 1", true, "EXPLAIN ANALYZE FORMAT = 'tidb_json' SELECT 1"},
		{"EXPLAIN 'sqldigest'", true, "EXPLAIN FORMAT = 'row' 'sqldigest'"},
		{"EXPLAIN ANALYZE 'sqldigest'", true, "EXPLAIN ANALYZE 'sqldigest'"},
		{"EXPLAIN format='json' 'sqldigest'", true, "EXPLAIN FORMAT = 'json' 'sqldigest'"},
		{"EXPLAIN ANALYZE format='json' 'sqldigest'", true, "EXPLAIN ANALYZE FORMAT = 'json' 'sqldigest'"},
	}
	RunTest(t, table, false)
}

func TestPrepare(t *testing.T) {
	table := []testCase{
		{"PREPARE pname FROM 'SELECT ?'", true, "PREPARE `pname` FROM 'SELECT ?'"},
		{"PREPARE pname FROM @test", true, "PREPARE `pname` FROM @`test`"},
		{"PREPARE `` FROM @test", true, "PREPARE `` FROM @`test`"},
	}
	RunTest(t, table, false)
}

func TestDeallocate(t *testing.T) {
	table := []testCase{
		{"DEALLOCATE PREPARE test", true, "DEALLOCATE PREPARE `test`"},
		{"DEALLOCATE PREPARE ``", true, "DEALLOCATE PREPARE ``"},
	}
	RunTest(t, table, false)
}

func TestExecute(t *testing.T) {
	table := []testCase{
		{"EXECUTE test", true, "EXECUTE `test`"},
		{"EXECUTE test USING @var1,@var2", true, "EXECUTE `test` USING @`var1`,@`var2`"},
		{"EXECUTE `` USING @var1,@var2", true, "EXECUTE `` USING @`var1`,@`var2`"},
	}
	RunTest(t, table, false)
}

func TestTrace(t *testing.T) {
	table := []testCase{
		{"trace begin", true, "TRACE START TRANSACTION"},
		{"trace commit", true, "TRACE COMMIT"},
		{"trace rollback", true, "TRACE ROLLBACK"},
		{"trace set a = 1", true, "TRACE SET @@SESSION.`a`=1"},
		{"trace select c1 from t1", true, "TRACE SELECT `c1` FROM `t1`"},
		{"trace delete t1, t2 from t1 inner join t2 inner join t3 where t1.id=t2.id and t2.id=t3.id;", true, "TRACE DELETE `t1`,`t2` FROM (`t1` JOIN `t2`) JOIN `t3` WHERE `t1`.`id`=`t2`.`id` AND `t2`.`id`=`t3`.`id`"},
		{"trace insert into t values (1), (2), (3)", true, "TRACE INSERT INTO `t` VALUES (1),(2),(3)"},
		{"trace replace into foo values (1 || 2)", true, "TRACE REPLACE INTO `foo` VALUES (1 OR 2)"},
		{"trace update t set id = id + 1 order by id desc;", true, "TRACE UPDATE `t` SET `id`=`id`+1 ORDER BY `id` DESC"},
		{"trace select c1 from t1 union (select c2 from t2) limit 1, 1", true, "TRACE SELECT `c1` FROM `t1` UNION (SELECT `c2` FROM `t2`) LIMIT 1,1"},
		{"trace format = 'row' select c1 from t1 union (select c2 from t2) limit 1, 1", true, "TRACE SELECT `c1` FROM `t1` UNION (SELECT `c2` FROM `t2`) LIMIT 1,1"},
		{"trace format = 'json' update t set id = id + 1 order by id desc;", true, "TRACE FORMAT = 'json' UPDATE `t` SET `id`=`id`+1 ORDER BY `id` DESC"},
		{"trace plan select c1 from t1", true, "TRACE PLAN SELECT `c1` FROM `t1`"},
		{"trace plan target = 'estimation' select c1 from t1", true, "TRACE PLAN TARGET = 'estimation' SELECT `c1` FROM `t1`"},
		{"trace plan target = 'arandomstring' select c1 from t1", true, "TRACE PLAN TARGET = 'arandomstring' SELECT `c1` FROM `t1`"},
	}
	RunTest(t, table, false)
}

func TestBinding(t *testing.T) {
	table := []testCase{
		{"create global binding for select * from t using select * from t use index(a)", true, "CREATE GLOBAL BINDING FOR SELECT * FROM `t` USING SELECT * FROM `t` USE INDEX (`a`)"},
		{"create session binding for select * from t using select * from t use index(a)", true, "CREATE SESSION BINDING FOR SELECT * FROM `t` USING SELECT * FROM `t` USE INDEX (`a`)"},
		{"drop global binding for select * from t", true, "DROP GLOBAL BINDING FOR SELECT * FROM `t`"},
		{"drop session binding for select * from t", true, "DROP SESSION BINDING FOR SELECT * FROM `t`"},
		{"drop global binding for select * from t using select * from t use index(a)", true, "DROP GLOBAL BINDING FOR SELECT * FROM `t` USING SELECT * FROM `t` USE INDEX (`a`)"},
		{"drop session binding for select * from t using select * from t use index(a)", true, "DROP SESSION BINDING FOR SELECT * FROM `t` USING SELECT * FROM `t` USE INDEX (`a`)"},
		{"show global bindings", true, "SHOW GLOBAL BINDINGS"},
		{"show session bindings", true, "SHOW SESSION BINDINGS"},
		{"set binding enabled for select * from t", true, "SET BINDING ENABLED FOR SELECT * FROM `t`"},
		{"set binding enabled for select * from t using select * from t use index(a)", true, "SET BINDING ENABLED FOR SELECT * FROM `t` USING SELECT * FROM `t` USE INDEX (`a`)"},
		{"set binding disabled for select * from t", true, "SET BINDING DISABLED FOR SELECT * FROM `t`"},
		{"set binding disabled for select * from t using select * from t use index(a)", true, "SET BINDING DISABLED FOR SELECT * FROM `t` USING SELECT * FROM `t` USE INDEX (`a`)"},
		{"create global binding for select * from t union all select * from t using select * from t use index(a) union all select * from t use index(a)", true, "CREATE GLOBAL BINDING FOR SELECT * FROM `t` UNION ALL SELECT * FROM `t` USING SELECT * FROM `t` USE INDEX (`a`) UNION ALL SELECT * FROM `t` USE INDEX (`a`)"},
		{"create session binding for select * from t union all select * from t using select * from t use index(a) union all select * from t use index(a)", true, "CREATE SESSION BINDING FOR SELECT * FROM `t` UNION ALL SELECT * FROM `t` USING SELECT * FROM `t` USE INDEX (`a`) UNION ALL SELECT * FROM `t` USE INDEX (`a`)"},
		{"drop global binding for select * from t union all select * from t using select * from t use index(a) union all select * from t use index(a)", true, "DROP GLOBAL BINDING FOR SELECT * FROM `t` UNION ALL SELECT * FROM `t` USING SELECT * FROM `t` USE INDEX (`a`) UNION ALL SELECT * FROM `t` USE INDEX (`a`)"},
		{"drop session binding for select * from t union all select * from t using select * from t use index(a) union all select * from t use index(a)", true, "DROP SESSION BINDING FOR SELECT * FROM `t` UNION ALL SELECT * FROM `t` USING SELECT * FROM `t` USE INDEX (`a`) UNION ALL SELECT * FROM `t` USE INDEX (`a`)"},
		{"drop global binding for select * from t union all select * from t", true, "DROP GLOBAL BINDING FOR SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create session binding for select 1 union select 2 intersect select 3 using select 1 union select 2 intersect select 3", true, "CREATE SESSION BINDING FOR SELECT 1 UNION SELECT 2 INTERSECT SELECT 3 USING SELECT 1 UNION SELECT 2 INTERSECT SELECT 3"},
		{"drop session binding for select 1 union select 2 intersect select 3 using select 1 union select 2 intersect select 3", true, "DROP SESSION BINDING FOR SELECT 1 UNION SELECT 2 INTERSECT SELECT 3 USING SELECT 1 UNION SELECT 2 INTERSECT SELECT 3"},
		{"drop session binding for select 1 union select 2 intersect select 3", true, "DROP SESSION BINDING FOR SELECT 1 UNION SELECT 2 INTERSECT SELECT 3"},
		// Use wildcards when creating binding
		{"create global binding using select * from *.t1", true, "CREATE GLOBAL BINDING FOR SELECT * FROM `*`.`t1` USING SELECT * FROM `*`.`t1`"},
		{"create global binding using select * from *.t1 where t1.a > (select max(a) from t2)", true, "CREATE GLOBAL BINDING FOR SELECT * FROM `*`.`t1` WHERE `t1`.`a`>(SELECT MAX(`a`) FROM `t2`) USING SELECT * FROM `*`.`t1` WHERE `t1`.`a`>(SELECT MAX(`a`) FROM `t2`)"},
		{"create session binding using select * from *.t1", true, "CREATE SESSION BINDING FOR SELECT * FROM `*`.`t1` USING SELECT * FROM `*`.`t1`"},
		{"create binding using select * from *.t1", true, "CREATE SESSION BINDING FOR SELECT * FROM `*`.`t1` USING SELECT * FROM `*`.`t1`"},
		// Update cases.
		{"CREATE GLOBAL BINDING FOR UPDATE `t` SET `a`=1 WHERE `b`=1 USING UPDATE /*+ USE_INDEX(`t` `b`)*/ `t` SET `a`=1 WHERE `b`=1", true, "CREATE GLOBAL BINDING FOR UPDATE `t` SET `a`=1 WHERE `b`=1 USING UPDATE /*+ USE_INDEX(`t` `b`)*/ `t` SET `a`=1 WHERE `b`=1"},
		{"CREATE SESSION BINDING FOR UPDATE `t` SET `a`=1 WHERE `b`=1 USING UPDATE /*+ USE_INDEX(`t` `b`)*/ `t` SET `a`=1 WHERE `b`=1", true, "CREATE SESSION BINDING FOR UPDATE `t` SET `a`=1 WHERE `b`=1 USING UPDATE /*+ USE_INDEX(`t` `b`)*/ `t` SET `a`=1 WHERE `b`=1"},
		{"drop global binding for update t set a = 1 where b = 1", true, "DROP GLOBAL BINDING FOR UPDATE `t` SET `a`=1 WHERE `b`=1"},
		{"drop session binding for update t set a = 1 where b = 1", true, "DROP SESSION BINDING FOR UPDATE `t` SET `a`=1 WHERE `b`=1"},
		{"DROP GLOBAL BINDING FOR UPDATE `t` SET `a`=1 WHERE `b`=1 USING UPDATE /*+ USE_INDEX(`t` `b`)*/ `t` SET `a`=1 WHERE `b`=1", true, "DROP GLOBAL BINDING FOR UPDATE `t` SET `a`=1 WHERE `b`=1 USING UPDATE /*+ USE_INDEX(`t` `b`)*/ `t` SET `a`=1 WHERE `b`=1"},
		{"DROP SESSION BINDING FOR UPDATE `t` SET `a`=1 WHERE `b`=1 USING UPDATE /*+ USE_INDEX(`t` `b`)*/ `t` SET `a`=1 WHERE `b`=1", true, "DROP SESSION BINDING FOR UPDATE `t` SET `a`=1 WHERE `b`=1 USING UPDATE /*+ USE_INDEX(`t` `b`)*/ `t` SET `a`=1 WHERE `b`=1"},
		// Multi-table Update.
		{"CREATE GLOBAL BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b` USING UPDATE /*+ INL_JOIN(`t1`)*/ `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`", true, "CREATE GLOBAL BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b` USING UPDATE /*+ INL_JOIN(`t1`)*/ `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`"},
		{"CREATE SESSION BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b` USING UPDATE /*+ INL_JOIN(`t1`)*/ `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`", true, "CREATE SESSION BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b` USING UPDATE /*+ INL_JOIN(`t1`)*/ `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`"},
		{"DROP GLOBAL BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`", true, "DROP GLOBAL BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`"},
		{"DROP SESSION BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`", true, "DROP SESSION BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`"},
		{"DROP GLOBAL BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b` USING UPDATE /*+ INL_JOIN(`t1`)*/ `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`", true, "DROP GLOBAL BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b` USING UPDATE /*+ INL_JOIN(`t1`)*/ `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`"},
		{"DROP SESSION BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b` USING UPDATE /*+ INL_JOIN(`t1`)*/ `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`", true, "DROP SESSION BINDING FOR UPDATE `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b` USING UPDATE /*+ INL_JOIN(`t1`)*/ `t1` JOIN `t2` SET `t1`.`a`=1 WHERE `t1`.`b`=`t2`.`b`"},
		// Delete cases.
		{"CREATE GLOBAL BINDING FOR DELETE FROM `t` WHERE `a`=1 USING DELETE /*+ USE_INDEX(`t` `a`)*/ FROM `t` WHERE `a`=1", true, "CREATE GLOBAL BINDING FOR DELETE FROM `t` WHERE `a`=1 USING DELETE /*+ USE_INDEX(`t` `a`)*/ FROM `t` WHERE `a`=1"},
		{"CREATE SESSION BINDING FOR DELETE FROM `t` WHERE `a`=1 USING DELETE /*+ USE_INDEX(`t` `a`)*/ FROM `t` WHERE `a`=1", true, "CREATE SESSION BINDING FOR DELETE FROM `t` WHERE `a`=1 USING DELETE /*+ USE_INDEX(`t` `a`)*/ FROM `t` WHERE `a`=1"},
		{"drop global binding for delete from t where a = 1", true, "DROP GLOBAL BINDING FOR DELETE FROM `t` WHERE `a`=1"},
		{"drop session binding for delete from t where a = 1", true, "DROP SESSION BINDING FOR DELETE FROM `t` WHERE `a`=1"},
		{"DROP GLOBAL BINDING FOR DELETE FROM `t` WHERE `a`=1 USING DELETE /*+ USE_INDEX(`t` `a`)*/ FROM `t` WHERE `a`=1", true, "DROP GLOBAL BINDING FOR DELETE FROM `t` WHERE `a`=1 USING DELETE /*+ USE_INDEX(`t` `a`)*/ FROM `t` WHERE `a`=1"},
		{"DROP SESSION BINDING FOR DELETE FROM `t` WHERE `a`=1 USING DELETE /*+ USE_INDEX(`t` `a`)*/ FROM `t` WHERE `a`=1", true, "DROP SESSION BINDING FOR DELETE FROM `t` WHERE `a`=1 USING DELETE /*+ USE_INDEX(`t` `a`)*/ FROM `t` WHERE `a`=1"},
		// Multi-table Delete.
		{"CREATE GLOBAL BINDING FOR DELETE `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1 USING DELETE /*+ HASH_JOIN(`t1`, `t2`)*/ `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1", true, "CREATE GLOBAL BINDING FOR DELETE `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1 USING DELETE /*+ HASH_JOIN(`t1`, `t2`)*/ `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1"},
		{"CREATE SESSION BINDING FOR DELETE `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1 USING DELETE /*+ HASH_JOIN(`t1`, `t2`)*/ `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1", true, "CREATE SESSION BINDING FOR DELETE `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1 USING DELETE /*+ HASH_JOIN(`t1`, `t2`)*/ `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1"},
		{"drop global binding for delete t1, t2 from t1 inner join t2 on t1.b = t2.b where t1.a = 1", true, "DROP GLOBAL BINDING FOR DELETE `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1"},
		{"drop session binding for delete t1, t2 from t1 inner join t2 on t1.b = t2.b where t1.a = 1", true, "DROP SESSION BINDING FOR DELETE `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1"},
		{"DROP GLOBAL BINDING FOR DELETE `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1 USING DELETE /*+ HASH_JOIN(`t1`, `t2`)*/ `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1", true, "DROP GLOBAL BINDING FOR DELETE `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1 USING DELETE /*+ HASH_JOIN(`t1`, `t2`)*/ `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1"},
		{"DROP SESSION BINDING FOR DELETE `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1 USING DELETE /*+ HASH_JOIN(`t1`, `t2`)*/ `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1", true, "DROP SESSION BINDING FOR DELETE `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1 USING DELETE /*+ HASH_JOIN(`t1`, `t2`)*/ `t1`,`t2` FROM `t1` JOIN `t2` ON `t1`.`b`=`t2`.`b` WHERE `t1`.`a`=1"},
		// Insert cases.
		{"CREATE GLOBAL BINDING FOR INSERT INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING INSERT INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1", true, "CREATE GLOBAL BINDING FOR INSERT INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING INSERT INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1"},
		{"CREATE SESSION BINDING FOR INSERT INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING INSERT INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1", true, "CREATE SESSION BINDING FOR INSERT INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING INSERT INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1"},
		{"drop global binding for insert into t1 select * from t2 where t1.a=1", true, "DROP GLOBAL BINDING FOR INSERT INTO `t1` SELECT * FROM `t2` WHERE `t1`.`a`=1"},
		{"drop session binding for insert into t1 select * from t2 where t1.a=1", true, "DROP SESSION BINDING FOR INSERT INTO `t1` SELECT * FROM `t2` WHERE `t1`.`a`=1"},
		{"DROP GLOBAL BINDING FOR INSERT INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING INSERT INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1", true, "DROP GLOBAL BINDING FOR INSERT INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING INSERT INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1"},
		{"DROP SESSION BINDING FOR INSERT INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING INSERT INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1", true, "DROP SESSION BINDING FOR INSERT INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING INSERT INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1"},
		// Replace cases.
		{"CREATE GLOBAL BINDING FOR REPLACE INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING REPLACE INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1", true, "CREATE GLOBAL BINDING FOR REPLACE INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING REPLACE INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1"},
		{"CREATE SESSION BINDING FOR REPLACE INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING REPLACE INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1", true, "CREATE SESSION BINDING FOR REPLACE INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING REPLACE INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1"},
		{"drop global binding for replace into t1 select * from t2 where t1.a=1", true, "DROP GLOBAL BINDING FOR REPLACE INTO `t1` SELECT * FROM `t2` WHERE `t1`.`a`=1"},
		{"drop session binding for replace into t1 select * from t2 where t1.a=1", true, "DROP SESSION BINDING FOR REPLACE INTO `t1` SELECT * FROM `t2` WHERE `t1`.`a`=1"},
		{"DROP GLOBAL BINDING FOR REPLACE INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING REPLACE INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1", true, "DROP GLOBAL BINDING FOR REPLACE INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING REPLACE INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1"},
		{"DROP SESSION BINDING FOR REPLACE INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING REPLACE INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1", true, "DROP SESSION BINDING FOR REPLACE INTO `t1` SELECT * FROM `t2` WHERE `t2`.`a`=1 USING REPLACE INTO `t1` SELECT /*+ USE_INDEX(`t2` `a`)*/ * FROM `t2` WHERE `t2`.`a`=1"},
		// Specify digest cases.
		{"DROP SESSION BINDING FOR SQL DIGEST 'a'", true, "DROP SESSION BINDING FOR SQL DIGEST 'a'"},
		{"drop global binding for sql digest 's'", true, "DROP GLOBAL BINDING FOR SQL DIGEST 's'"},
		{"drop global binding for sql digest @a, @b, 'test1,test2', @c, 'test333'", true, "DROP GLOBAL BINDING FOR SQL DIGEST @`a`, @`b`, 'test1,test2', @`c`, 'test333'"},
		{"create session binding from history using plan digest 'sss'", true, "CREATE SESSION BINDING FROM HISTORY USING PLAN DIGEST 'sss'"},
		{"create session binding from history using plan digest @a, @b, 'test1,test2', @c, 'test333'", true, "CREATE SESSION BINDING FROM HISTORY USING PLAN DIGEST @`a`, @`b`, 'test1,test2', @`c`, 'test333'"},
		{"CREATE GLOBAL BINDING FROM HISTORY USING PLAN DIGEST 'sss'", true, "CREATE GLOBAL BINDING FROM HISTORY USING PLAN DIGEST 'sss'"},
		{"set binding enabled for sql digest '1'", true, "SET BINDING ENABLED FOR SQL DIGEST '1'"},
		{"set binding disabled for sql digest '1'", true, "SET BINDING DISABLED FOR SQL DIGEST '1'"},
		// Explain explore for a specified SQL.
		{"explain explore 'select a from t'", true, "EXPLAIN EXPLORE 'select a from t'"},
		{"explain explore '23adc8e6f62'", true, "EXPLAIN EXPLORE '23adc8e6f62'"},
	}
	RunTest(t, table, false)

	p := parser.New()
	sms, _, err := p.Parse("create global binding for select * from t using select * from t use index(a)", "", "")
	require.NoError(t, err)
	v, ok := sms[0].(*ast.CreateBindingStmt)
	require.True(t, ok)
	require.Equal(t, "select * from t", v.OriginNode.Text())
	require.Equal(t, "select * from t use index(a)", v.HintedNode.Text())
	require.True(t, v.GlobalScope)
}

func TestView(t *testing.T) {
	table := []testCase{
		{"create view v as select * from t", true, "CREATE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t`"},
		{"create or replace view v as select * from t", true, "CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t`"},
		{"create or replace algorithm = undefined view v as select * from t", true, "CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t`"},
		{"create or replace algorithm = merge view v as select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t`"},
		{"create or replace algorithm = temptable view v as select * from t", true, "CREATE OR REPLACE ALGORITHM = TEMPTABLE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' view v as select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security definer view v as select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v as select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` AS SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as select * from t with local check option", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS SELECT * FROM `t` WITH LOCAL CHECK OPTION"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as select * from t with cascaded check option", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = current_user view v as select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t`"},

		// create view with `(` select statement `)`
		{"create view v as (select * from t)", true, "CREATE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t`)"},
		{"create or replace view v as (select * from t)", true, "CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t`)"},
		{"create or replace algorithm = undefined view v as (select * from t)", true, "CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t`)"},
		{"create or replace algorithm = merge view v as (select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t`)"},
		{"create or replace algorithm = temptable view v as (select * from t)", true, "CREATE OR REPLACE ALGORITHM = TEMPTABLE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = 'root' view v as (select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = 'root' sql security definer view v as (select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v as (select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` AS (SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as (select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS (SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as (select * from t) with local check option", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS (SELECT * FROM `t`) WITH LOCAL CHECK OPTION"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as (select * from t) with cascaded check option", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS (SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = current_user view v as (select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t`)"},

		// create view with union statement
		{"create view v as select * from t union select * from t", true, "CREATE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION SELECT * FROM `t`"},
		{"create or replace view v as select * from t union select * from t", true, "CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION SELECT * FROM `t`"},
		{"create or replace algorithm = undefined view v as select * from t union select * from t", true, "CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION SELECT * FROM `t`"},
		{"create or replace algorithm = merge view v as select * from t union select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION SELECT * FROM `t`"},
		{"create or replace algorithm = temptable view v as select * from t union select * from t", true, "CREATE OR REPLACE ALGORITHM = TEMPTABLE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' view v as select * from t union select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security definer view v as select * from t union select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v as select * from t union select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` AS SELECT * FROM `t` UNION SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as select * from t union select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS SELECT * FROM `t` UNION SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as select * from t union select * from t with local check option", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS SELECT * FROM `t` UNION SELECT * FROM `t` WITH LOCAL CHECK OPTION"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as select * from t union select * from t with cascaded check option", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS SELECT * FROM `t` UNION SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = current_user view v as select * from t union select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION SELECT * FROM `t`"},

		// create view with union all statement
		{"create view v as select * from t union all select * from t", true, "CREATE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create or replace view v as select * from t union all select * from t", true, "CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create or replace algorithm = undefined view v as select * from t union all select * from t", true, "CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create or replace algorithm = merge view v as select * from t union all select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create or replace algorithm = temptable view v as select * from t union all select * from t", true, "CREATE OR REPLACE ALGORITHM = TEMPTABLE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' view v as select * from t union all select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security definer view v as select * from t union all select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v as select * from t union all select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as select * from t union all select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as select * from t union all select * from t with local check option", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS SELECT * FROM `t` UNION ALL SELECT * FROM `t` WITH LOCAL CHECK OPTION"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as select * from t union all select * from t with cascaded check option", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
		{"create or replace algorithm = merge definer = current_user view v as select * from t union all select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},

		// create view with `(` union statement `)`
		{"create view v as (select * from t union all select * from t)", true, "CREATE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`)"},
		{"create or replace view v as (select * from t union all select * from t)", true, "CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`)"},
		{"create or replace algorithm = undefined view v as (select * from t union all select * from t)", true, "CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`)"},
		{"create or replace algorithm = merge view v as (select * from t union all select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`)"},
		{"create or replace algorithm = temptable view v as (select * from t union all select * from t)", true, "CREATE OR REPLACE ALGORITHM = TEMPTABLE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = 'root' view v as (select * from t union all select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = 'root' sql security definer view v as (select * from t union all select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY DEFINER VIEW `v` AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v as (select * from t union all select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as (select * from t union all select * from t)", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as (select * from t union all select * from t) with local check option", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`) WITH LOCAL CHECK OPTION"},
		{"create or replace algorithm = merge definer = 'root' sql security invoker view v(a,b) as (select * from t union all select * from t) with cascaded check option", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = `root`@`%` SQL SECURITY INVOKER VIEW `v` (`a`,`b`) AS (SELECT * FROM `t` UNION ALL SELECT * FROM `t`)"},
		{"create or replace algorithm = merge definer = current_user view v as select * from t union all select * from t", true, "CREATE OR REPLACE ALGORITHM = MERGE DEFINER = CURRENT_USER SQL SECURITY DEFINER VIEW `v` AS SELECT * FROM `t` UNION ALL SELECT * FROM `t`"},
	}
	RunTest(t, table, false)

	// Test case for the text of the select statement in create view statement.
	p := parser.New()
	sms, _, err := p.Parse("create view v as select * from t", "", "")
	require.NoError(t, err)
	v, ok := sms[0].(*ast.CreateViewStmt)
	require.True(t, ok)
	require.Equal(t, ast.AlgorithmUndefined, v.Algorithm)
	require.Equal(t, "select * from t", v.Select.Text())
	require.Equal(t, ast.SecurityDefiner, v.Security)
	require.Equal(t, ast.CheckOptionCascaded, v.CheckOption)

	src := `CREATE OR REPLACE ALGORITHM = UNDEFINED DEFINER = root@localhost
                  SQL SECURITY DEFINER
			      VIEW V(a,b,c) AS select c,d,e from t
                  WITH CASCADED CHECK OPTION;`

	var st ast.StmtNode
	st, err = p.ParseOneStmt(src, "", "")
	require.NoError(t, err)
	v, ok = st.(*ast.CreateViewStmt)
	require.True(t, ok)
	require.True(t, v.OrReplace)
	require.Equal(t, ast.AlgorithmUndefined, v.Algorithm)
	require.Equal(t, "root", v.Definer.Username)
	require.Equal(t, "localhost", v.Definer.Hostname)
	require.Equal(t, ast.NewCIStr("a"), v.Cols[0])
	require.Equal(t, ast.NewCIStr("b"), v.Cols[1])
	require.Equal(t, ast.NewCIStr("c"), v.Cols[2])
	require.Equal(t, "select c,d,e from t", v.Select.Text())
	require.Equal(t, ast.SecurityDefiner, v.Security)
	require.Equal(t, ast.CheckOptionCascaded, v.CheckOption)

	src = `
CREATE VIEW v1 AS SELECT * FROM t;
CREATE VIEW v2 AS SELECT 123123123123123;
`
	nodes, _, err := p.Parse(src, "", "")
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, nodes[0].(*ast.CreateViewStmt).Select.Text(), "SELECT * FROM t")
	require.Equal(t, nodes[1].(*ast.CreateViewStmt).Select.Text(), "SELECT 123123123123123")
}

func TestTimestampDiffUnit(t *testing.T) {
	// Test case for timestampdiff unit.
	// TimeUnit should be unified to upper case.
	p := parser.New()
	stmt, _, err := p.Parse("SELECT TIMESTAMPDIFF(MONTH,'2003-02-01','2003-05-01'), TIMESTAMPDIFF(month,'2003-02-01','2003-05-01');", "", "")
	require.NoError(t, err)
	ss := stmt[0].(*ast.SelectStmt)
	fields := ss.Fields.Fields
	require.Len(t, fields, 2)
	expr := fields[0].Expr
	f, ok := expr.(*ast.FuncCallExpr)
	require.True(t, ok)
	require.Equal(t, ast.TimeUnitMonth, f.Args[0].(*ast.TimeUnitExpr).Unit)

	expr = fields[1].Expr
	f, ok = expr.(*ast.FuncCallExpr)
	require.True(t, ok)
	require.Equal(t, ast.TimeUnitMonth, f.Args[0].(*ast.TimeUnitExpr).Unit)

	// Test Illegal TimeUnit for TimestampDiff
	table := []testCase{
		{"SELECT TIMESTAMPDIFF(SECOND_MICROSECOND,'2003-02-01','2003-05-01')", false, ""},
		{"SELECT TIMESTAMPDIFF(MINUTE_MICROSECOND,'2003-02-01','2003-05-01')", false, ""},
		{"SELECT TIMESTAMPDIFF(MINUTE_SECOND,'2003-02-01','2003-05-01')", false, ""},
		{"SELECT TIMESTAMPDIFF(HOUR_MICROSECOND,'2003-02-01','2003-05-01')", false, ""},
		{"SELECT TIMESTAMPDIFF(HOUR_SECOND,'2003-02-01','2003-05-01')", false, ""},
		{"SELECT TIMESTAMPDIFF(HOUR_MINUTE,'2003-02-01','2003-05-01')", false, ""},
		{"SELECT TIMESTAMPDIFF(DAY_MICROSECOND,'2003-02-01','2003-05-01')", false, ""},
		{"SELECT TIMESTAMPDIFF(DAY_SECOND,'2003-02-01','2003-05-01')", false, ""},
		{"SELECT TIMESTAMPDIFF(DAY_MINUTE,'2003-02-01','2003-05-01')", false, ""},
		{"SELECT TIMESTAMPDIFF(DAY_HOUR,'2003-02-01','2003-05-01')", false, ""},
		{"SELECT TIMESTAMPDIFF(YEAR_MONTH,'2003-02-01','2003-05-01')", false, ""},
	}
	RunTest(t, table, false)
}

func TestFuncCallExprOffset(t *testing.T) {
	// Test case for offset field on func call expr.
	p := parser.New()
	stmt, _, err := p.Parse("SELECT s.a(), b();", "", "")
	require.NoError(t, err)
	ss := stmt[0].(*ast.SelectStmt)
	fields := ss.Fields.Fields
	require.Len(t, fields, 2)

	{
		// s.a()
		expr := fields[0].Expr
		f, ok := expr.(*ast.FuncCallExpr)
		require.True(t, ok)
		require.Equal(t, 7, f.OriginTextPosition())
	}

	{
		// b()
		expr := fields[1].Expr
		f, ok := expr.(*ast.FuncCallExpr)
		require.True(t, ok)
		require.Equal(t, 14, f.OriginTextPosition())
	}
}

func TestSessionManage(t *testing.T) {
	table := []testCase{
		// Kill statement.
		// See https://dev.mysql.com/doc/refman/5.7/en/kill.html
		{"kill 23123", true, "KILL 23123"},
		{"kill CONNECTION_ID()", true, "KILL CONNECTION_ID()"},
		{"kill connection 23123", true, "KILL 23123"},
		{"kill query 23123", true, "KILL QUERY 23123"},
		{"kill tidb 23123", true, "KILL TIDB 23123"},
		{"kill tidb connection 23123", true, "KILL TIDB 23123"},
		{"kill tidb query 23123", true, "KILL TIDB QUERY 23123"},
		{"show processlist", true, "SHOW PROCESSLIST"},
		{"show full processlist", true, "SHOW FULL PROCESSLIST"},
		{"shutdown", true, "SHUTDOWN"},
		{"restart", true, "RESTART"},
	}
	RunTest(t, table, false)
}

func TestParseShowOpenTables(t *testing.T) {
	table := []testCase{
		{"SHOW OPEN TABLES", true, "SHOW OPEN TABLES"},
		{"SHOW OPEN TABLES IN test", true, "SHOW OPEN TABLES IN `test`"},
		{"SHOW OPEN TABLES FROM test", true, "SHOW OPEN TABLES IN `test`"},
	}
	RunTest(t, table, false)
}

func TestSQLModeANSIQuotes(t *testing.T) {
	p := parser.New()
	p.SetSQLMode(mysql.ModeANSIQuotes)
	tests := []string{
		`CREATE TABLE "table" ("id" int)`,
		`select * from t "tt"`,
	}
	for _, test := range tests {
		_, _, err := p.Parse(test, "", "")
		require.NoError(t, err)
	}
}

func TestDDLStatements(t *testing.T) {
	p := parser.New()
	// Tests that whatever the charset it is define, we always assign utf8 charset and utf8_bin collate.
	createTableStr := `CREATE TABLE t (
		a varchar(64) binary,
		b char(10) charset utf8 collate utf8_general_ci,
		c text charset latin1) ENGINE=innoDB DEFAULT CHARSET=utf8 COLLATE=utf8_bin`
	stmts, _, err := p.Parse(createTableStr, "", "")
	require.NoError(t, err)
	stmt := stmts[0].(*ast.CreateTableStmt)
	require.True(t, mysql.HasBinaryFlag(stmt.Cols[0].Tp.GetFlag()))
	for _, colDef := range stmt.Cols[1:] {
		require.False(t, mysql.HasBinaryFlag(colDef.Tp.GetFlag()))
	}
	for _, tblOpt := range stmt.Options {
		switch tblOpt.Tp {
		case ast.TableOptionCharset:
			require.Equal(t, "utf8", tblOpt.StrValue)
		case ast.TableOptionCollate:
			require.Equal(t, "utf8_bin", tblOpt.StrValue)
		}
	}
	createTableStr = `CREATE TABLE t (
		a varbinary(64),
		b binary(10),
		c blob)`
	stmts, _, err = p.Parse(createTableStr, "", "")
	require.NoError(t, err)
	stmt = stmts[0].(*ast.CreateTableStmt)
	for _, colDef := range stmt.Cols {
		require.Equal(t, charset.CharsetBin, colDef.Tp.GetCharset())
		require.Equal(t, charset.CollationBin, colDef.Tp.GetCollate())
		require.True(t, mysql.HasBinaryFlag(colDef.Tp.GetFlag()))
	}
	// Test set collate for all column types
	createTableStr = `CREATE TABLE t (
		c_int int collate utf8_bin,
		c_real real collate utf8_bin,
		c_float float collate utf8_bin,
		c_bool bool collate utf8_bin,
		c_char char collate utf8_bin,
		c_binary binary collate utf8_bin,
		c_varchar varchar(2) collate utf8_bin,
		c_year year collate utf8_bin,
		c_date date collate utf8_bin,
		c_time time collate utf8_bin,
		c_datetime datetime collate utf8_bin,
		c_timestamp timestamp collate utf8_bin,
		c_tinyblob tinyblob collate utf8_bin,
		c_blob blob collate utf8_bin,
		c_mediumblob mediumblob collate utf8_bin,
		c_longblob longblob collate utf8_bin,
		c_bit bit collate utf8_bin,
		c_long_varchar long varchar collate utf8_bin,
		c_tinytext tinytext collate utf8_bin,
		c_text text collate utf8_bin,
		c_mediumtext mediumtext collate utf8_bin,
		c_longtext longtext collate utf8_bin,
		c_decimal decimal collate utf8_bin,
		c_numeric numeric collate utf8_bin,
		c_enum enum('1') collate utf8_bin,
		c_set set('1') collate utf8_bin,
		c_json json collate utf8_bin)`
	_, _, err = p.Parse(createTableStr, "", "")
	require.NoError(t, err)

	createTableStr = `CREATE TABLE t (c_double double(10))`
	_, _, err = p.Parse(createTableStr, "", "")
	require.EqualError(t, err, "[parser:1149]You have an error in your SQL syntax; check the manual that corresponds to your MySQL server version for the right syntax to use")
	p.SetStrictDoubleTypeCheck(false)
	_, _, err = p.Parse(createTableStr, "", "")
	require.NoError(t, err)
	p.SetStrictDoubleTypeCheck(true)

	createTableStr = `CREATE TABLE t (c_double double(10, 2))`
	_, _, err = p.Parse(createTableStr, "", "")
	require.NoError(t, err)

	createTableStr = `create global temporary table t010(local_01 int, local_03 varchar(20))`
	_, _, err = p.Parse(createTableStr, "", "")
	require.EqualError(t, err, "line 1 column 70 near \"\"GLOBAL TEMPORARY and ON COMMIT DELETE ROWS must appear together ")

	createTableStr = `create global temporary table t010(local_01 int, local_03 varchar(20)) on commit preserve rows`
	_, _, err = p.Parse(createTableStr, "", "")
	require.NoError(t, err)
}

func TestAnalyze(t *testing.T) {
	table := []testCase{
		{"analyze table t1", true, "ANALYZE TABLE `t1`"},
		{"analyze table t1.*", false, ""},
		{"analyze table t,t1", true, "ANALYZE TABLE `t`,`t1`"},
		{"analyze table t1 index", true, "ANALYZE TABLE `t1` INDEX"},
		{"analyze table t1 index a", true, "ANALYZE TABLE `t1` INDEX `a`"},
		{"analyze table t1 index a,b", true, "ANALYZE TABLE `t1` INDEX `a`,`b`"},
		{"analyze table t with 4 buckets", true, "ANALYZE TABLE `t` WITH 4 BUCKETS"},
		{"analyze table t with 4 topn", true, "ANALYZE TABLE `t` WITH 4 TOPN"},
		{"analyze table t with 4 cmsketch width", true, "ANALYZE TABLE `t` WITH 4 CMSKETCH WIDTH"},
		{"analyze table t with 4 cmsketch depth", true, "ANALYZE TABLE `t` WITH 4 CMSKETCH DEPTH"},
		{"analyze table t with 4 samples", true, "ANALYZE TABLE `t` WITH 4 SAMPLES"},
		{"analyze table t with 4 buckets, 4 topn, 4 cmsketch width, 4 cmsketch depth, 4 samples", true, "ANALYZE TABLE `t` WITH 4 BUCKETS, 4 TOPN, 4 CMSKETCH WIDTH, 4 CMSKETCH DEPTH, 4 SAMPLES"},
		{"analyze table t index a with 4 buckets", true, "ANALYZE TABLE `t` INDEX `a` WITH 4 BUCKETS"},
		{"analyze table t partition a", true, "ANALYZE TABLE `t` PARTITION `a`"},
		{"analyze table t partition a with 4 buckets", true, "ANALYZE TABLE `t` PARTITION `a` WITH 4 BUCKETS"},
		{"analyze table t partition a index b", true, "ANALYZE TABLE `t` PARTITION `a` INDEX `b`"},
		{"analyze table t partition a index b with 4 buckets", true, "ANALYZE TABLE `t` PARTITION `a` INDEX `b` WITH 4 BUCKETS"},
		{"analyze incremental table t index", true, "ANALYZE INCREMENTAL TABLE `t` INDEX"},
		{"analyze incremental table t index idx", true, "ANALYZE INCREMENTAL TABLE `t` INDEX `idx`"},
		{"analyze table t update histogram on b with 1024 buckets", true, "ANALYZE TABLE `t` UPDATE HISTOGRAM ON `b` WITH 1024 BUCKETS"},
		{"analyze table t drop histogram on b", true, "ANALYZE TABLE `t` DROP HISTOGRAM ON `b`"},
		{"analyze table t update histogram on c1, c2;", true, "ANALYZE TABLE `t` UPDATE HISTOGRAM ON `c1`,`c2`"},
		{"analyze table t drop histogram on c1, c2;", true, "ANALYZE TABLE `t` DROP HISTOGRAM ON `c1`,`c2`"},
		{"analyze table t update histogram on t.c1, t.c2", false, ""},
		{"analyze table t drop histogram on t.c1, t.c2", false, ""},
		{"analyze table t1,t2 all columns", true, "ANALYZE TABLE `t1`,`t2` ALL COLUMNS"},
		{"analyze table t partition a all columns", true, "ANALYZE TABLE `t` PARTITION `a` ALL COLUMNS"},
		{"analyze table t1,t2 all columns with 4 topn", true, "ANALYZE TABLE `t1`,`t2` ALL COLUMNS WITH 4 TOPN"},
		{"analyze table t partition a all columns with 1024 buckets", true, "ANALYZE TABLE `t` PARTITION `a` ALL COLUMNS WITH 1024 BUCKETS"},
		{"analyze table t1,t2 predicate columns", true, "ANALYZE TABLE `t1`,`t2` PREDICATE COLUMNS"},
		{"analyze table t partition a predicate columns", true, "ANALYZE TABLE `t` PARTITION `a` PREDICATE COLUMNS"},
		{"analyze table t1,t2 predicate columns with 4 topn", true, "ANALYZE TABLE `t1`,`t2` PREDICATE COLUMNS WITH 4 TOPN"},
		{"analyze table t partition a predicate columns with 1024 buckets", true, "ANALYZE TABLE `t` PARTITION `a` PREDICATE COLUMNS WITH 1024 BUCKETS"},
		{"analyze table t columns c1,c2", true, "ANALYZE TABLE `t` COLUMNS `c1`,`c2`"},
		{"analyze table t partition a columns c1,c2", true, "ANALYZE TABLE `t` PARTITION `a` COLUMNS `c1`,`c2`"},
		{"analyze table t columns t.c1,t.c2", false, ""},
		{"analyze table t partition a columns t.c1,t.c2", false, ""},
		{"analyze table t columns c1,c2 with 4 topn", true, "ANALYZE TABLE `t` COLUMNS `c1`,`c2` WITH 4 TOPN"},
		{"analyze table t partition a columns c1,c2 with 1024 buckets", true, "ANALYZE TABLE `t` PARTITION `a` COLUMNS `c1`,`c2` WITH 1024 BUCKETS"},
		{"analyze table t index a columns c", false, ""},
		{"analyze table t index a all columns", false, ""},
		{"analyze table t index a predicate columns", false, ""},
		{"analyze table t with 10 samplerate", true, "ANALYZE TABLE `t` WITH 10 SAMPLERATE"},
		{"analyze table t with 0.1 samplerate", true, "ANALYZE TABLE `t` WITH 0.1 SAMPLERATE"},
		{"analyze no_write_to_binlog table t1", true, "ANALYZE NO_WRITE_TO_BINLOG TABLE `t1`"},
		{"analyze local table t,t1", true, "ANALYZE NO_WRITE_TO_BINLOG TABLE `t`,`t1`"},
	}
	RunTest(t, table, false)
}

func TestTableSample(t *testing.T) {
	table := []testCase{
		// positive test cases
		{"select * from tbl tablesample system (50);", true, "SELECT * FROM `tbl` TABLESAMPLE SYSTEM (50)"},
		{"select * from tbl tablesample system (50 percent);", true, "SELECT * FROM `tbl` TABLESAMPLE SYSTEM (50 PERCENT)"},
		{"select * from tbl tablesample system (49.9 percent);", true, "SELECT * FROM `tbl` TABLESAMPLE SYSTEM (49.9 PERCENT)"},
		{"select * from tbl tablesample system (120 rows);", true, "SELECT * FROM `tbl` TABLESAMPLE SYSTEM (120 ROWS)"},
		{"select * from tbl tablesample bernoulli (50);", true, "SELECT * FROM `tbl` TABLESAMPLE BERNOULLI (50)"},
		{"select * from tbl tablesample (50);", true, "SELECT * FROM `tbl` TABLESAMPLE (50)"},
		{"select * from tbl tablesample (50) repeatable (123456789);", true, "SELECT * FROM `tbl` TABLESAMPLE (50) REPEATABLE(123456789)"},
		{"select * from tbl as a tablesample (50);", true, "SELECT * FROM `tbl` AS `a` TABLESAMPLE (50)"},
		{"select * from tbl `tablesample` tablesample (50);", true, "SELECT * FROM `tbl` AS `tablesample` TABLESAMPLE (50)"},
		{"select * from tbl tablesample (50) where id > 20;", true, "SELECT * FROM `tbl` TABLESAMPLE (50) WHERE `id`>20"},
		{"select * from tbl partition (p0) tablesample (50);", true, "SELECT * FROM `tbl` PARTITION(`p0`) TABLESAMPLE (50)"},
		{"select * from tbl tablesample (0 percent);", true, "SELECT * FROM `tbl` TABLESAMPLE (0 PERCENT)"},
		{"select * from tbl tablesample (100 percent);", true, "SELECT * FROM `tbl` TABLESAMPLE (100 PERCENT)"},
		{"select * from tbl tablesample (0 rows);", true, "SELECT * FROM `tbl` TABLESAMPLE (0 ROWS)"},
		{"select * from tbl tablesample ('34');", true, "SELECT * FROM `tbl` TABLESAMPLE (_UTF8MB4'34')"},
		{"select * from tbl1 tablesample (10), tbl2 tablesample (20);", true, "SELECT * FROM (`tbl1` TABLESAMPLE (10)) JOIN `tbl2` TABLESAMPLE (20)"},
		{"select * from tbl1 a tablesample (10) join tbl2 b tablesample (20) on a.id <> b.id;", true, "SELECT * FROM `tbl1` AS `a` TABLESAMPLE (10) JOIN `tbl2` AS `b` TABLESAMPLE (20) ON `a`.`id`!=`b`.`id`"},
		{"select * from demo tablesample bernoulli(50) limit 1 into outfile '/tmp/sample.csv';", true, "SELECT * FROM `demo` TABLESAMPLE BERNOULLI (50) LIMIT 1 INTO OUTFILE '/tmp/sample.csv'"},
		{"select * from demo tablesample bernoulli(50) order by a, b into outfile '/tmp/sample.csv';", true, "SELECT * FROM `demo` TABLESAMPLE BERNOULLI (50) ORDER BY `a`,`b` INTO OUTFILE '/tmp/sample.csv'"},

		// negative test cases
		{"select * from tbl tablesample system(50) a;", false, ""},
		{"select * from tbl tablesample (50) partition (p0);", false, ""},
		{"select * from tbl where id > 20 tablesample system(50);", false, ""},
		{"select * from (select * from tbl) a tablesample system(50);", false, ""},
		{"select * from tbl tablesample system(50) tablesample system(50);", false, ""},
		{"select * from tbl tablesample system(50, 50);", false, ""},
		{"select * from tbl tablesample dhfksdlfljcoew(50);", false, ""},
		{"select * from tbl tablesample system;", false, ""},
		{"select * from tbl tablesample system (33) repeatable;", false, ""},
		{"select 1 from dual tablesample system (50);", false, ""},
	}
	RunTest(t, table, false)
	p := parser.New()
	cases := []string{
		"select * from tbl tablesample (33.3 + 44.4);",
		"select * from tbl tablesample (33.3 + 44.4 percent);",
		"select * from tbl tablesample (33 + 44 rows);",
		"select * from tbl tablesample (33 + 44 rows) repeatable (55 + 66);",
		"select * from tbl tablesample (200);",
		"select * from tbl tablesample (-10);",
		"select * from tbl tablesample (null);",
		"select * from tbl tablesample (33.3 rows);",
		"select * from tbl tablesample (-4 rows);",
		"select * from tbl tablesample (50) repeatable ('ssss');",
		"delete from tbl using tbl2 tablesample(10 rows) repeatable (111) where tbl.id = tbl2.id",
		"update tbl tablesample regions() set id = '1'",
	}
	for _, sql := range cases {
		_, err := p.ParseOneStmt(sql, "", "")
		require.NoErrorf(t, err, "source %v", sql)
	}
}

func TestGeneratedColumn(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
		expr  string
	}{
		{"create table t (c int, d int generated always as (c + 1) virtual)", true, "c + 1"},
		{"create table t (c int, d int as (   c + 1   ) virtual)", true, "c + 1"},
		{"create table t (c int, d int as (1 + 1) stored)", true, "1 + 1"},
	}
	p := parser.New()
	for _, tbl := range tests {
		stmtNodes, _, err := p.Parse(tbl.input, "", "")
		if tbl.ok {
			require.NoError(t, err)
			stmtNode := stmtNodes[0]
			for _, col := range stmtNode.(*ast.CreateTableStmt).Cols {
				for _, opt := range col.Options {
					if opt.Tp == ast.ColumnOptionGenerated {
						require.Equal(t, tbl.expr, opt.Expr.Text())
					}
				}
			}
		} else {
			require.Error(t, err)
		}
	}

	_, _, err := p.Parse("create table t1 (a int, b int as (a + 1) default 10);", "", "")
	require.Equal(t, err.Error(), "[ddl:1221]Incorrect usage of DEFAULT and generated column")
	_, _, err = p.Parse("create table t1 (a int, b int as (a + 1) on update now());", "", "")
	require.Equal(t, err.Error(), "[ddl:1221]Incorrect usage of ON UPDATE and generated column")
	_, _, err = p.Parse("create table t1 (a int, b int as (a + 1) auto_increment);", "", "")
	require.Equal(t, err.Error(), "[ddl:1221]Incorrect usage of AUTO_INCREMENT and generated column")
}

func TestSetTransaction(t *testing.T) {
	// Set transaction is equivalent to setting the global or session value of tx_isolation.
	// For example:
	// SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED
	// SET SESSION tx_isolation='READ-COMMITTED'
	tests := []struct {
		input    string
		isGlobal bool
		value    string
	}{
		{
			"SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED",
			false, "READ-COMMITTED",
		},
		{
			"SET GLOBAL TRANSACTION ISOLATION LEVEL REPEATABLE READ",
			true, "REPEATABLE-READ",
		},
	}
	p := parser.New()
	for _, tbl := range tests {
		stmt1, err := p.ParseOneStmt(tbl.input, "", "")
		require.NoError(t, err)
		setStmt := stmt1.(*ast.SetStmt)
		vars := setStmt.Variables[0]
		require.Equal(t, "tx_isolation", vars.Name)
		require.Equal(t, tbl.isGlobal, vars.IsGlobal)
		require.Equal(t, true, vars.IsSystem)
		require.Equal(t, tbl.value, vars.Value.(ast.ValueExpr).GetValue())
	}
}

func TestSideEffect(t *testing.T) {
	// This test cover a bug that parse an error SQL doesn't leave the parser in a
	// clean state, cause the following SQL parse fail.
	p := parser.New()
	_, err := p.ParseOneStmt("create table t /*!50100 'abc', 'abc' */;", "", "")
	require.Error(t, err)

	_, err = p.ParseOneStmt("show tables;", "", "")
	require.NoError(t, err)
}

func TestTablePartition(t *testing.T) {
	table := []testCase{
		{"ALTER TABLE t1 TRUNCATE PARTITION p0", true, "ALTER TABLE `t1` TRUNCATE PARTITION `p0`"},
		{"ALTER TABLE t1 TRUNCATE PARTITION p0, p1", true, "ALTER TABLE `t1` TRUNCATE PARTITION `p0`,`p1`"},
		{"ALTER TABLE t1 TRUNCATE PARTITION ALL", true, "ALTER TABLE `t1` TRUNCATE PARTITION ALL"},
		{"ALTER TABLE t1 TRUNCATE PARTITION ALL, p0", false, ""},
		{"ALTER TABLE t1 TRUNCATE PARTITION p0, ALL", false, ""},

		{"ALTER TABLE t1 OPTIMIZE PARTITION p0", true, "ALTER TABLE `t1` OPTIMIZE PARTITION `p0`"},
		{"ALTER TABLE t1 OPTIMIZE PARTITION NO_WRITE_TO_BINLOG p0", true, "ALTER TABLE `t1` OPTIMIZE PARTITION NO_WRITE_TO_BINLOG `p0`"},
		// LOCAL is alias to NO_WRITE_TO_BINLOG
		{"ALTER TABLE t1 OPTIMIZE PARTITION LOCAL p0", true, "ALTER TABLE `t1` OPTIMIZE PARTITION NO_WRITE_TO_BINLOG `p0`"},
		{"ALTER TABLE t1 OPTIMIZE PARTITION p0, p1", true, "ALTER TABLE `t1` OPTIMIZE PARTITION `p0`,`p1`"},
		{"ALTER TABLE t1 OPTIMIZE PARTITION NO_WRITE_TO_BINLOG p0, p1", true, "ALTER TABLE `t1` OPTIMIZE PARTITION NO_WRITE_TO_BINLOG `p0`,`p1`"},
		{"ALTER TABLE t1 OPTIMIZE PARTITION LOCAL p0, p1", true, "ALTER TABLE `t1` OPTIMIZE PARTITION NO_WRITE_TO_BINLOG `p0`,`p1`"},
		{"ALTER TABLE t1 OPTIMIZE PARTITION ALL", true, "ALTER TABLE `t1` OPTIMIZE PARTITION ALL"},
		{"ALTER TABLE t1 OPTIMIZE PARTITION NO_WRITE_TO_BINLOG ALL", true, "ALTER TABLE `t1` OPTIMIZE PARTITION NO_WRITE_TO_BINLOG ALL"},
		{"ALTER TABLE t1 OPTIMIZE PARTITION LOCAL ALL", true, "ALTER TABLE `t1` OPTIMIZE PARTITION NO_WRITE_TO_BINLOG ALL"},
		{"ALTER TABLE t1 OPTIMIZE PARTITION ALL, p0", false, ""},
		{"ALTER TABLE t1 OPTIMIZE PARTITION p0, ALL", false, ""},
		// The first `LOCAL` should be recognized as unreserved keyword `LOCAL` (alias to `NO_WRITE_TO_BINLOG`),
		// and the remains should re recognized as identifier, used as partition name here.
		{"ALTER TABLE t_n OPTIMIZE PARTITION LOCAL", false, ""},
		{"ALTER TABLE t_n OPTIMIZE PARTITION LOCAL local", true, "ALTER TABLE `t_n` OPTIMIZE PARTITION NO_WRITE_TO_BINLOG `local`"},
		{"ALTER TABLE t_n OPTIMIZE PARTITION LOCAL local, local", true, "ALTER TABLE `t_n` OPTIMIZE PARTITION NO_WRITE_TO_BINLOG `local`,`local`"},

		{"ALTER TABLE t1 REPAIR PARTITION p0", true, "ALTER TABLE `t1` REPAIR PARTITION `p0`"},
		{"ALTER TABLE t1 REPAIR PARTITION NO_WRITE_TO_BINLOG p0", true, "ALTER TABLE `t1` REPAIR PARTITION NO_WRITE_TO_BINLOG `p0`"},
		// LOCAL is alias to NO_WRITE_TO_BINLOG
		{"ALTER TABLE t1 REPAIR PARTITION LOCAL p0", true, "ALTER TABLE `t1` REPAIR PARTITION NO_WRITE_TO_BINLOG `p0`"},
		{"ALTER TABLE t1 REPAIR PARTITION p0, p1", true, "ALTER TABLE `t1` REPAIR PARTITION `p0`,`p1`"},
		{"ALTER TABLE t1 REPAIR PARTITION NO_WRITE_TO_BINLOG p0, p1", true, "ALTER TABLE `t1` REPAIR PARTITION NO_WRITE_TO_BINLOG `p0`,`p1`"},
		{"ALTER TABLE t1 REPAIR PARTITION LOCAL p0, p1", true, "ALTER TABLE `t1` REPAIR PARTITION NO_WRITE_TO_BINLOG `p0`,`p1`"},
		{"ALTER TABLE t1 REPAIR PARTITION ALL", true, "ALTER TABLE `t1` REPAIR PARTITION ALL"},
		{"ALTER TABLE t1 REPAIR PARTITION NO_WRITE_TO_BINLOG ALL", true, "ALTER TABLE `t1` REPAIR PARTITION NO_WRITE_TO_BINLOG ALL"},
		{"ALTER TABLE t1 REPAIR PARTITION LOCAL ALL", true, "ALTER TABLE `t1` REPAIR PARTITION NO_WRITE_TO_BINLOG ALL"},
		{"ALTER TABLE t1 REPAIR PARTITION ALL, p0", false, ""},
		{"ALTER TABLE t1 REPAIR PARTITION p0, ALL", false, ""},
		// The first `LOCAL` should be recognized as unreserved keyword `LOCAL` (alias to `NO_WRITE_TO_BINLOG`),
		// and the remains should re recognized as identifier, used as partition name here.
		{"ALTER TABLE t_n REPAIR PARTITION LOCAL", false, ""},
		{"ALTER TABLE t_n REPAIR PARTITION LOCAL local", true, "ALTER TABLE `t_n` REPAIR PARTITION NO_WRITE_TO_BINLOG `local`"},
		{"ALTER TABLE t_n REPAIR PARTITION LOCAL local, local", true, "ALTER TABLE `t_n` REPAIR PARTITION NO_WRITE_TO_BINLOG `local`,`local`"},

		{"ALTER TABLE t1 IMPORT PARTITION p0 TABLESPACE", true, "ALTER TABLE `t1` IMPORT PARTITION `p0` TABLESPACE"},
		{"ALTER TABLE t1 IMPORT PARTITION p0, p1 TABLESPACE", true, "ALTER TABLE `t1` IMPORT PARTITION `p0`,`p1` TABLESPACE"},
		{"ALTER TABLE t1 IMPORT PARTITION ALL TABLESPACE", true, "ALTER TABLE `t1` IMPORT PARTITION ALL TABLESPACE"},
		{"ALTER TABLE t1 IMPORT PARTITION ALL, p0 TABLESPACE", false, ""},
		{"ALTER TABLE t1 IMPORT PARTITION p0, ALL TABLESPACE", false, ""},

		{"ALTER TABLE t1 DISCARD PARTITION p0 TABLESPACE", true, "ALTER TABLE `t1` DISCARD PARTITION `p0` TABLESPACE"},
		{"ALTER TABLE t1 DISCARD PARTITION p0, p1 TABLESPACE", true, "ALTER TABLE `t1` DISCARD PARTITION `p0`,`p1` TABLESPACE"},
		{"ALTER TABLE t1 DISCARD PARTITION ALL TABLESPACE", true, "ALTER TABLE `t1` DISCARD PARTITION ALL TABLESPACE"},
		{"ALTER TABLE t1 DISCARD PARTITION ALL, p0 TABLESPACE", false, ""},
		{"ALTER TABLE t1 DISCARD PARTITION p0, ALL TABLESPACE", false, ""},

		{"ALTER TABLE t1 ADD PARTITION (PARTITION `p5` VALUES LESS THAN (2010) COMMENT 'APSTART \\' APEND')", true, "ALTER TABLE `t1` ADD PARTITION (PARTITION `p5` VALUES LESS THAN (2010) COMMENT = 'APSTART '' APEND')"},
		{"ALTER TABLE t1 ADD PARTITION (PARTITION `p5` VALUES LESS THAN (2010) COMMENT = 'xxx')", true, "ALTER TABLE `t1` ADD PARTITION (PARTITION `p5` VALUES LESS THAN (2010) COMMENT = 'xxx')"},
		{`CREATE TABLE t1 (a int not null,b int not null,c int not null,primary key(a,b))
		partition by range (a)
		partitions 3
		(partition x1 values less than (5),
		 partition x2 values less than (10),
		 partition x3 values less than maxvalue);`, true, "CREATE TABLE `t1` (`a` INT NOT NULL,`b` INT NOT NULL,`c` INT NOT NULL,PRIMARY KEY(`a`, `b`)) PARTITION BY RANGE (`a`) (PARTITION `x1` VALUES LESS THAN (5),PARTITION `x2` VALUES LESS THAN (10),PARTITION `x3` VALUES LESS THAN (MAXVALUE))"},
		{"CREATE TABLE t1 (a int not null) partition by range (a) (partition x1 values less than (5) tablespace ts1)", true, "CREATE TABLE `t1` (`a` INT NOT NULL) PARTITION BY RANGE (`a`) (PARTITION `x1` VALUES LESS THAN (5) TABLESPACE = `ts1`)"},
		{`create table t (a int) partition by range (a)
		  (PARTITION p0 VALUES LESS THAN (63340531200) ENGINE = MyISAM,
		   PARTITION p1 VALUES LESS THAN (63342604800) ENGINE MyISAM)`, true, "CREATE TABLE `t` (`a` INT) PARTITION BY RANGE (`a`) (PARTITION `p0` VALUES LESS THAN (63340531200) ENGINE = MyISAM,PARTITION `p1` VALUES LESS THAN (63342604800) ENGINE = MyISAM)"},
		{`create table t (a int) partition by range (a)
		  (PARTITION p0 VALUES LESS THAN (63340531200) ENGINE = MyISAM COMMENT 'xxx',
		   PARTITION p1 VALUES LESS THAN (63342604800) ENGINE = MyISAM)`, true, "CREATE TABLE `t` (`a` INT) PARTITION BY RANGE (`a`) (PARTITION `p0` VALUES LESS THAN (63340531200) ENGINE = MyISAM COMMENT = 'xxx',PARTITION `p1` VALUES LESS THAN (63342604800) ENGINE = MyISAM)"},
		{`create table t1 (a int) partition by range (a)
		  (PARTITION p0 VALUES LESS THAN (63340531200) COMMENT 'xxx' ENGINE = MyISAM ,
		   PARTITION p1 VALUES LESS THAN (63342604800) ENGINE = MyISAM)`, true, "CREATE TABLE `t1` (`a` INT) PARTITION BY RANGE (`a`) (PARTITION `p0` VALUES LESS THAN (63340531200) COMMENT = 'xxx' ENGINE = MyISAM,PARTITION `p1` VALUES LESS THAN (63342604800) ENGINE = MyISAM)"},
		{`create table t (id int)
		    partition by range (id)
		    subpartition by key (id) subpartitions 2
		    (partition p0 values less than (42))`, true, "CREATE TABLE `t` (`id` INT) PARTITION BY RANGE (`id`) SUBPARTITION BY KEY (`id`) SUBPARTITIONS 2 (PARTITION `p0` VALUES LESS THAN (42))"},
		{`create table t (id int)
		    partition by range (id)
		    subpartition by hash (id)
		    (partition p0 values less than (42))`, true, "CREATE TABLE `t` (`id` INT) PARTITION BY RANGE (`id`) SUBPARTITION BY HASH (`id`) (PARTITION `p0` VALUES LESS THAN (42))"},
		{`create table t1 (a varchar(5), b int signed, c varchar(10), d datetime)
		partition by range columns(b,c)
		subpartition by hash(to_seconds(d))
		( partition p0 values less than (2, 'b'),
		  partition p1 values less than (4, 'd'),
		  partition p2 values less than (10, 'za'));`, true,
			"CREATE TABLE `t1` (`a` VARCHAR(5),`b` INT,`c` VARCHAR(10),`d` DATETIME) PARTITION BY RANGE COLUMNS (`b`,`c`) SUBPARTITION BY HASH (TO_SECONDS(`d`)) (PARTITION `p0` VALUES LESS THAN (2, _UTF8MB4'b'),PARTITION `p1` VALUES LESS THAN (4, _UTF8MB4'd'),PARTITION `p2` VALUES LESS THAN (10, _UTF8MB4'za'))"},
		{`CREATE TABLE t1 (a INT, b TIMESTAMP DEFAULT '0000-00-00 00:00:00')
ENGINE=INNODB PARTITION BY LINEAR HASH (a) PARTITIONS 1;`, true, "CREATE TABLE `t1` (`a` INT,`b` TIMESTAMP DEFAULT _UTF8MB4'0000-00-00 00:00:00') ENGINE = INNODB PARTITION BY LINEAR HASH (`a`) PARTITIONS 1"},

		// empty clause is valid only for HASH/KEY partitions
		{"create table t1 (a int) partition by hash (a) (partition x, partition y)", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY HASH (`a`) (PARTITION `x`,PARTITION `y`)"},
		{"create table t1 (a int) partition by key (a) (partition x, partition y)", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY KEY (`a`) (PARTITION `x`,PARTITION `y`)"},
		{"create table t1 (a int) partition by range (a) (partition x, partition y)", false, ""},
		{"create table t1 (a int) partition by list (a) (partition x, partition y)", false, ""},
		{"create table t1 (a int) partition by system_time (partition x, partition y)", false, ""},
		// VALUES LESS THAN clause is valid only for RANGE partitions
		{"create table t1 (a int) partition by hash (a) (partition x values less than (10))", false, ""},
		{"create table t1 (a int) partition by key (a) (partition x values less than (10))", false, ""},
		{"create table t1 (a int) partition by range (a) (partition x values less than (maxvalue))", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY RANGE (`a`) (PARTITION `x` VALUES LESS THAN (MAXVALUE))"},
		{"create table t1 (a int) partition by range (a) (partition x values less than (default))", false, ""},
		{"create table t (a varchar(100), b int) partition by list columns (a) (partition p1 values in ('a','b','DEFAULT'), partition pDef values in (default))", true, "CREATE TABLE `t` (`a` VARCHAR(100),`b` INT) PARTITION BY LIST COLUMNS (`a`) (PARTITION `p1` VALUES IN (_UTF8MB4'a', _UTF8MB4'b', _UTF8MB4'DEFAULT'),PARTITION `pDef` DEFAULT)"},
		{"create table t1 (a int) partition by range (a) (partition x values less than (10))", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY RANGE (`a`) (PARTITION `x` VALUES LESS THAN (10))"},
		{"create table t1 (a int) partition by list (a) (partition x values less than (10))", false, ""},
		{"create table t1 (a int) partition by system_time (partition x values less than (10))", false, ""},
		// VALUES IN clause is valid only for LIST partitions
		{"create table t1 (a int) partition by hash (a) (partition x values in (10))", false, ""},
		{"create table t1 (a int) partition by key (a) (partition x values in (10))", false, ""},
		{"create table t1 (a int) partition by range (a) (partition x values in (10))", false, ""},
		{"create table t1 (a int) partition by list (a) (partition x values in (10))", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY LIST (`a`) (PARTITION `x` VALUES IN (10))"},
		{"create table t1 (a int) partition by list (a) (partition x values in (default))", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY LIST (`a`) (PARTITION `x` DEFAULT)"},
		{"create table t1 (a int) partition by list (a) (partition x values in (maxvalue))", false, ""},
		{"create table t1 (a int) partition by list (a) (partition x values in (default, 10))", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY LIST (`a`) (PARTITION `x` VALUES IN (DEFAULT, 10))"},
		{"create table t1 (a int) partition by system_time (partition x values in (10))", false, ""},
		// HISTORY/CURRENT clauses are valid only for SYSTEM_TIME partitions
		{"create table t1 (a int) partition by hash (a) (partition x history, partition y current)", false, ""},
		{"create table t1 (a int) partition by key (a) (partition x history, partition y current)", false, ""},
		{"create table t1 (a int) partition by range (a) (partition x history, partition y current)", false, ""},
		{"create table t1 (a int) partition by list (a) (partition x history, partition y current)", false, ""},
		{"create table t1 (a int) partition by system_time (partition x history, partition y current)", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY SYSTEM_TIME (PARTITION `x` HISTORY,PARTITION `y` CURRENT)"},

		// LIST, RANGE and SYSTEM_TIME partitions all required definitions
		{"create table t1 (a int) partition by hash (a)", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY HASH (`a`) PARTITIONS 1"},
		{"create table t1 (a int) partition by key (a)", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY KEY (`a`) PARTITIONS 1"},
		{"create table t1 (a int) partition by range (a)", false, ""},
		{"create table t1 (a int) partition by list (a)", false, ""},
		{"create table t1 (a int) partition by system_time", false, ""},
		// SYSTEM_TIME required 2 or more partitions
		{"create table t1 (a int) partition by system_time (partition x history)", false, ""},
		{"create table t1 (a int) partition by system_time (partition x current)", false, ""},

		// number of columns and number of values in VALUES clauses must match
		{"create table t1 (a int, b int) partition by range (a) (partition x values less than (10, 20))", false, ""},
		{"create table t (id int) partition by range columns (id) (partition p0 values less than (1, 2))", false, ""},
		{"create table t1 (a int, b int) partition by range columns (a, b) (partition x values less than (10, 20))", true, "CREATE TABLE `t1` (`a` INT,`b` INT) PARTITION BY RANGE COLUMNS (`a`,`b`) (PARTITION `x` VALUES LESS THAN (10, 20))"},
		{"create table t1 (a int, b int) partition by range columns (a, b) (partition x values less than (10))", false, ""},
		{"create table t1 (a int, b int) partition by range columns (a, b) (partition x values less than maxvalue)", false, ""},
		{"create table t1 (a int, b int) partition by list (a) (partition x values in ((10, 20)))", false, ""},
		{"create table t1 (a int, b int) partition by list columns (a, b) (partition x values in ((10, 20)))", true, "CREATE TABLE `t1` (`a` INT,`b` INT) PARTITION BY LIST COLUMNS (`a`,`b`) (PARTITION `x` VALUES IN ((10, 20)))"},
		{"create table t1 (a int, b int) partition by list columns (a, b) (partition x values in (10, 20))", false, ""},
		{"create table t1 (a int, b int) partition by list columns (a, b) (partition x values in (10, (20, 30)))", false, ""},
		{"create table t1 (a int, b int) partition by list columns (a, b) (partition x values in ((10, 20), 30))", false, ""},
		{"create table t1 (a int, b int) partition by list columns (a, b) (partition x values in ((10, 20), (30, 40, 50)))", false, ""},

		// there must be at least one column/partition/value inside (...)
		{"create table t1 (a int) partition by hash (a) ()", false, ""},
		{"create table t1 (a int primary key) partition by key ()", true, "CREATE TABLE `t1` (`a` INT PRIMARY KEY) PARTITION BY KEY () PARTITIONS 1"},
		{"create table t1 (a int) partition by range columns () (partition x values less than maxvalue)", false, ""},
		{"create table t1 (a int) partition by list columns () (partition x default)", false, ""},
		{"create table t1 (a int) partition by range (a) (partition x values less than ())", false, ""},
		{"create table t1 (a int) partition by list (a) (partition x values in ())", false, ""},
		{"create table t1 (a int) partition by list (a) (partition x default)", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY LIST (`a`) (PARTITION `x` DEFAULT)"},

		// only hash and key subpartitions are allowed
		{"create table t1 (a int, b int) partition by range (a) subpartition by range (b) (partition x values less than maxvalue)", false, ""},

		// number of partitions/subpartitions must be matching
		{"create table t1 (a int) partition by hash (a) partitions 2 (partition x)", false, ""},
		{"create table t1 (a int) partition by hash (a) partitions 2 (partition x, partition y)", true, "CREATE TABLE `t1` (`a` INT) PARTITION BY HASH (`a`) (PARTITION `x`,PARTITION `y`)"},
		{"create table t1 (a int, b int) partition by range (a) subpartition by hash (b) subpartitions 2 (partition x values less than maxvalue (subpartition y))", false, ""},
		{
			"create table t1 (a int, b int) partition by range (a) subpartition by hash (b) subpartitions 2 (partition x values less than maxvalue (subpartition y, subpartition z))", true,
			"CREATE TABLE `t1` (`a` INT,`b` INT) PARTITION BY RANGE (`a`) SUBPARTITION BY HASH (`b`) SUBPARTITIONS 2 (PARTITION `x` VALUES LESS THAN (MAXVALUE) (SUBPARTITION `y`,SUBPARTITION `z`))",
		},
		{
			"create table t1 (a int, b int) partition by range (a) subpartition by hash (b) (partition x values less than (10) (subpartition y,subpartition z),partition a values less than (20) (subpartition b,subpartition c))", true,
			"CREATE TABLE `t1` (`a` INT,`b` INT) PARTITION BY RANGE (`a`) SUBPARTITION BY HASH (`b`) SUBPARTITIONS 2 (PARTITION `x` VALUES LESS THAN (10) (SUBPARTITION `y`,SUBPARTITION `z`),PARTITION `a` VALUES LESS THAN (20) (SUBPARTITION `b`,SUBPARTITION `c`))",
		},
		{"create table t1 (a int, b int) partition by range (a) subpartition by hash (b) (partition x values less than (10) (subpartition y),partition a values less than (20) (subpartition b,subpartition c))", false, ""},
		{"create table t1 (a int, b int) partition by range (a) (partition x values less than (10) (subpartition y))", false, ""},
		{"create table t1 (a int) partition by hash (a) partitions 0", false, ""},
		{"create table t1 (a int, b int) partition by range (a) subpartition by hash (b) subpartitions 0 (partition x values less than (10))", false, ""},

		// other partition tests
		{"create table t1 (a int) partition by system_time interval 7 day limit 50000 (partition x history, partition y current)", false, ""},
		{
			"create table t1 (a int) partition by system_time interval 7 day (partition x history, partition y current)", true,
			"CREATE TABLE `t1` (`a` INT) PARTITION BY SYSTEM_TIME INTERVAL 7 DAY (PARTITION `x` HISTORY,PARTITION `y` CURRENT)",
		},
		{
			"create table t1 (a int) partition by system_time limit 50000 (partition x history, partition y current)", true,
			"CREATE TABLE `t1` (`a` INT) PARTITION BY SYSTEM_TIME LIMIT 50000 (PARTITION `x` HISTORY,PARTITION `y` CURRENT)",
		},
		{
			"create table t1 (a int) partition by hash(a) (partition x engine InnoDB comment 'xxxx' data directory '/var/data' index directory '/var/index' max_rows 70000 min_rows 50 tablespace `innodb_file_per_table` nodegroup 255)", true,
			"CREATE TABLE `t1` (`a` INT) PARTITION BY HASH (`a`) (PARTITION `x` ENGINE = InnoDB COMMENT = 'xxxx' DATA DIRECTORY = '/var/data' INDEX DIRECTORY = '/var/index' MAX_ROWS = 70000 MIN_ROWS = 50 TABLESPACE = `innodb_file_per_table` NODEGROUP = 255)",
		},
		{
			"create table t1 (a int, b int) partition by range(a) subpartition by hash(b) (partition x values less than maxvalue (subpartition y engine InnoDB comment 'xxxx' data directory '/var/data' index directory '/var/index' max_rows 70000 min_rows 50 tablespace `innodb_file_per_table` nodegroup 255))", true,
			"CREATE TABLE `t1` (`a` INT,`b` INT) PARTITION BY RANGE (`a`) SUBPARTITION BY HASH (`b`) SUBPARTITIONS 1 (PARTITION `x` VALUES LESS THAN (MAXVALUE) (SUBPARTITION `y` ENGINE = InnoDB COMMENT = 'xxxx' DATA DIRECTORY = '/var/data' INDEX DIRECTORY = '/var/index' MAX_ROWS = 70000 MIN_ROWS = 50 TABLESPACE = `innodb_file_per_table` NODEGROUP = 255))",
		},
	}
	RunTest(t, table, false)

	// Check comment content.
	p := parser.New()
	stmt, err := p.ParseOneStmt("create table t (id int) partition by range (id) (partition p0 values less than (10) comment 'check')", "", "")
	require.NoError(t, err)
	createTable := stmt.(*ast.CreateTableStmt)
	comment, ok := createTable.Partition.Definitions[0].Comment()
	require.True(t, ok)
	require.Equal(t, "check", comment)
}

func TestTablePartitionNameList(t *testing.T) {
	table := []testCase{
		{`select * from t partition (p0,p1)`, true, ""},
	}

	p := parser.New()
	for _, tbl := range table {
		stmt, _, err := p.Parse(tbl.src, "", "")
		require.NoError(t, err)

		sel := stmt[0].(*ast.SelectStmt)
		source, ok := sel.From.TableRefs.Left.(*ast.TableSource)
		require.True(t, ok)
		tableName, ok := source.Source.(*ast.TableName)
		require.True(t, ok)
		require.Len(t, tableName.PartitionNames, 2)
		require.Equal(t, ast.CIStr{O: "p0", L: "p0"}, tableName.PartitionNames[0])
		require.Equal(t, ast.CIStr{O: "p1", L: "p1"}, tableName.PartitionNames[1])
	}
}

func TestNotExistsSubquery(t *testing.T) {
	table := []testCase{
		{`select * from t1 where not exists (select * from t2 where t1.a = t2.a)`, true, ""},
	}

	p := parser.New()
	for _, tbl := range table {
		stmt, _, err := p.Parse(tbl.src, "", "")
		require.NoError(t, err)

		sel := stmt[0].(*ast.SelectStmt)
		exists, ok := sel.Where.(*ast.ExistsSubqueryExpr)
		require.True(t, ok)
		require.Equal(t, tbl.ok, exists.Not)
	}
}

func TestWindowFunctionIdentifier(t *testing.T) {
	//nolint: prealloc
	var table []testCase
	for key := range parser.WindowFuncTokenMapForTest {
		table = append(table, testCase{fmt.Sprintf("select 1 %s", key), false, fmt.Sprintf("SELECT 1 AS `%s`", key)})
	}
	RunTest(t, table, true)

	for i := range table {
		table[i].ok = true
	}
	RunTest(t, table, false)
}

func TestWindowFunctions(t *testing.T) {
	table := []testCase{
		// For window function descriptions.
		// See https://dev.mysql.com/doc/refman/8.0/en/window-function-descriptions.html
		{`SELECT CUME_DIST() OVER w FROM t;`, true, "SELECT CUME_DIST() OVER `w` FROM `t`"},
		{`SELECT DENSE_RANK() OVER (w) FROM t;`, true, "SELECT DENSE_RANK() OVER (`w`) FROM `t`"},
		{`SELECT FIRST_VALUE(val) OVER w FROM t;`, true, "SELECT FIRST_VALUE(`val`) OVER `w` FROM `t`"},
		{`SELECT FIRST_VALUE(val) RESPECT NULLS OVER w FROM t;`, true, "SELECT FIRST_VALUE(`val`) OVER `w` FROM `t`"},
		{`SELECT FIRST_VALUE(val) IGNORE NULLS OVER w FROM t;`, true, "SELECT FIRST_VALUE(`val`) IGNORE NULLS OVER `w` FROM `t`"},
		{`SELECT LAG(val) OVER (w) FROM t;`, true, "SELECT LAG(`val`) OVER (`w`) FROM `t`"},
		{`SELECT LAG(val, 1) OVER (w) FROM t;`, true, "SELECT LAG(`val`, 1) OVER (`w`) FROM `t`"},
		{`SELECT LAG(val, 1, def) OVER (w) FROM t;`, true, "SELECT LAG(`val`, 1, `def`) OVER (`w`) FROM `t`"},
		{`SELECT LAST_VALUE(val) OVER (w) FROM t;`, true, "SELECT LAST_VALUE(`val`) OVER (`w`) FROM `t`"},
		{`SELECT LEAD(val) OVER w FROM t;`, true, "SELECT LEAD(`val`) OVER `w` FROM `t`"},
		{`SELECT LEAD(val, 1) OVER w FROM t;`, true, "SELECT LEAD(`val`, 1) OVER `w` FROM `t`"},
		{`SELECT LEAD(val, 1, def) OVER w FROM t;`, true, "SELECT LEAD(`val`, 1, `def`) OVER `w` FROM `t`"},
		{`SELECT NTH_VALUE(val, 233) OVER w FROM t;`, true, "SELECT NTH_VALUE(`val`, 233) OVER `w` FROM `t`"},
		{`SELECT NTH_VALUE(val, 233) FROM FIRST OVER w FROM t;`, true, "SELECT NTH_VALUE(`val`, 233) OVER `w` FROM `t`"},
		{`SELECT NTH_VALUE(val, 233) FROM LAST OVER w FROM t;`, true, "SELECT NTH_VALUE(`val`, 233) FROM LAST OVER `w` FROM `t`"},
		{`SELECT NTH_VALUE(val, 233) FROM LAST IGNORE NULLS OVER w FROM t;`, true, "SELECT NTH_VALUE(`val`, 233) FROM LAST IGNORE NULLS OVER `w` FROM `t`"},
		{`SELECT NTH_VALUE(val) OVER w FROM t;`, false, ""},
		{`SELECT NTILE(233) OVER (w) FROM t;`, true, "SELECT NTILE(233) OVER (`w`) FROM `t`"},
		{`SELECT PERCENT_RANK() OVER (w) FROM t;`, true, "SELECT PERCENT_RANK() OVER (`w`) FROM `t`"},
		{`SELECT RANK() OVER (w) FROM t;`, true, "SELECT RANK() OVER (`w`) FROM `t`"},
		{`SELECT ROW_NUMBER() OVER (w) FROM t;`, true, "SELECT ROW_NUMBER() OVER (`w`) FROM `t`"},
		{`SELECT n, LAG(n, 1, 0) OVER (w), LEAD(n, 1, 0) OVER w, n + LAG(n, 1, 0) OVER (w) FROM fib;`, true, "SELECT `n`,LAG(`n`, 1, 0) OVER (`w`),LEAD(`n`, 1, 0) OVER `w`,`n`+LAG(`n`, 1, 0) OVER (`w`) FROM `fib`"},

		// For window function concepts and syntax.
		// See https://dev.mysql.com/doc/refman/8.0/en/window-functions-usage.html
		{`SELECT SUM(profit) OVER(PARTITION BY country) AS country_profit FROM sales;`, true, "SELECT SUM(`profit`) OVER (PARTITION BY `country`) AS `country_profit` FROM `sales`"},
		{`SELECT SUM(profit) OVER() AS country_profit FROM sales;`, true, "SELECT SUM(`profit`) OVER () AS `country_profit` FROM `sales`"},
		{`SELECT AVG(profit) OVER() AS country_profit FROM sales;`, true, "SELECT AVG(`profit`) OVER () AS `country_profit` FROM `sales`"},
		{`SELECT BIT_XOR(profit) OVER() AS country_profit FROM sales;`, true, "SELECT BIT_XOR(`profit`) OVER () AS `country_profit` FROM `sales`"},
		{`SELECT COUNT(profit) OVER() AS country_profit FROM sales;`, true, "SELECT COUNT(`profit`) OVER () AS `country_profit` FROM `sales`"},
		{`SELECT COUNT(ALL profit) OVER() AS country_profit FROM sales;`, true, "SELECT COUNT(`profit`) OVER () AS `country_profit` FROM `sales`"},
		{`SELECT COUNT(*) OVER() AS country_profit FROM sales;`, true, "SELECT COUNT(1) OVER () AS `country_profit` FROM `sales`"},
		{`SELECT MAX(profit) OVER() AS country_profit FROM sales;`, true, "SELECT MAX(`profit`) OVER () AS `country_profit` FROM `sales`"},
		{`SELECT MIN(profit) OVER() AS country_profit FROM sales;`, true, "SELECT MIN(`profit`) OVER () AS `country_profit` FROM `sales`"},
		{`SELECT SUM(profit) OVER() AS country_profit FROM sales;`, true, "SELECT SUM(`profit`) OVER () AS `country_profit` FROM `sales`"},
		{`SELECT ROW_NUMBER() OVER(PARTITION BY country) AS row_num1 FROM sales;`, true, "SELECT ROW_NUMBER() OVER (PARTITION BY `country`) AS `row_num1` FROM `sales`"},
		{`SELECT ROW_NUMBER() OVER(PARTITION BY country, d ORDER BY year, product) AS row_num2 FROM sales;`, true, "SELECT ROW_NUMBER() OVER (PARTITION BY `country`, `d` ORDER BY `year`,`product`) AS `row_num2` FROM `sales`"},

		// For window function frame specification.
		// See https://dev.mysql.com/doc/refman/8.0/en/window-functions-frames.html
		{`SELECT SUM(val) OVER (PARTITION BY subject ORDER BY time ROWS UNBOUNDED PRECEDING) FROM t;`, true, "SELECT SUM(`val`) OVER (PARTITION BY `subject` ORDER BY `time` ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) FROM `t`"},
		{`SELECT AVG(val) OVER (PARTITION BY subject ORDER BY time ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING) FROM t;`, true, "SELECT AVG(`val`) OVER (PARTITION BY `subject` ORDER BY `time` ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING) FROM `t`"},
		{`SELECT AVG(val) OVER (ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING) FROM t;`, true, "SELECT AVG(`val`) OVER (ROWS BETWEEN 1 PRECEDING AND 1 FOLLOWING) FROM `t`"},
		{`SELECT AVG(val) OVER (ROWS BETWEEN 1 PRECEDING AND UNBOUNDED FOLLOWING) FROM t;`, true, "SELECT AVG(`val`) OVER (ROWS BETWEEN 1 PRECEDING AND UNBOUNDED FOLLOWING) FROM `t`"},
		{`SELECT AVG(val) OVER (RANGE BETWEEN INTERVAL 5 DAY PRECEDING AND INTERVAL '2:30' MINUTE_SECOND FOLLOWING) FROM t;`, true, "SELECT AVG(`val`) OVER (RANGE BETWEEN INTERVAL 5 DAY PRECEDING AND INTERVAL _UTF8MB4'2:30' MINUTE_SECOND FOLLOWING) FROM `t`"},
		{`SELECT AVG(val) OVER (RANGE BETWEEN CURRENT ROW AND CURRENT ROW) FROM t;`, true, "SELECT AVG(`val`) OVER (RANGE BETWEEN CURRENT ROW AND CURRENT ROW) FROM `t`"},
		{`SELECT AVG(val) OVER (RANGE CURRENT ROW) FROM t;`, true, "SELECT AVG(`val`) OVER (RANGE BETWEEN CURRENT ROW AND CURRENT ROW) FROM `t`"},

		// For named windows.
		// See https://dev.mysql.com/doc/refman/8.0/en/window-functions-named-windows.html
		{`SELECT RANK() OVER (w) FROM t WINDOW w AS (ORDER BY val);`, true, "SELECT RANK() OVER (`w`) FROM `t` WINDOW `w` AS (ORDER BY `val`)"},
		{`SELECT RANK() OVER w FROM t WINDOW w AS ();`, true, "SELECT RANK() OVER `w` FROM `t` WINDOW `w` AS ()"},
		{`SELECT FIRST_VALUE(year) OVER (w ORDER BY year) AS first FROM sales WINDOW w AS (PARTITION BY country);`, true, "SELECT FIRST_VALUE(`year`) OVER (`w` ORDER BY `year`) AS `first` FROM `sales` WINDOW `w` AS (PARTITION BY `country`)"},
		{`SELECT RANK() OVER (w1) FROM t WINDOW w1 AS (w2), w2 AS (), w3 AS (w1);`, true, "SELECT RANK() OVER (`w1`) FROM `t` WINDOW `w1` AS (`w2`),`w2` AS (),`w3` AS (`w1`)"},
		{`SELECT RANK() OVER w1 FROM t WINDOW w1 AS (w2), w2 AS (w3), w3 AS (w1);`, true, "SELECT RANK() OVER `w1` FROM `t` WINDOW `w1` AS (`w2`),`w2` AS (`w3`),`w3` AS (`w1`)"},

		// For TSO functions
		{`select tidb_parse_tso(1)`, true, "SELECT TIDB_PARSE_TSO(1)"},
		{`select tidb_parse_tso_logical(1)`, true, "SELECT TIDB_PARSE_TSO_LOGICAL(1)"},
		{`select tidb_bounded_staleness('2015-09-21 00:07:01', NOW())`, true, "SELECT TIDB_BOUNDED_STALENESS(_UTF8MB4'2015-09-21 00:07:01', NOW())"},
		{`select tidb_bounded_staleness(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())`, true, "SELECT TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())"},
		{`select tidb_bounded_staleness('2015-09-21 00:07:01', '2021-04-27 11:26:13')`, true, "SELECT TIDB_BOUNDED_STALENESS(_UTF8MB4'2015-09-21 00:07:01', _UTF8MB4'2021-04-27 11:26:13')"},

		{`select from_unixtime(404411537129996288)`, true, "SELECT FROM_UNIXTIME(404411537129996288)"},
		{`select from_unixtime(404411537129996288.22)`, true, "SELECT FROM_UNIXTIME(404411537129996288.22)"},
	}
	RunTest(t, table, true)
}

type windowFrameBoundChecker struct {
	fb     *ast.FrameBound
	exprRc int
	unit   ast.TimeUnitType
	t      *testing.T
}

// Enter implements ast.Visitor interface.
func (wfc *windowFrameBoundChecker) Enter(inNode ast.Node) (outNode ast.Node, skipChildren bool) {
	if _, ok := inNode.(*ast.FrameBound); ok {
		wfc.fb = inNode.(*ast.FrameBound)
		if wfc.fb.Unit != ast.TimeUnitInvalid {
			_, ok := wfc.fb.Expr.(ast.ValueExpr)
			require.False(wfc.t, ok)
		}
	}
	return inNode, false
}

// Leave implements ast.Visitor interface.
func (wfc *windowFrameBoundChecker) Leave(inNode ast.Node) (node ast.Node, ok bool) {
	if _, ok := inNode.(*ast.FrameBound); ok {
		wfc.fb = nil
	}
	if wfc.fb != nil {
		if inNode == wfc.fb.Expr {
			wfc.exprRc++
		}
		wfc.unit = wfc.fb.Unit
	}
	return inNode, true
}

// For issue #51
// See https://github.com/pingcap/parser/pull/51 for details
func TestVisitFrameBound(t *testing.T) {
	p := parser.New()
	p.EnableWindowFunc(true)
	table := []struct {
		s      string
		exprRc int
		unit   ast.TimeUnitType
	}{
		{`SELECT AVG(val) OVER (RANGE INTERVAL 1+3 MINUTE_SECOND PRECEDING) FROM t;`, 1, ast.TimeUnitMinuteSecond},
		{`SELECT AVG(val) OVER (RANGE 5 PRECEDING) FROM t;`, 1, ast.TimeUnitInvalid},
		{`SELECT AVG(val) OVER () FROM t;`, 0, ast.TimeUnitInvalid},
	}
	for _, tbl := range table {
		stmt, err := p.ParseOneStmt(tbl.s, "", "")
		require.NoError(t, err)
		checker := windowFrameBoundChecker{t: t}
		stmt.Accept(&checker)
		require.Equal(t, tbl.exprRc, checker.exprRc)
		require.Equal(t, tbl.unit, checker.unit)
	}
}

func TestFieldText(t *testing.T) {
	p := parser.New()
	stmts, _, err := p.Parse("select a from t", "", "")
	require.NoError(t, err)
	tmp := stmts[0].(*ast.SelectStmt)
	require.Equal(t, "a", tmp.Fields.Fields[0].Text())

	sqls := []string{
		"trace select a from t",
		"trace format = 'row' select a from t",
		"trace format = 'json' select a from t",
	}
	for _, sql := range sqls {
		stmts, _, err = p.Parse(sql, "", "")
		require.NoError(t, err)
		traceStmt := stmts[0].(*ast.TraceStmt)
		require.Equal(t, sql, traceStmt.Text())
		require.Equal(t, "select a from t", traceStmt.Stmt.Text())
	}
}

// See https://github.com/pingcap/parser/issue/94
func TestQuotedSystemVariables(t *testing.T) {
	p := parser.New()

	st, err := p.ParseOneStmt(
		"select @@Sql_Mode, @@`SQL_MODE`, @@session.`sql_mode`, @@global.`s ql``mode`, @@session.'sql\\nmode', @@local.\"sql\\\"mode\";",
		"",
		"",
	)
	require.NoError(t, err)
	ss := st.(*ast.SelectStmt)
	expected := []*ast.VariableExpr{
		{
			Name:          "sql_mode",
			IsGlobal:      false,
			IsSystem:      true,
			ExplicitScope: false,
		},
		{
			Name:          "sql_mode",
			IsGlobal:      false,
			IsSystem:      true,
			ExplicitScope: false,
		},
		{
			Name:          "sql_mode",
			IsGlobal:      false,
			IsSystem:      true,
			ExplicitScope: true,
		},
		{
			Name:          "s ql`mode",
			IsGlobal:      true,
			IsSystem:      true,
			ExplicitScope: true,
		},
		{
			Name:          "sql\nmode",
			IsGlobal:      false,
			IsSystem:      true,
			ExplicitScope: true,
		},
		{
			Name:          `sql"mode`,
			IsGlobal:      false,
			IsSystem:      true,
			ExplicitScope: true,
		},
	}

	require.Len(t, ss.Fields.Fields, len(expected))
	for i, field := range ss.Fields.Fields {
		ve := field.Expr.(*ast.VariableExpr)
		comment := fmt.Sprintf("field %d, ve = %v", i, ve)
		require.Equal(t, expected[i].Name, ve.Name, comment)
		require.Equal(t, expected[i].IsGlobal, ve.IsGlobal, comment)
		require.Equal(t, expected[i].IsSystem, ve.IsSystem, comment)
		require.Equal(t, expected[i].ExplicitScope, ve.ExplicitScope, comment)
	}
}

// See https://github.com/pingcap/parser/issue/95
func TestQuotedVariableColumnName(t *testing.T) {
	p := parser.New()

	st, err := p.ParseOneStmt(
		"select @abc, @`abc`, @'aBc', @\"AbC\", @6, @`6`, @'6', @\"6\", @@sql_mode, @@`sql_mode`, @;",
		"",
		"",
	)
	require.NoError(t, err)
	ss := st.(*ast.SelectStmt)
	expected := []string{
		"@abc",
		"@`abc`",
		"@'aBc'",
		`@"AbC"`,
		"@6",
		"@`6`",
		"@'6'",
		`@"6"`,
		"@@sql_mode",
		"@@`sql_mode`",
		"@",
	}

	require.Len(t, ss.Fields.Fields, len(expected))
	for i, field := range ss.Fields.Fields {
		require.Equal(t, expected[i], field.Text())
	}
}

func TestCharset(t *testing.T) {
	p := parser.New()

	st, err := p.ParseOneStmt("ALTER SCHEMA GLOBAL DEFAULT CHAR SET utf8mb4", "", "")
	require.NoError(t, err)
	require.NotNil(t, st.(*ast.AlterDatabaseStmt))
	st, err = p.ParseOneStmt("ALTER DATABASE CHAR SET = utf8mb4", "", "")
	require.NoError(t, err)
	require.NotNil(t, st.(*ast.AlterDatabaseStmt))
	st, err = p.ParseOneStmt("ALTER DATABASE DEFAULT CHAR SET = utf8mb4", "", "")
	require.NoError(t, err)
	require.NotNil(t, st.(*ast.AlterDatabaseStmt))
}

func TestUnderscoreCharset(t *testing.T) {
	p := parser.New()
	tests := []struct {
		cs        string
		parseFail bool
		unSupport bool
	}{
		{"utf8", false, false},
		{"gbk", false, true},
		{"ujis", false, true},
		{"gbk1", true, true},
		{"ujisx", true, true},
	}
	for _, tt := range tests {
		sql := fmt.Sprintf("select hex(_%s '3F')", tt.cs)
		_, err := p.ParseOneStmt(sql, "", "")
		if tt.parseFail {
			require.EqualError(t, err, fmt.Sprintf("line 1 column %d near \"'3F')\" ", len(tt.cs)+17))
		} else if tt.unSupport {
			require.EqualError(t, err, ast.ErrUnknownCharacterSet.GenWithStack("Unsupported character introducer: '%-.64s'", tt.cs).Error())
		} else {
			require.NoError(t, err)
		}
	}
}

func TestFulltextSearch(t *testing.T) {
	p := parser.New()

	st, err := p.ParseOneStmt("SELECT * FROM fulltext_test WHERE MATCH(content) AGAINST('search')", "", "")
	require.NoError(t, err)
	require.NotNil(t, st.(*ast.SelectStmt))

	st, err = p.ParseOneStmt("SELECT * FROM fulltext_test WHERE MATCH() AGAINST('search')", "", "")
	require.Error(t, err)
	require.Nil(t, st)

	st, err = p.ParseOneStmt("SELECT * FROM fulltext_test WHERE MATCH(content) AGAINST()", "", "")
	require.Error(t, err)
	require.Nil(t, st)

	st, err = p.ParseOneStmt("SELECT * FROM fulltext_test WHERE MATCH(content) AGAINST('search' IN)", "", "")
	require.Error(t, err)
	require.Nil(t, st)

	st, err = p.ParseOneStmt("SELECT * FROM fulltext_test WHERE MATCH(content) AGAINST('search' IN BOOLEAN MODE WITH QUERY EXPANSION)", "", "")
	require.Error(t, err)
	require.Nil(t, st)

	st, err = p.ParseOneStmt("SELECT * FROM fulltext_test WHERE MATCH(title,content) AGAINST('search' IN NATURAL LANGUAGE MODE)", "", "")
	require.NoError(t, err)
	require.NotNil(t, st.(*ast.SelectStmt))
	writer := bytes.NewBufferString("")
	st.(*ast.SelectStmt).Where.Format(writer)
	require.Equal(t, "MATCH(title,content) AGAINST(\"search\")", writer.String())

	st, err = p.ParseOneStmt("SELECT * FROM fulltext_test WHERE MATCH(title,content) AGAINST('search' IN BOOLEAN MODE)", "", "")
	require.NoError(t, err)
	require.NotNil(t, st.(*ast.SelectStmt))
	writer.Reset()
	st.(*ast.SelectStmt).Where.Format(writer)
	require.Equal(t, "MATCH(title,content) AGAINST(\"search\" IN BOOLEAN MODE)", writer.String())

	st, err = p.ParseOneStmt("SELECT * FROM fulltext_test WHERE MATCH(title,content) AGAINST('search' WITH QUERY EXPANSION)", "", "")
	require.NoError(t, err)
	require.NotNil(t, st.(*ast.SelectStmt))
	writer.Reset()
	st.(*ast.SelectStmt).Where.Format(writer)
	require.Equal(t, "MATCH(title,content) AGAINST(\"search\" WITH QUERY EXPANSION)", writer.String())
}

func TestStartTransaction(t *testing.T) {
	cases := []testCase{
		{"START TRANSACTION READ WRITE", true, "START TRANSACTION"},
		{"START TRANSACTION WITH CONSISTENT SNAPSHOT", true, "START TRANSACTION"},
		{"START TRANSACTION WITH CAUSAL CONSISTENCY ONLY", true, "START TRANSACTION WITH CAUSAL CONSISTENCY ONLY"},
		{"START TRANSACTION READ ONLY", true, "START TRANSACTION READ ONLY"},
		{"START TRANSACTION READ ONLY AS OF", false, ""},
		{"START TRANSACTION READ ONLY AS OF TIMESTAMP", false, ""},
		{"START TRANSACTION READ ONLY AS OF TIMESTAMP '2015-09-21 00:07:01'", true, "START TRANSACTION READ ONLY AS OF TIMESTAMP _UTF8MB4'2015-09-21 00:07:01'"},
		{"START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(_UTF8MB4'2015-09-21 00:07:01', NOW())", true, "START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(_UTF8MB4'2015-09-21 00:07:01', NOW())"},
		{"START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())", true, "START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())"},
		{"START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(_UTF8MB4'2015-09-21 00:07:01', '2021-04-27 11:26:13')", true, "START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(_UTF8MB4'2015-09-21 00:07:01', _UTF8MB4'2021-04-27 11:26:13')"},
	}

	RunTest(t, cases, false)
}

func TestSignedInt64OutOfRange(t *testing.T) {
	p := parser.New()
	cases := []string{
		"recover table by job 18446744073709551612",
		"recover table t 18446744073709551612",
		"admin check index t idx (0, 18446744073709551612)",
		"create user abc@def with max_queries_per_hour 18446744073709551612",
	}

	for _, s := range cases {
		_, err := p.ParseOneStmt(s, "", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "out of range")
	}
}

// CleanNodeText set the text of node and all child node empty.
// For test only.
func CleanNodeText(node ast.Node) {
	var cleaner nodeTextCleaner
	node.Accept(&cleaner)
}

// nodeTextCleaner clean the text of a node and it's child node.
// For test only.
type nodeTextCleaner struct {
}

func cleanPartition(n ast.Node) {
	if p, ok := n.(*ast.PartitionOptions); ok && p != nil {
		var tmpCleaner nodeTextCleaner
		if p.Interval != nil {
			p.Interval.SetText(nil, "")
			p.Interval.SetOriginTextPosition(0)
			p.Interval.IntervalExpr.Expr.Accept(&tmpCleaner)
			if p.Interval.FirstRangeEnd != nil {
				(*p.Interval.FirstRangeEnd).Accept(&tmpCleaner)
			}
			if p.Interval.LastRangeEnd != nil {
				(*p.Interval.LastRangeEnd).Accept(&tmpCleaner)
			}
		}
	}
}

// Enter implements Visitor interface.
func (checker *nodeTextCleaner) Enter(in ast.Node) (out ast.Node, skipChildren bool) {
	in.SetText(nil, "")
	in.SetOriginTextPosition(0)
	if v, ok := in.(ast.ValueExpr); ok && v != nil {
		tpFlag := v.GetType().GetFlag()
		if tpFlag&mysql.UnderScoreCharsetFlag != 0 {
			// ignore underscore charset flag to let `'abc' = _utf8'abc'` pass
			tpFlag ^= mysql.UnderScoreCharsetFlag
			v.GetType().SetFlag(tpFlag)
		}
	}

	switch node := in.(type) {
	case *ast.CreateTableStmt:
		for _, opt := range node.Options {
			switch opt.Tp {
			case ast.TableOptionCharset:
				opt.StrValue = strings.ToUpper(opt.StrValue)
			case ast.TableOptionCollate:
				opt.StrValue = strings.ToUpper(opt.StrValue)
			}
		}
		for _, col := range node.Cols {
			col.Tp.SetCharset(strings.ToUpper(col.Tp.GetCharset()))
			col.Tp.SetCollate(strings.ToUpper(col.Tp.GetCollate()))

			for i, option := range col.Options {
				if option.Tp == 0 && option.Expr == nil && !option.Stored && option.Refer == nil {
					col.Options = slices.Delete(col.Options, i, i+1)
				}
			}
		}
	case *ast.DeleteStmt:
		for _, tableHint := range node.TableHints {
			tableHint.HintName.O = ""
		}
	case *ast.UpdateStmt:
		for _, tableHint := range node.TableHints {
			tableHint.HintName.O = ""
		}
	case *ast.Constraint:
		if node.Option != nil {
			if node.Option.KeyBlockSize == 0x0 && node.Option.Tp == 0 && node.Option.Comment == "" {
				node.Option = nil
			}
		}
	case *ast.FuncCallExpr:
		node.FnName.O = strings.ToLower(node.FnName.O)
		node.SetOriginTextPosition(0)
	case *ast.AggregateFuncExpr:
		node.F = strings.ToLower(node.F)
	case *ast.SelectField:
		node.Offset = 0
	case *test_driver.ValueExpr:
		if node.Kind() == test_driver.KindMysqlDecimal {
			_ = node.GetMysqlDecimal().FromString(node.GetMysqlDecimal().ToString())
		}
	case *ast.GrantStmt:
		var privs []*ast.PrivElem
		for _, v := range node.Privs {
			if v.Priv != 0 {
				privs = append(privs, v)
			}
		}
		node.Privs = privs
	case *ast.AlterTableStmt:
		var specs []*ast.AlterTableSpec
		for _, v := range node.Specs {
			if v.Tp != 0 && !(v.Tp == ast.AlterTableOption && len(v.Options) == 0) {
				specs = append(specs, v)
			}
		}
		node.Specs = specs
	case *ast.Join:
		node.ExplicitParens = false
	case *ast.ColumnDef:
		node.Tp.CleanElemIsBinaryLit()
	case *ast.PartitionOptions:
		cleanPartition(node)
	}
	return in, false
}

// Leave implements Visitor interface.
func (checker *nodeTextCleaner) Leave(in ast.Node) (out ast.Node, ok bool) {
	return in, true
}

// For BRIE
func TestBRIE(t *testing.T) {
	table := []testCase{
		{"BACKUP DATABASE a TO 'local:///tmp/archive01/'", true, "BACKUP DATABASE `a` TO 'local:///tmp/archive01/'"},
		{"BACKUP SCHEMA a TO 'local:///tmp/archive01/'", true, "BACKUP DATABASE `a` TO 'local:///tmp/archive01/'"},
		{"BACKUP DATABASE a,b,c TO 'noop://'", true, "BACKUP DATABASE `a`, `b`, `c` TO 'noop://'"},
		{"BACKUP DATABASE a.b TO 'noop://'", false, ""},
		{"BACKUP DATABASE * TO 'noop://'", true, "BACKUP DATABASE * TO 'noop://'"},
		{"BACKUP DATABASE *, a TO 'noop://'", false, ""},
		{"BACKUP DATABASE a, * TO 'noop://'", false, ""},
		{"BACKUP DATABASE TO 'noop://'", false, ""},
		{"BACKUP TABLE a TO 'noop://' checksum_concurrency 4 compression_level 4 ignore_stats 1 compression_type 'lz4'", true, "BACKUP TABLE `a` TO 'noop://' CHECKSUM_CONCURRENCY = 4 COMPRESSION_LEVEL = 4 IGNORE_STATS = 1 COMPRESSION_TYPE = 'lz4'"},
		{"RESTORE TABLE a FROM 'noop://' checksum_concurrency 4 wait_tiflash_ready 1 with_sys_table 1", true, "RESTORE TABLE `a` FROM 'noop://' CHECKSUM_CONCURRENCY = 4 WAIT_TIFLASH_READY = 1 WITH_SYS_TABLE = 1"},
		{"BACKUP TABLE a.b TO 'noop://'", true, "BACKUP TABLE `a`.`b` TO 'noop://'"},
		{"BACKUP TABLE a.b,c.d,e TO 'noop://'", true, "BACKUP TABLE `a`.`b`, `c`.`d`, `e` TO 'noop://'"},
		{"BACKUP TABLE a.* TO 'noop://'", false, ""},
		{"BACKUP TABLE * TO 'noop://'", false, ""},
		{"BACKUP TABLE TO 'noop://'", false, ""},
		{"RESTORE DATABASE * FROM 's3://bucket/path/'", true, "RESTORE DATABASE * FROM 's3://bucket/path/'"},

		{"BACKUP DATABASE * TO 'noop://' LAST_BACKUP = '2020-02-02 14:14:14'", true, "BACKUP DATABASE * TO 'noop://' LAST_BACKUP = '2020-02-02 14:14:14'"},
		{"BACKUP DATABASE * TO 'noop://' LAST_BACKUP = 1234567890", true, "BACKUP DATABASE * TO 'noop://' LAST_BACKUP = 1234567890"},

		{"backup database * to 'noop://' rate_limit 500 MB/second snapshot 5 minute ago", true, "BACKUP DATABASE * TO 'noop://' RATE_LIMIT = 500 MB/SECOND SNAPSHOT = 300000000 MICROSECOND AGO"},
		{"backup database * to 'noop://' snapshot = '2020-03-18 18:13:54'", true, "BACKUP DATABASE * TO 'noop://' SNAPSHOT = '2020-03-18 18:13:54'"},
		{"backup database * to 'noop://' snapshot = 1234567890", true, "BACKUP DATABASE * TO 'noop://' SNAPSHOT = 1234567890"},
		{"restore table g from 'noop://' concurrency 40 checksum 0 online 1", true, "RESTORE TABLE `g` FROM 'noop://' CONCURRENCY = 40 CHECKSUM = OFF ONLINE = 1"},
		{
			"backup table x to 's3://bucket/path/?endpoint=https://test-cluster-s3.local&access-key=aaaaaaaaa&secret-access-key=bbbbbbbb&force-path-style=1'",
			true,
			"BACKUP TABLE `x` TO 's3://bucket/path/?endpoint=https://test-cluster-s3.local&access-key=aaaaaaaaa&secret-access-key=bbbbbbbb&force-path-style=1'",
		},
		{
			"backup database * to 's3://bucket/path/?provider=alibaba&region=us-west-9&storage-class=glacier&sse=AES256&acl=authenticated-read&use-accelerate-endpoint=1' send_credentials_to_tikv = 1",
			true,
			"BACKUP DATABASE * TO 's3://bucket/path/?provider=alibaba&region=us-west-9&storage-class=glacier&sse=AES256&acl=authenticated-read&use-accelerate-endpoint=1' SEND_CREDENTIALS_TO_TIKV = 1",
		},
		{
			"restore database * from 'gcs://bucket/path/?endpoint=https://test-cluster.gcs.local&storage-class=coldline&predefined-acl=OWNER&credentials-file=/data/private/creds.json'",
			true,
			"RESTORE DATABASE * FROM 'gcs://bucket/path/?endpoint=https://test-cluster.gcs.local&storage-class=coldline&predefined-acl=OWNER&credentials-file=/data/private/creds.json'",
		},
		{"restore table g from 'noop://' checksum off", true, "RESTORE TABLE `g` FROM 'noop://' CHECKSUM = OFF"},
		{"restore table g from 'noop://' checksum optional", true, "RESTORE TABLE `g` FROM 'noop://' CHECKSUM = OPTIONAL"},
		{"backup logs to 'noop://'", true, "BACKUP LOGS TO 'noop://'"},
		{"backup logs to 'noop://' start_ts='20220304'", true, "BACKUP LOGS TO 'noop://' START_TS = '20220304'"},
		{"pause backup logs", true, "PAUSE BACKUP LOGS"},
		{"pause backup logs gc_ttl='20220304'", true, "PAUSE BACKUP LOGS GC_TTL = '20220304'"},
		{"resume backup logs", true, "RESUME BACKUP LOGS"},
		{"show backup logs status", true, "SHOW BACKUP LOGS STATUS"},
		{"show backup logs metadata from 'noop://'", true, "SHOW BACKUP LOGS METADATA FROM 'noop://'"},
		{"show br job 1234", true, "SHOW BR JOB 1234"},
		{"show br job query 1234", true, "SHOW BR JOB QUERY 1234"},
		{"cancel br job 1234", true, "CANCEL BR JOB 1234"},
		{"purge backup logs from 'noop://'", true, "PURGE BACKUP LOGS FROM 'noop://'"},
		{"purge backup logs from 'noop://' until_ts='2012122304'", true, "PURGE BACKUP LOGS FROM 'noop://' UNTIL_TS = '2012122304'"},
		{"restore point from 'noop://log_backup'", true, "RESTORE POINT FROM 'noop://log_backup'"},
		{"restore point from 'noop://log_backup' full_backup_storage='noop://full_log'", true, "RESTORE POINT FROM 'noop://log_backup' FULL_BACKUP_STORAGE = 'noop://full_log'"},
		{"restore point from 'noop://log_backup' full_backup_storage='noop://full_log' restored_ts='20230123'", true, "RESTORE POINT FROM 'noop://log_backup' FULL_BACKUP_STORAGE = 'noop://full_log' RESTORED_TS = '20230123'"},
		{"restore point from 'noop://log_backup' full_backup_storage='noop://full_log' start_ts='20230101' restored_ts='20230123'", true, "RESTORE POINT FROM 'noop://log_backup' FULL_BACKUP_STORAGE = 'noop://full_log' START_TS = '20230101' RESTORED_TS = '20230123'"},
	}

	RunTest(t, table, false)
}

func TestStatisticsOps(t *testing.T) {
	table := []testCase{
		{"create statistics stats1 (cardinality) on t(a,b,c)", true, "CREATE STATISTICS `stats1` (CARDINALITY) ON `t`(`a`, `b`, `c`)"},
		{"create statistics stats2 (dependency) on t(a,b)", true, "CREATE STATISTICS `stats2` (DEPENDENCY) ON `t`(`a`, `b`)"},
		{"create statistics stats3 (correlation) on t(a,b)", true, "CREATE STATISTICS `stats3` (CORRELATION) ON `t`(`a`, `b`)"},
		{"create statistics stats3 on t(a,b)", false, ""},
		{"create statistics if not exists stats1 (cardinality) on t(a,b,c)", true, "CREATE STATISTICS IF NOT EXISTS `stats1` (CARDINALITY) ON `t`(`a`, `b`, `c`)"},
		{"create statistics if not exists stats2 (dependency) on t(a,b)", true, "CREATE STATISTICS IF NOT EXISTS `stats2` (DEPENDENCY) ON `t`(`a`, `b`)"},
		{"create statistics if not exists stats3 (correlation) on t(a,b)", true, "CREATE STATISTICS IF NOT EXISTS `stats3` (CORRELATION) ON `t`(`a`, `b`)"},
		{"create statistics if not exists stats3 on t(a,b)", false, ""},
		{"create statistics stats1(cardinality) on t(a,b,c)", true, "CREATE STATISTICS `stats1` (CARDINALITY) ON `t`(`a`, `b`, `c`)"},
		{"drop statistics stats1", true, "DROP STATISTICS `stats1`"},
	}
	RunTest(t, table, false)

	p := parser.New()
	sms, _, err := p.Parse("create statistics if not exists stats1 (cardinality) on t(a,b,c)", "", "")
	require.NoError(t, err)
	v, ok := sms[0].(*ast.CreateStatisticsStmt)
	require.True(t, ok)
	require.True(t, v.IfNotExists)
	require.Equal(t, "stats1", v.StatsName)
	require.Equal(t, ast.StatsTypeCardinality, v.StatsType)
	require.Equal(t, ast.CIStr{O: "t", L: "t"}, v.Table.Name)
	require.Len(t, v.Columns, 3)
	require.Equal(t, ast.CIStr{O: "a", L: "a"}, v.Columns[0].Name)
	require.Equal(t, ast.CIStr{O: "b", L: "b"}, v.Columns[1].Name)
	require.Equal(t, ast.CIStr{O: "c", L: "c"}, v.Columns[2].Name)
}

func TestHighNotPrecedenceMode(t *testing.T) {
	p := parser.New()
	var sb strings.Builder

	sms, _, err := p.Parse("SELECT NOT 1 BETWEEN -5 AND 5", "", "")
	require.NoError(t, err)
	v, ok := sms[0].(*ast.SelectStmt)
	require.True(t, ok)
	v1, ok := v.Fields.Fields[0].Expr.(*ast.UnaryOperationExpr)
	require.True(t, ok)
	require.Equal(t, opcode.Not, v1.Op)
	err = sms[0].Restore(NewRestoreCtx(DefaultRestoreFlags, &sb))
	require.NoError(t, err)
	restoreSQL := sb.String()
	require.Equal(t, "SELECT NOT 1 BETWEEN -5 AND 5", restoreSQL)
	sb.Reset()

	sms, _, err = p.Parse("SELECT !1 BETWEEN -5 AND 5", "", "")
	require.NoError(t, err)
	v, ok = sms[0].(*ast.SelectStmt)
	require.True(t, ok)
	_, ok = v.Fields.Fields[0].Expr.(*ast.BetweenExpr)
	require.True(t, ok)
	err = sms[0].Restore(NewRestoreCtx(DefaultRestoreFlags, &sb))
	require.NoError(t, err)
	restoreSQL = sb.String()
	require.Equal(t, "SELECT !1 BETWEEN -5 AND 5", restoreSQL)
	sb.Reset()

	p = parser.New()
	p.SetSQLMode(mysql.ModeHighNotPrecedence)
	sms, _, err = p.Parse("SELECT NOT 1 BETWEEN -5 AND 5", "", "")
	require.NoError(t, err)
	v, ok = sms[0].(*ast.SelectStmt)
	require.True(t, ok)
	_, ok = v.Fields.Fields[0].Expr.(*ast.BetweenExpr)
	require.True(t, ok)
	err = sms[0].Restore(NewRestoreCtx(DefaultRestoreFlags, &sb))
	require.NoError(t, err)
	restoreSQL = sb.String()
	require.Equal(t, "SELECT !1 BETWEEN -5 AND 5", restoreSQL)
}

// For CTE
func TestCTE(t *testing.T) {
	table := []testCase{
		{"WITH `cte` AS (SELECT 1,2) SELECT `col1`,`col2` FROM `cte`", true, "WITH `cte` AS (SELECT 1,2) SELECT `col1`,`col2` FROM `cte`"},
		{"WITH `cte` (col1, col2) AS (SELECT 1,2 UNION ALL SELECT 3,4) SELECT col1, col2 FROM cte;", true, "WITH `cte` (`col1`, `col2`) AS (SELECT 1,2 UNION ALL SELECT 3,4) SELECT `col1`,`col2` FROM `cte`"},
		{"WITH `cte` AS (SELECT 1,2), cte2 as (select 3) SELECT `col1`,`col2` FROM `cte`", true, "WITH `cte` AS (SELECT 1,2), `cte2` AS (SELECT 3) SELECT `col1`,`col2` FROM `cte`"},
		{"WITH RECURSIVE cte (n) AS (  SELECT 1  UNION ALL  SELECT n + 1 FROM cte WHERE n < 5)SELECT * FROM cte;", true, "WITH RECURSIVE `cte` (`n`) AS (SELECT 1 UNION ALL SELECT `n`+1 FROM `cte` WHERE `n`<5) SELECT * FROM `cte`"},
		{"with cte(a) as (select 1) update t, cte set t.a=1  where t.a=cte.a;", true, "WITH `cte` (`a`) AS (SELECT 1) UPDATE (`t`) JOIN `cte` SET `t`.`a`=1 WHERE `t`.`a`=`cte`.`a`"},
		{"with cte(a) as (select 1) delete t from t, cte where t.a=cte.a;", true, "WITH `cte` (`a`) AS (SELECT 1) DELETE `t` FROM (`t`) JOIN `cte` WHERE `t`.`a`=`cte`.`a`"},
		{"WITH cte1 AS (SELECT 1) SELECT * FROM (WITH cte2 AS (SELECT 2) SELECT * FROM cte2 JOIN cte1) AS dt;", true, "WITH `cte1` AS (SELECT 1) SELECT * FROM (WITH `cte2` AS (SELECT 2) SELECT * FROM `cte2` JOIN `cte1`) AS `dt`"},
		{"WITH cte AS (SELECT 1) SELECT /*+ MAX_EXECUTION_TIME(1000) */ * FROM cte;", true, "WITH `cte` AS (SELECT 1) SELECT /*+ MAX_EXECUTION_TIME(1000)*/ * FROM `cte`"},
		{"with cte as (table t) table cte;", true, "WITH `cte` AS (TABLE `t`) TABLE `cte`"},
		{"with cte as (select 1) select 1 union with cte as (select 1) select * from cte;", false, ""},
		{"with cte as (select 1) (select 1);", true, "WITH `cte` AS (SELECT 1) (SELECT 1)"},
		{"with cte as (select 1) (select 1 union select 1)", true, "WITH `cte` AS (SELECT 1) (SELECT 1 UNION SELECT 1)"},
		{"select * from (with cte as (select 1) select 1 union select 2) qn", true, "SELECT * FROM (WITH `cte` AS (SELECT 1) SELECT 1 UNION SELECT 2) AS `qn`"},
		{"select * from t where 1 > (with cte as (select 2) select * from cte)", true, "SELECT * FROM `t` WHERE 1>(WITH `cte` AS (SELECT 2) SELECT * FROM `cte`)"},
		{"( with cte(n) as ( select 1 )  select n+1 from cte  union select n+2 from cte) union select 1", true, "(WITH `cte` (`n`) AS (SELECT 1) SELECT `n`+1 FROM `cte` UNION SELECT `n`+2 FROM `cte`) UNION SELECT 1"},
		{"( with cte(n) as ( select 1 )  select n+1 from cte) union select 1", true, "(WITH `cte` (`n`) AS (SELECT 1) SELECT `n`+1 FROM `cte`) UNION SELECT 1"},
		{"( with cte(n) as ( select 1 )  (select n+1 from cte)) union select 1", true, "(WITH `cte` (`n`) AS (SELECT 1) (SELECT `n`+1 FROM `cte`)) UNION SELECT 1"},
	}

	RunTest(t, table, false)
}

// For CTE Merge
func TestCTEMerge(t *testing.T) {
	table := []testCase{
		{"WITH `cte` AS (SELECT 1,2) SELECT `col1`,`col2` FROM `cte`", true, "WITH `cte` AS (SELECT 1,2) SELECT `col1`,`col2` FROM `cte`"},
		{"WITH `cte` (col1, col2) AS (SELECT 1,2 UNION ALL SELECT 3,4) SELECT col1, col2 FROM cte;", true, "WITH `cte` (`col1`, `col2`) AS (SELECT 1,2 UNION ALL SELECT 3,4) SELECT `col1`,`col2` FROM `cte`"},
		{"WITH `cte` AS (SELECT 1,2), cte2 as (select 3) SELECT `col1`,`col2` FROM `cte`", true, "WITH `cte` AS (SELECT 1,2), `cte2` AS (SELECT 3) SELECT `col1`,`col2` FROM `cte`"},
		{"with cte(a) as (select 1) update t, cte set t.a=1  where t.a=cte.a;", true, "WITH `cte` (`a`) AS (SELECT 1) UPDATE (`t`) JOIN `cte` SET `t`.`a`=1 WHERE `t`.`a`=`cte`.`a`"},
		{"with cte(a) as (select 1) delete t from t, cte where t.a=cte.a;", true, "WITH `cte` (`a`) AS (SELECT 1) DELETE `t` FROM (`t`) JOIN `cte` WHERE `t`.`a`=`cte`.`a`"},
		{"WITH cte1 AS (SELECT 1) SELECT * FROM (WITH cte2 AS (SELECT 2) SELECT * FROM cte2 JOIN cte1) AS dt;", true, "WITH `cte1` AS (SELECT 1) SELECT * FROM (WITH `cte2` AS (SELECT 2) SELECT * FROM `cte2` JOIN `cte1`) AS `dt`"},
		{"WITH cte AS (SELECT 1) SELECT /*+ MAX_EXECUTION_TIME(1000) */ * FROM cte;", true, "WITH `cte` AS (SELECT 1) SELECT /*+ MAX_EXECUTION_TIME(1000)*/ * FROM `cte`"},
		{"with cte as (table t) table cte;", true, "WITH `cte` AS (TABLE `t`) TABLE `cte`"},
		{"with cte as (select 1) select 1 union with cte as (select 1) select * from cte;", false, ""},
		{"with cte as (select 1) (select 1);", true, "WITH `cte` AS (SELECT 1) (SELECT 1)"},
		{"with cte as (select 1) (select 1 union select 1)", true, "WITH `cte` AS (SELECT 1) (SELECT 1 UNION SELECT 1)"},
		{"select * from (with cte as (select 1) select 1 union select 2) qn", true, "SELECT * FROM (WITH `cte` AS (SELECT 1) SELECT 1 UNION SELECT 2) AS `qn`"},
		{"select * from t where 1 > (with cte as (select 2) select * from cte)", true, "SELECT * FROM `t` WHERE 1>(WITH `cte` AS (SELECT 2) SELECT * FROM `cte`)"},
		{"( with cte(n) as ( select 1 )  select n+1 from cte  union select n+2 from cte) union select 1", true, "(WITH `cte` (`n`) AS (SELECT 1) SELECT `n`+1 FROM `cte` UNION SELECT `n`+2 FROM `cte`) UNION SELECT 1"},
		{"( with cte(n) as ( select 1 )  select n+1 from cte) union select 1", true, "(WITH `cte` (`n`) AS (SELECT 1) SELECT `n`+1 FROM `cte`) UNION SELECT 1"},
		{"( with cte(n) as ( select 1 )  (select n+1 from cte)) union select 1", true, "(WITH `cte` (`n`) AS (SELECT 1) (SELECT `n`+1 FROM `cte`)) UNION SELECT 1"},
	}

	RunTest(t, table, false)
}

func TestAsOfClause(t *testing.T) {
	table := []testCase{
		{"SELECT * FROM `t` AS /* comment */ a;", true, "SELECT * FROM `t` AS `a`"},
		{"SELECT * FROM `t` AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW());", true, "SELECT * FROM `t` AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())"},
		{"select * from `t` as of timestamp '2021-04-15 00:00:00'", true, "SELECT * FROM `t` AS OF TIMESTAMP _UTF8MB4'2021-04-15 00:00:00'"},
		{"SELECT * FROM (`a` AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())) JOIN `b` AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW());", true, "SELECT * FROM (`a` AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())) JOIN `b` AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())"},
		{"INSERT INTO `employees` (SELECT * FROM `employees` AS OF TIMESTAMP (DATE_SUB(NOW(), INTERVAL _UTF8MB4'60' MINUTE)) NOT IN (SELECT * FROM `employees`))", true, "INSERT INTO `employees` (SELECT * FROM `employees` AS OF TIMESTAMP (DATE_SUB(NOW(), INTERVAL _UTF8MB4'60' MINUTE)) NOT IN (SELECT * FROM `employees`))"},
		{"SET TRANSACTION READ ONLY as of timestamp '2021-04-21 00:42:12'", true, "SET @@SESSION.`tx_read_ts`=_UTF8MB4'2021-04-21 00:42:12'"},
		{"START TRANSACTION READ ONLY AS OF TIMESTAMP '2015-09-21 00:07:01'", true, "START TRANSACTION READ ONLY AS OF TIMESTAMP _UTF8MB4'2015-09-21 00:07:01'"},
		{"START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(_UTF8MB4'2015-09-21 00:07:01', NOW())", true, "START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(_UTF8MB4'2015-09-21 00:07:01', NOW())"},
		{"START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())", true, "START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(DATE_SUB(NOW(), INTERVAL 3 SECOND), NOW())"},
		{"START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(_UTF8MB4'2015-09-21 00:07:01', '2021-04-27 11:26:13')", true, "START TRANSACTION READ ONLY AS OF TIMESTAMP TIDB_BOUNDED_STALENESS(_UTF8MB4'2015-09-21 00:07:01', _UTF8MB4'2021-04-27 11:26:13')"},
	}
	RunTest(t, table, false)
}

// For `PARTITION BY [LINEAR] KEY ALGORITHM` syntax
func TestPartitionKeyAlgorithm(t *testing.T) {
	table := []testCase{
		{"CREATE TABLE t  (c1 integer ,c2 integer) PARTITION BY LINEAR KEY ALGORITHM = 1 (c1,c2) PARTITIONS 4", true, "CREATE TABLE `t` (`c1` INT,`c2` INT) PARTITION BY LINEAR KEY ALGORITHM = 1 (`c1`,`c2`) PARTITIONS 4"},
		{"CREATE TABLE t  (c1 integer ,c2 integer) PARTITION BY LINEAR KEY ALGORITHM = -1 (c1,c2) PARTITIONS 4", false, ""},
		{"CREATE TABLE t  (c1 integer ,c2 integer) PARTITION BY LINEAR KEY ALGORITHM = 0 (c1,c2) PARTITIONS 4", false, ""},
		{"CREATE TABLE t  (c1 integer ,c2 integer) PARTITION BY LINEAR KEY ALGORITHM = 3 (c1,c2) PARTITIONS 4", false, ""},
	}

	RunTest(t, table, false)
}

// server side help syntax
func TestHelp(t *testing.T) {
	table := []testCase{
		{"HELP 'select'", true, "HELP 'select'"},
	}

	RunTest(t, table, false)
}

func TestWithoutCharsetFlags(t *testing.T) {
	type testCaseWithFlag struct {
		src     string
		ok      bool
		restore string
		flag    RestoreFlags
	}

	flag := RestoreStringSingleQuotes | RestoreSpacesAroundBinaryOperation | RestoreBracketAroundBinaryOperation | RestoreNameBackQuotes
	cases := []testCaseWithFlag{
		{"select 'a'", true, "SELECT 'a'", flag | RestoreStringWithoutCharset},
		{"select _utf8'a'", true, "SELECT 'a'", flag | RestoreStringWithoutCharset},
		{"select _utf8mb4'a'", true, "SELECT 'a'", flag | RestoreStringWithoutCharset},
		{"select _utf8 X'D0B1'", true, "SELECT x'd0b1'", flag | RestoreStringWithoutCharset},

		{"select _utf8mb4'a'", true, "SELECT 'a'", flag | RestoreStringWithoutDefaultCharset},
		{"select _utf8'a'", true, "SELECT _utf8'a'", flag | RestoreStringWithoutDefaultCharset},
		{"select _utf8'a'", true, "SELECT _utf8'a'", flag | RestoreStringWithoutDefaultCharset},
		{"select _utf8 X'D0B1'", true, "SELECT _utf8 x'd0b1'", flag | RestoreStringWithoutDefaultCharset},
	}

	p := parser.New()
	p.EnableWindowFunc(false)
	for _, tbl := range cases {
		stmts, _, err := p.Parse(tbl.src, "", "")
		if !tbl.ok {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		// restore correctness test
		var sb strings.Builder
		restoreSQLs := ""
		for _, stmt := range stmts {
			sb.Reset()
			ctx := NewRestoreCtx(tbl.flag, &sb)
			ctx.DefaultDB = "test"
			err = stmt.Restore(ctx)
			require.NoError(t, err)
			restoreSQL := sb.String()
			if restoreSQLs != "" {
				restoreSQLs += "; "
			}
			restoreSQLs += restoreSQL
		}
		require.Equal(t, tbl.restore, restoreSQLs)
	}
}

func TestRestoreBinOpWithBrackets(t *testing.T) {
	cases := []testCase{
		{"select mod(a+b, 4)+1", true, "SELECT (((`a` + `b`) % 4) + 1)"},
		{"SELECT MOD(10, 2 BETWEEN 0 and 5)", true, "SELECT (10 % (2 BETWEEN 0 AND 5))"}, // issue #59000
		{"select mod( year(a) - abs(weekday(a) + dayofweek(a)), 4) + 1", true, "SELECT (((year(`a`) - abs((weekday(`a`) + dayofweek(`a`)))) % 4) + 1)"},
	}

	p := parser.New()
	p.EnableWindowFunc(false)
	for _, tbl := range cases {
		_, _, err := p.Parse(tbl.src, "", "")
		comment := fmt.Sprintf("source %v", tbl.src)
		if !tbl.ok {
			require.Error(t, err, comment)
			continue
		}
		require.NoError(t, err, comment)
		// restore correctness test
		if tbl.ok {
			var sb strings.Builder
			comment := fmt.Sprintf("source %v", tbl.src)
			stmts, _, err := p.Parse(tbl.src, "", "")
			require.NoError(t, err, comment)
			restoreSQLs := ""
			for _, stmt := range stmts {
				sb.Reset()
				ctx := NewRestoreCtx(RestoreStringSingleQuotes|RestoreSpacesAroundBinaryOperation|RestoreBracketAroundBinaryOperation|RestoreStringWithoutCharset|RestoreNameBackQuotes, &sb)
				ctx.DefaultDB = "test"
				err = stmt.Restore(ctx)
				require.NoError(t, err, comment)
				restoreSQL := sb.String()
				comment = fmt.Sprintf("source %v; restore %v", tbl.src, restoreSQL)
				if restoreSQLs != "" {
					restoreSQLs += "; "
				}
				restoreSQLs += restoreSQL
			}
			comment = fmt.Sprintf("restore %v; expect %v", restoreSQLs, tbl.restore)
			require.Equal(t, tbl.restore, restoreSQLs, comment)
		}
	}
}

// For CTE bindings.
func TestCTEBindings(t *testing.T) {
	table := []testCase{
		{"WITH `cte` AS (SELECT * from t) SELECT `col1`,`col2` FROM `cte`", true, "WITH `cte` AS (SELECT * FROM `test`.`t`) SELECT `col1`,`col2` FROM `cte`"},
		{"WITH `cte` (col1, col2) AS (SELECT * from t UNION ALL SELECT 3,4) SELECT col1, col2 FROM cte;", true, "WITH `cte` (`col1`, `col2`) AS (SELECT * FROM `test`.`t` UNION ALL SELECT 3,4) SELECT `col1`,`col2` FROM `cte`"},
		{"WITH `cte` AS (SELECT * from t), cte2 as (select * from cte) SELECT `col1`,`col2` FROM `cte`", true, "WITH `cte` AS (SELECT * FROM `test`.`t`), `cte2` AS (SELECT * FROM `cte`) SELECT `col1`,`col2` FROM `cte`"},
		{"WITH RECURSIVE cte (n) AS (  SELECT * from t  UNION ALL  SELECT n + 1 FROM cte WHERE n < 5)SELECT * FROM cte;", true, "WITH RECURSIVE `cte` (`n`) AS (SELECT * FROM `test`.`t` UNION ALL SELECT `n` + 1 FROM `cte` WHERE `n` < 5) SELECT * FROM `cte`"},
		{"with cte(a) as (select * from t) update t, cte set t.a=1  where t.a=cte.a;", true, "WITH `cte` (`a`) AS (SELECT * FROM `test`.`t`) UPDATE (`test`.`t`) JOIN `cte` SET `t`.`a`=1 WHERE `t`.`a` = `cte`.`a`"},
		{"with cte(a) as (select * from t) delete t from t, cte where t.a=cte.a;", true, "WITH `cte` (`a`) AS (SELECT * FROM `test`.`t`) DELETE `test`.`t` FROM (`test`.`t`) JOIN `cte` WHERE `t`.`a` = `cte`.`a`"},
		{"WITH cte1 AS (SELECT * from t) SELECT * FROM (WITH cte2 AS (SELECT * from cte1) SELECT * FROM cte2 JOIN cte1) AS dt;", true, "WITH `cte1` AS (SELECT * FROM `test`.`t`) SELECT * FROM (WITH `cte2` AS (SELECT * FROM `cte1`) SELECT * FROM `cte2` JOIN `cte1`) AS `dt`"},
		{"WITH cte AS (SELECT * from t) SELECT /*+ MAX_EXECUTION_TIME(1000) */ * FROM cte;", true, "WITH `cte` AS (SELECT * FROM `test`.`t`) SELECT /*+ MAX_EXECUTION_TIME(1000)*/ * FROM `cte`"},
		{"with cte as (table t) table cte;", true, "WITH `cte` AS (TABLE `test`.`t`) TABLE `cte`"},
		{"with cte as (select * from t) select 1 union with cte as (select * from t) select * from cte;", false, ""},
		{"with cte as (select * from t) (select * from t);", true, "WITH `cte` AS (SELECT * FROM `test`.`t`) (SELECT * FROM `test`.`t`)"},
		{"with cte as (select 1) (select 1 union select * from t)", true, "WITH `cte` AS (SELECT 1) (SELECT 1 UNION SELECT * FROM `test`.`t`)"},
		{"select * from (with cte as (select * from t) select 1 union select * from t) qn", true, "SELECT * FROM (WITH `cte` AS (SELECT * FROM `test`.`t`) SELECT 1 UNION SELECT * FROM `test`.`t`) AS `qn`"},
		{"select * from t where 1 > (with cte as (select * from t) select * from cte)", true, "SELECT * FROM `test`.`t` WHERE 1 > (WITH `cte` AS (SELECT * FROM `test`.`t`) SELECT * FROM `cte`)"},
		{"( with cte(n) as ( select * from t )  select n+1 from cte  union select n+2 from cte) union select 1", true, "(WITH `cte` (`n`) AS (SELECT * FROM `test`.`t`) SELECT `n` + 1 FROM `cte` UNION SELECT `n` + 2 FROM `cte`) UNION SELECT 1"},
		{"( with cte(n) as ( select * from t )  select n+1 from cte) union select * from t", true, "(WITH `cte` (`n`) AS (SELECT * FROM `test`.`t`) SELECT `n` + 1 FROM `cte`) UNION SELECT * FROM `test`.`t`"},
		{"with cte as (select * from t union select * from cte) select * from cte", true, "WITH `cte` AS (SELECT * FROM `test`.`t` UNION SELECT * FROM `test`.`cte`) SELECT * FROM `cte`"},
	}

	p := parser.New()
	p.EnableWindowFunc(false)
	for _, tbl := range table {
		_, _, err := p.Parse(tbl.src, "", "")
		comment := fmt.Sprintf("source %v", tbl.src)
		if !tbl.ok {
			require.Error(t, err, comment)
			continue
		}
		require.NoError(t, err, comment)
		// restore correctness test
		if tbl.ok {
			var sb strings.Builder
			comment := fmt.Sprintf("source %v", tbl.src)
			stmts, _, err := p.Parse(tbl.src, "", "")
			require.NoError(t, err, comment)
			restoreSQLs := ""
			for _, stmt := range stmts {
				sb.Reset()
				ctx := NewRestoreCtx(RestoreStringSingleQuotes|RestoreSpacesAroundBinaryOperation|RestoreStringWithoutCharset|RestoreNameBackQuotes, &sb)
				ctx.DefaultDB = "test"
				err = stmt.Restore(ctx)
				require.NoError(t, err, comment)
				restoreSQL := sb.String()
				comment = fmt.Sprintf("source %v; restore %v", tbl.src, restoreSQL)
				if restoreSQLs != "" {
					restoreSQLs += "; "
				}
				restoreSQLs += restoreSQL
			}
			comment = fmt.Sprintf("restore %v; expect %v", restoreSQLs, tbl.restore)
			require.Equal(t, tbl.restore, restoreSQLs, comment)
		}
	}
}

func TestPlanReplayer(t *testing.T) {
	table := []testCase{
		{"PLAN REPLAYER DUMP EXPLAIN SELECT a FROM t", true, "PLAN REPLAYER DUMP EXPLAIN SELECT `a` FROM `t`"},
		{"PLAN REPLAYER DUMP EXPLAIN SELECT * FROM t WHERE a > 10", true, "PLAN REPLAYER DUMP EXPLAIN SELECT * FROM `t` WHERE `a`>10"},
		{"PLAN REPLAYER DUMP EXPLAIN ANALYZE SELECT * FROM t WHERE a > 10", true, "PLAN REPLAYER DUMP EXPLAIN ANALYZE SELECT * FROM `t` WHERE `a`>10"},
		{"PLAN REPLAYER DUMP EXPLAIN SLOW QUERY WHERE a > 10 and t < 1 ORDER BY t LIMIT 10", true, "PLAN REPLAYER DUMP EXPLAIN SLOW QUERY WHERE `a`>10 AND `t`<1 ORDER BY `t` LIMIT 10"},
		{"PLAN REPLAYER DUMP EXPLAIN ANALYZE SLOW QUERY WHERE a > 10 and t < 1 ORDER BY t LIMIT 10", true, "PLAN REPLAYER DUMP EXPLAIN ANALYZE SLOW QUERY WHERE `a`>10 AND `t`<1 ORDER BY `t` LIMIT 10"},
		{"PLAN REPLAYER DUMP EXPLAIN SLOW QUERY WHERE a > 10 and t < 1 LIMIT 10", true, "PLAN REPLAYER DUMP EXPLAIN SLOW QUERY WHERE `a`>10 AND `t`<1 LIMIT 10"},
		{"PLAN REPLAYER DUMP EXPLAIN ANALYZE SLOW QUERY WHERE a > 10 and t < 1 LIMIT 10", true, "PLAN REPLAYER DUMP EXPLAIN ANALYZE SLOW QUERY WHERE `a`>10 AND `t`<1 LIMIT 10"},
		{"PLAN REPLAYER DUMP EXPLAIN SLOW QUERY LIMIT 10", true, "PLAN REPLAYER DUMP EXPLAIN SLOW QUERY LIMIT 10"},
		{"PLAN REPLAYER DUMP EXPLAIN ANALYZE SLOW QUERY LIMIT 10", true, "PLAN REPLAYER DUMP EXPLAIN ANALYZE SLOW QUERY LIMIT 10"},
		{"PLAN REPLAYER DUMP EXPLAIN SLOW QUERY", true, "PLAN REPLAYER DUMP EXPLAIN SLOW QUERY"},
		{"PLAN REPLAYER DUMP EXPLAIN ANALYZE SLOW QUERY", true, "PLAN REPLAYER DUMP EXPLAIN ANALYZE SLOW QUERY"},
		{"PLAN REPLAYER LOAD '/tmp/sdfaalskdjf.zip'", true, "PLAN REPLAYER LOAD '/tmp/sdfaalskdjf.zip'"},
		{"PLAN REPLAYER DUMP EXPLAIN 'sql.txt'", true, "PLAN REPLAYER DUMP EXPLAIN 'sql.txt'"},
		{"PLAN REPLAYER DUMP EXPLAIN ANALYZE 'sql.txt'", true, "PLAN REPLAYER DUMP EXPLAIN ANALYZE 'sql.txt'"},
		{"PLAN REPLAYER CAPTURE '123' '123'", true, "PLAN REPLAYER CAPTURE '123' '123'"},
		{"PLAN REPLAYER CAPTURE REMOVE '123' '123'", true, "PLAN REPLAYER CAPTURE REMOVE '123' '123'"},
	}
	RunTest(t, table, false)

	p := parser.New()
	sms, _, err := p.Parse("PLAN REPLAYER DUMP EXPLAIN SELECT a FROM t", "", "")
	require.NoError(t, err)
	v, ok := sms[0].(*ast.PlanReplayerStmt)
	require.True(t, ok)
	require.Equal(t, "SELECT a FROM t", v.Stmt.Text())
	require.False(t, v.Analyze)

	sms, _, err = p.Parse("PLAN REPLAYER DUMP EXPLAIN ANALYZE SELECT a FROM t", "", "")
	require.NoError(t, err)
	v, ok = sms[0].(*ast.PlanReplayerStmt)
	require.True(t, ok)
	require.Equal(t, "SELECT a FROM t", v.Stmt.Text())
	require.True(t, v.Analyze)
}

func TestTrafficStmt(t *testing.T) {
	table := []testCase{
		{"traffic capture to '/tmp' duration='1s' encryption_method='aes' compress=true", true, "TRAFFIC CAPTURE TO '/tmp' DURATION = '1s' ENCRYPTION_METHOD = 'aes' COMPRESS = TRUE"},
		{"traffic capture to '/tmp' duration '1s' encryption_method 'aes' compress true", true, "TRAFFIC CAPTURE TO '/tmp' DURATION = '1s' ENCRYPTION_METHOD = 'aes' COMPRESS = TRUE"},
		{"traffic capture to '/tmp' encryption_method='aes' duration='1s'", true, "TRAFFIC CAPTURE TO '/tmp' ENCRYPTION_METHOD = 'aes' DURATION = '1s'"},
		{"traffic capture to '/tmp' duration='1m'", true, "TRAFFIC CAPTURE TO '/tmp' DURATION = '1m'"},
		{"traffic capture to '/tmp' duration='1'", false, ""},
		{"traffic capture to '/tmp' duration=1s", false, ""},
		{"traffic capture to '/tmp' compress='true'", false, ""},
		{"traffic capture duration='1m'", false, ""},
		{"traffic capture", false, ""},
		{"traffic replay from '/tmp' user='root' password='123456' speed=1.0 read_only=true", true, "TRAFFIC REPLAY FROM '/tmp' USER = 'root' PASSWORD = '123456' SPEED = 1.0 READONLY = TRUE"},
		{"traffic replay from '/tmp' user 'root' password '123456' speed 1.0 read_only true", true, "TRAFFIC REPLAY FROM '/tmp' USER = 'root' PASSWORD = '123456' SPEED = 1.0 READONLY = TRUE"},
		{"traffic replay from '/tmp' speed 1.0 user='root'", true, "TRAFFIC REPLAY FROM '/tmp' SPEED = 1.0 USER = 'root'"},
		{"traffic replay from '/tmp' speed=1", true, "TRAFFIC REPLAY FROM '/tmp' SPEED = 1"},
		{"traffic replay from '/tmp' speed=0.5", true, "TRAFFIC REPLAY FROM '/tmp' SPEED = 0.5"},
		{"traffic replay from '/tmp' speed=-1", false, ""},
		{"traffic replay speed=1", false, ""},
		{"traffic replay", false, ""},
		{"show traffic jobs", true, "SHOW TRAFFIC JOBS"},
		{"show traffic jobs duration='1m'", false, ""},
		{"show traffic", false, ""},
		{"cancel traffic jobs", true, "CANCEL TRAFFIC JOBS"},
		{"cancel traffic jobs duration='1m'", false, ""},
		{"cancel traffic", false, ""},
		{"traffic test", false, ""},
		{"traffic", false, ""},
	}

	p := parser.New()
	var sb strings.Builder
	for _, tbl := range table {
		stmts, _, err := p.Parse(tbl.src, "", "")
		if !tbl.ok {
			require.Error(t, err, tbl.src)
			continue
		}
		require.NoError(t, err, tbl.src)
		require.Len(t, stmts, 1)
		v, ok := stmts[0].(*ast.TrafficStmt)
		require.True(t, ok)
		switch v.OpType {
		case ast.TrafficOpCapture, ast.TrafficOpReplay:
			require.Equal(t, "/tmp", v.Dir)
		}
		sb.Reset()
		ctx := NewRestoreCtx(RestoreStringSingleQuotes|RestoreSpacesAroundBinaryOperation|RestoreStringWithoutCharset|RestoreNameBackQuotes, &sb)
		err = v.Restore(ctx)
		require.NoError(t, err)
		require.Equal(t, tbl.restore, sb.String())
	}
}

func TestGBKEncoding(t *testing.T) {
	p := parser.New()
	gbkEncoding, _ := charset.Lookup("gbk")
	encoder := gbkEncoding.NewEncoder()
	sql, err := encoder.String("create table 测试表 (测试列 varchar(255) default 'GBK测试用例');")
	require.NoError(t, err)

	stmt, _, err := p.ParseSQL(sql)
	require.NoError(t, err)
	checker := &gbkEncodingChecker{}
	_, _ = stmt[0].Accept(checker)
	require.NotEqual(t, "测试表", checker.tblName)
	require.NotEqual(t, "测试列", checker.colName)

	gbkOpt := parser.CharsetClient("gbk")
	stmt, _, err = p.ParseSQL(sql, gbkOpt)
	require.NoError(t, err)
	_, _ = stmt[0].Accept(checker)
	require.Equal(t, "测试表", checker.tblName)
	require.Equal(t, "测试列", checker.colName)
	require.Equal(t, "GBK测试用例", checker.expr)

	_, _, err = p.ParseSQL("select _gbk '\xc6\x5c' from dual;")
	require.Error(t, err)

	for _, test := range []struct {
		sql string
		err bool
	}{
		{"select '\xc6\x5c' from `\xab\x60`;", false},
		{`prepare p1 from "insert into t values ('中文');";`, false},
		{"select '啊';", false},
		{"create table t1(s set('a一','b二','c三'));", false},
		{"insert into t3 values('一a');", false},
		{"select '\xa5\x5c'", false},
		{"select '''\xa5\x5c'", false},
		{"select ```\xa5\x5c`", false},
		{"select '\x65\x5c'", true},
	} {
		_, _, err = p.ParseSQL(test.sql, gbkOpt)
		if test.err {
			require.Error(t, err, test.sql)
		} else {
			require.NoError(t, err, test.sql)
		}
	}
}

func TestGB18030Encoding(t *testing.T) {
	p := parser.New()
	gb18030Encoding, _ := charset.Lookup("gb18030")
	encoder := gb18030Encoding.NewEncoder()
	sql, err := encoder.String("create table 测试表 (测试列 varchar(255) default 'GB18030测试用例');")
	require.NoError(t, err)

	stmt, _, err := p.ParseSQL(sql)
	require.NoError(t, err)
	checker := &gbkEncodingChecker{}
	_, _ = stmt[0].Accept(checker)
	require.NotEqual(t, "测试表", checker.tblName)
	require.NotEqual(t, "测试列", checker.colName)

	gb18030Opt := parser.CharsetClient("gb18030")
	stmt, _, err = p.ParseSQL(sql, gb18030Opt)
	require.NoError(t, err)
	_, _ = stmt[0].Accept(checker)
	require.Equal(t, "测试表", checker.tblName)
	require.Equal(t, "测试列", checker.colName)
	require.Equal(t, "GB18030测试用例", checker.expr)

	_, _, err = p.ParseSQL("select _gbk '\xc6\x5c' from dual;")
	require.Error(t, err)

	for _, test := range []struct {
		sql string
		err bool
	}{
		{"select '\xc6\x5c' from `\xab\x60`;", false},
		{`prepare p1 from "insert into t values ('中文');";`, false},
		{"select '啊';", false},
		{"create table t1(s set('a一','b二','c三'));", false},
		{"insert into t3 values('一a');", false},
		{"select '\xa5\x5c'", false},
		{"select '''\xa5\x5c'", false},
		{"select ```\xa5\x5c`", false},
		{"select '\x65\x5c'", true},
	} {
		_, _, err = p.ParseSQL(test.sql, gb18030Opt)
		if test.err {
			require.Error(t, err, test.sql)
		} else {
			require.NoError(t, err, test.sql)
		}
	}
}

type gbkEncodingChecker struct {
	tblName string
	colName string
	expr    string
}

func (g *gbkEncodingChecker) Enter(n ast.Node) (node ast.Node, skipChildren bool) {
	if tn, ok := n.(*ast.TableName); ok {
		g.tblName = tn.Name.O
		return n, false
	}
	if cn, ok := n.(*ast.ColumnName); ok {
		g.colName = cn.Name.O
		return n, false
	}
	if c, ok := n.(*ast.ColumnOption); ok {
		if ve, ok := c.Expr.(ast.ValueExpr); ok {
			g.expr = ve.GetString()
			return n, false
		}
	}
	return n, false
}

func (g *gbkEncodingChecker) Leave(n ast.Node) (node ast.Node, ok bool) {
	return n, true
}

func TestInsertStatementMemoryAllocation(t *testing.T) {
	sql := "insert t values (1)" + strings.Repeat(",(1)", 1000)
	var oldStats, newStats runtime.MemStats
	runtime.ReadMemStats(&oldStats)
	_, err := parser.New().ParseOneStmt(sql, "", "")
	require.NoError(t, err)
	runtime.ReadMemStats(&newStats)
	require.Less(t, int(newStats.TotalAlloc-oldStats.TotalAlloc), 1024*500)
}

func TestCharsetIntroducer(t *testing.T) {
	p := parser.New()
	defer charset.RemoveCharset("gbk")
	// `_gbk` is treated as a character set.
	_, _, err := p.Parse("select _gbk 'a';", "", "")
	require.EqualError(t, err, "[ddl:1115]Unsupported character introducer: 'gbk'")
	_, _, err = p.Parse("select _gbk 0x1234;", "", "")
	require.EqualError(t, err, "[ddl:1115]Unsupported character introducer: 'gbk'")
	_, _, err = p.Parse("select _gbk 0b101001;", "", "")
	require.EqualError(t, err, "[ddl:1115]Unsupported character introducer: 'gbk'")
}

func TestNonTransactionalDML(t *testing.T) {
	cases := []testCase{
		// deletes
		{"batch on c limit 10 delete from t where c = 10", true,
			"BATCH ON `c` LIMIT 10 DELETE FROM `t` WHERE `c`=10"},
		{"batch on c limit 10 dry run delete from t where c = 10", true,
			"BATCH ON `c` LIMIT 10 DRY RUN DELETE FROM `t` WHERE `c`=10"},
		{"batch on c limit 10 dry run query delete from t where c = 10", true,
			"BATCH ON `c` LIMIT 10 DRY RUN QUERY DELETE FROM `t` WHERE `c`=10"},
		{"batch limit 10 delete from t where c = 10", true,
			"BATCH LIMIT 10 DELETE FROM `t` WHERE `c`=10"},
		{"batch limit 10 dry run delete from t where c = 10", true,
			"BATCH LIMIT 10 DRY RUN DELETE FROM `t` WHERE `c`=10"},
		{"batch limit 10 dry run query delete from t where c = 10", true,
			"BATCH LIMIT 10 DRY RUN QUERY DELETE FROM `t` WHERE `c`=10"},
		// updates
		{"batch on c limit 10 update t set c = 10", true,
			"BATCH ON `c` LIMIT 10 UPDATE `t` SET `c`=10"},
		{"batch on c limit 10 dry run update t set c = 10", true,
			"BATCH ON `c` LIMIT 10 DRY RUN UPDATE `t` SET `c`=10"},
		{"batch on c limit 10 dry run query update t set c = 10", true,
			"BATCH ON `c` LIMIT 10 DRY RUN QUERY UPDATE `t` SET `c`=10"},
		{"batch limit 10 update t set c = 10", true,
			"BATCH LIMIT 10 UPDATE `t` SET `c`=10"},
		{"batch limit 10 dry run update t set c = 10", true,
			"BATCH LIMIT 10 DRY RUN UPDATE `t` SET `c`=10"},
		{"batch limit 10 dry run query update t set c = 10", true,
			"BATCH LIMIT 10 DRY RUN QUERY UPDATE `t` SET `c`=10"},
		// inserts
		{"batch on c limit 10 insert into t1 select * from t2 where c = 10", true,
			"BATCH ON `c` LIMIT 10 INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10"},
		{"batch on c limit 10 dry run insert into t1 select * from t2 where c = 10", true,
			"BATCH ON `c` LIMIT 10 DRY RUN INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10"},
		{"batch on c limit 10 dry run query insert into t1 select * from t2 where c = 10", true,
			"BATCH ON `c` LIMIT 10 DRY RUN QUERY INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10"},
		{"batch limit 10 insert into t1 select * from t2 where c = 10", true,
			"BATCH LIMIT 10 INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10"},
		{"batch limit 10 dry run insert into t1 select * from t2 where c = 10", true,
			"BATCH LIMIT 10 DRY RUN INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10"},
		{"batch limit 10 dry run query insert into t1 select * from t2 where c = 10", true,
			"BATCH LIMIT 10 DRY RUN QUERY INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10"},
		// inserts on duplicate key update
		{"batch on c limit 10 insert into t1 select * from t2 where c = 10 on duplicate key update t1.val = t2.val", true,
			"BATCH ON `c` LIMIT 10 INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10 ON DUPLICATE KEY UPDATE `t1`.`val`=`t2`.`val`"},
		{"batch on c limit 10 dry run insert into t1 select * from t2 where c = 10 on duplicate key update t1.val = t2.val", true,
			"BATCH ON `c` LIMIT 10 DRY RUN INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10 ON DUPLICATE KEY UPDATE `t1`.`val`=`t2`.`val`"},
		{"batch on c limit 10 dry run query insert into t1 select * from t2 where c = 10 on duplicate key update t1.val = t2.val", true,
			"BATCH ON `c` LIMIT 10 DRY RUN QUERY INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10 ON DUPLICATE KEY UPDATE `t1`.`val`=`t2`.`val`"},
		{"batch limit 10 insert into t1 select * from t2 where c = 10 on duplicate key update t1.val = t2.val", true,
			"BATCH LIMIT 10 INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10 ON DUPLICATE KEY UPDATE `t1`.`val`=`t2`.`val`"},
		{"batch limit 10 dry run insert into t1 select * from t2 where c = 10 on duplicate key update t1.val = t2.val", true,
			"BATCH LIMIT 10 DRY RUN INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10 ON DUPLICATE KEY UPDATE `t1`.`val`=`t2`.`val`"},
		{"batch limit 10 dry run query insert into t1 select * from t2 where c = 10 on duplicate key update t1.val = t2.val", true,
			"BATCH LIMIT 10 DRY RUN QUERY INSERT INTO `t1` SELECT * FROM `t2` WHERE `c`=10 ON DUPLICATE KEY UPDATE `t1`.`val`=`t2`.`val`"},
	}

	RunTest(t, cases, false)
}

func TestIntervalPartition(t *testing.T) {
	table := []testCase{
		{"CREATE TABLE t (c1 integer,c2 integer) PARTITION BY RANGE (c1) INTERVAL (1000)", true, "CREATE TABLE `t` (`c1` INT,`c2` INT) PARTITION BY RANGE (`c1`) INTERVAL (1000)"},
		{"CREATE TABLE t (c1 int, c2 date) PARTITION BY RANGE (c2) INTERVAL (1 Month)", true, "CREATE TABLE `t` (`c1` INT,`c2` DATE) PARTITION BY RANGE (`c2`) INTERVAL (1 MONTH)"},
		{"CREATE TABLE t (c1 int, c2 date) PARTITION BY RANGE (c1) (partition p1 values less than (22))", true, "CREATE TABLE `t` (`c1` INT,`c2` DATE) PARTITION BY RANGE (`c1`) (PARTITION `p1` VALUES LESS THAN (22))"},
		{`CREATE TABLE t (c1 int, c2 date) PARTITION BY RANGE COLUMNS (c2) INTERVAL (1 year) first partition less than ("2022-02-01")`, false, ""},
		{`CREATE TABLE t (c1 int, c2 datetime) PARTITION BY RANGE COLUMNS (c2) INTERVAL (1 day) first partition less than ("2022-01-02") last partition less than ("2022-06-01") NULL PARTITION MAXVALUE PARTITION`, true, "CREATE TABLE `t` (`c1` INT,`c2` DATETIME) PARTITION BY RANGE COLUMNS (`c2`) INTERVAL (1 DAY) FIRST PARTITION LESS THAN (_UTF8MB4'2022-01-02') LAST PARTITION LESS THAN (_UTF8MB4'2022-06-01') NULL PARTITION MAXVALUE PARTITION"},
		{`ALTER TABLE t LAST PARTITION LESS THAN (1000)`, true, "ALTER TABLE `t` LAST PARTITION LESS THAN (1000)"},
		{`ALTER TABLE t REORGANIZE MAX PARTITION INTO NEW LAST PARTITION LESS THAN (1000)`, false, ""},
		{`ALTER TABLE t REORGANIZE MAX PARTITION INTO LAST PARTITION LESS THAN (1000)`, false, ""},
		{`ALTER TABLE t REORGANIZE MAXVALUE PARTITION INTO NEW LAST PARTITION LESS THAN (1000)`, false, ""},
		{`ALTER TABLE t REORGANIZE MAXVALUE PARTITION INTO LAST PARTITION LESS THAN (1000)`, false, ""},
		{`ALTER TABLE t split MAXVALUE PARTITION LESS THAN (1000)`, true, "ALTER TABLE `t` SPLIT MAXVALUE PARTITION LESS THAN (1000)"},
		{`ALTER TABLE t merge first PARTITION LESS THAN (1000)`, true, "ALTER TABLE `t` MERGE FIRST PARTITION LESS THAN (1000)"},
		{`ALTER TABLE t first PARTITION LESS THAN (1000)`, true, "ALTER TABLE `t` FIRST PARTITION LESS THAN (1000)"},
	}

	RunTest(t, table, false)
}

func TestTTLTableOption(t *testing.T) {
	table := []testCase{
		// create table with various temporal interval
		{"create table t (created_at datetime) TTL = created_at + INTERVAL 3.1415 YEAR", true, "CREATE TABLE `t` (`created_at` DATETIME) TTL = `created_at` + INTERVAL 3.1415 YEAR"},
		{"create table t (created_at datetime) TTL = created_at + INTERVAL '1 1:1:1' DAY_SECOND", true, "CREATE TABLE `t` (`created_at` DATETIME) TTL = `created_at` + INTERVAL _UTF8MB4'1 1:1:1' DAY_SECOND"},
		{"create table t (created_at datetime) TTL = created_at + INTERVAL 1 YEAR", true, "CREATE TABLE `t` (`created_at` DATETIME) TTL = `created_at` + INTERVAL 1 YEAR"},
		{"create table t (created_at datetime) TTL = created_at + INTERVAL 1 YEAR TTL_ENABLE = 'OFF'", true, "CREATE TABLE `t` (`created_at` DATETIME) TTL = `created_at` + INTERVAL 1 YEAR TTL_ENABLE = 'OFF'"},
		{"create table t (created_at datetime) TTL created_at + INTERVAL 1 YEAR TTL_ENABLE 'OFF'", true, "CREATE TABLE `t` (`created_at` DATETIME) TTL = `created_at` + INTERVAL 1 YEAR TTL_ENABLE = 'OFF'"},
		{"create table t (created_at datetime) TTL created_at + INTERVAL 1 YEAR TTL_ENABLE 'OFF' TTL_JOB_INTERVAL='8h'", true, "CREATE TABLE `t` (`created_at` DATETIME) TTL = `created_at` + INTERVAL 1 YEAR TTL_ENABLE = 'OFF' TTL_JOB_INTERVAL = '8h'"},
		{"create table t (created_at datetime) /*T![ttl] ttl=created_at + INTERVAL 1 YEAR ttl_enable='ON'*/", true, "CREATE TABLE `t` (`created_at` DATETIME) TTL = `created_at` + INTERVAL 1 YEAR TTL_ENABLE = 'ON'"},

		// alter table with various temporal interval
		{"alter table t TTL = created_at + INTERVAL 1 MONTH", true, "ALTER TABLE `t` TTL = `created_at` + INTERVAL 1 MONTH"},
		{"alter table t TTL_ENABLE = 'ON'", true, "ALTER TABLE `t` TTL_ENABLE = 'ON'"},
		{"alter table t TTL_ENABLE = 'OFF'", true, "ALTER TABLE `t` TTL_ENABLE = 'OFF'"},
		{"alter table t TTL = created_at + INTERVAL 1 MONTH TTL_ENABLE 'OFF'", true, "ALTER TABLE `t` TTL = `created_at` + INTERVAL 1 MONTH TTL_ENABLE = 'OFF'"},
		{"alter table t TTL = created_at + INTERVAL 1 MONTH TTL_ENABLE 'OFF' TTL_JOB_INTERVAL '1h'", true, "ALTER TABLE `t` TTL = `created_at` + INTERVAL 1 MONTH TTL_ENABLE = 'OFF' TTL_JOB_INTERVAL = '1h'"},
		{"alter table t /*T![ttl] ttl=created_at + INTERVAL 1 YEAR ttl_enable='ON'*/", true, "ALTER TABLE `t` TTL = `created_at` + INTERVAL 1 YEAR TTL_ENABLE = 'ON'"},
		{"alter table t /*T![ttl] ttl=created_at + INTERVAL 1 YEAR ttl_enable='ON' TTL_JOB_INTERVAL='8h'*/", true, "ALTER TABLE `t` TTL = `created_at` + INTERVAL 1 YEAR TTL_ENABLE = 'ON' TTL_JOB_INTERVAL = '8h'"},
		{"alter table t /*T![ttl] ttl=created_at + INTERVAL 1 YEAR ttl_enable='ON' TTL_JOB_INTERVAL='8.645124531235h'*/", true, "ALTER TABLE `t` TTL = `created_at` + INTERVAL 1 YEAR TTL_ENABLE = 'ON' TTL_JOB_INTERVAL = '8.645124531235h'"},

		// alter table to remove ttl settings
		{"alter table t remove ttl", true, "ALTER TABLE `t` REMOVE TTL"},

		// validate invalid TTL_ENABLE settings
		{"create table t (created_at datetime) TTL_ENABLE = 'test_case'", false, ""},
		{"create table t (created_at datetime) /*T![ttl] TTL_ENABLE = 'test_case' */", false, ""},
		{"alter table t /*T![ttl] TTL_ENABLE = 'test_case' */", false, ""},

		// validate invalid TTL_JOB_INTERVAL settings
		{"create table t (created_at datetime) TTL_JOB_INTERVAL = '@monthly'", false, ""},
		{"create table t (created_at datetime) TTL_JOB_INTERVAL = '10hourxx'", false, ""},
		{"create table t (created_at datetime) TTL_JOB_INTERVAL = '10.10.255h'", false, ""},
	}

	RunTest(t, table, false)
}

func TestIssue45898(t *testing.T) {
	p := parser.New()
	p.ParseSQL("a.")
	stmts, _, err := p.ParseSQL("select count(1) from t")
	require.NoError(t, err)
	var sb strings.Builder
	restoreCtx := NewRestoreCtx(DefaultRestoreFlags, &sb)
	sb.Reset()
	stmts[0].Restore(restoreCtx)
	require.Equal(t, "SELECT COUNT(1) FROM `t`", sb.String())
}

func TestMultiStmt(t *testing.T) {
	p := parser.New()
	stmts, _, err := p.Parse("SELECT 'foo'; SELECT 'foo;bar','baz'; select 'foo' , 'bar' , 'baz' ;select 1", "", "")
	require.NoError(t, err)
	require.Equal(t, len(stmts), 4)
	stmt1 := stmts[0].(*ast.SelectStmt)
	stmt2 := stmts[1].(*ast.SelectStmt)
	stmt3 := stmts[2].(*ast.SelectStmt)
	stmt4 := stmts[3].(*ast.SelectStmt)
	require.Equal(t, "'foo'", stmt1.Fields.Fields[0].Text())
	require.Equal(t, "'foo;bar'", stmt2.Fields.Fields[0].Text())
	require.Equal(t, "'baz'", stmt2.Fields.Fields[1].Text())
	require.Equal(t, "'foo'", stmt3.Fields.Fields[0].Text())
	require.Equal(t, "'bar'", stmt3.Fields.Fields[1].Text())
	require.Equal(t, "'baz'", stmt3.Fields.Fields[2].Text())
	require.Equal(t, "1", stmt4.Fields.Fields[0].Text())
}

// https://dev.mysql.com/doc/refman/8.1/en/other-vendor-data-types.html
func TestCompatTypes(t *testing.T) {
	table := []testCase{
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 BOOL)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` TINYINT(1))"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 BOOLEAN)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` TINYINT(1))"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 CHARACTER VARYING(0))`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` VARCHAR(0))"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 FIXED)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` DECIMAL)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 FLOAT4)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` FLOAT)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 FLOAT8)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` DOUBLE)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 INT1)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` TINYINT)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 INT2)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` SMALLINT)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 INT3)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` MEDIUMINT)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 INT4)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` INT)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 INT8)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` BIGINT)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 LONG VARBINARY)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` MEDIUMBLOB)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 LONG VARCHAR)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` MEDIUMTEXT)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 LONG)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` MEDIUMTEXT)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 MIDDLEINT)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` MEDIUMINT)"},
		{`CREATE TABLE t(id INT PRIMARY KEY, c1 NUMERIC)`, true, "CREATE TABLE `t` (`id` INT PRIMARY KEY,`c1` DECIMAL)"},
	}

	RunTest(t, table, false)
}

func TestVector(t *testing.T) {
	table := []testCase{
		{"CREATE TABLE t (a VECTOR)", true, "CREATE TABLE `t` (`a` VECTOR)"},
		{"CREATE TABLE t (a VECTOR<FLOAT>)", true, "CREATE TABLE `t` (`a` VECTOR)"},
		{"CREATE TABLE t (a VECTOR<INT>)", false, ""},
		{"CREATE TABLE t (a VECTOR<DOUBLE>)", false, ""},
		{"CREATE TABLE t (a VECTOR<ABC>)", false, ""},
		{"CREATE TABLE t (a VECTOR(5)<FLOAT>)", false, ""},
	}

	RunTest(t, table, false)
}

func TestExplainExplore(t *testing.T) {
	cases := []testCase{
		{`explain explore 'digestxxx'`, true, `EXPLAIN EXPLORE 'digestxxx'`},
		{`explain explore select 1 from t`, true, "EXPLAIN EXPLORE SELECT 1 FROM `t`"},
		{`explain explore select 1 from t1, t2`, true, "EXPLAIN EXPLORE SELECT 1 FROM (`t1`) JOIN `t2`"},
		{`explain explore select 1 from t where t1.a > (select max(a) from t2)`, true, "EXPLAIN EXPLORE SELECT 1 FROM `t` WHERE `t1`.`a`>(SELECT MAX(`a`) FROM `t2`)"},
	}
	RunTest(t, cases, false)
}

func TestSecondaryEngineAttribute(t *testing.T) {
	table := []testCase{
		// Valid Partition-level SECONDARY_ENGINE_ATTRIBUTE
		{
			"CREATE TABLE t (id INT) PARTITION BY RANGE (id) (" +
				"PARTITION p0 VALUES LESS THAN (10) SECONDARY_ENGINE_ATTRIBUTE='{\"key\":\"value\"}'," +
				"PARTITION p1 VALUES LESS THAN (20) SECONDARY_ENGINE_ATTRIBUTE='{\"key\":\"value2\"}')",
			true,
			"CREATE TABLE `t` (`id` INT) PARTITION BY RANGE (`id`) (" +
				"PARTITION `p0` VALUES LESS THAN (10) SECONDARY_ENGINE_ATTRIBUTE = '{\"key\":\"value\"}'," +
				"PARTITION `p1` VALUES LESS THAN (20) SECONDARY_ENGINE_ATTRIBUTE = '{\"key\":\"value2\"}')",
		},

		// Valid Table-level SECONDARY_ENGINE_ATTRIBUTE
		{
			"CREATE TABLE t (id INT) SECONDARY_ENGINE_ATTRIBUTE='{\"key\":\"value\"}'",
			true,
			"CREATE TABLE `t` (`id` INT) SECONDARY_ENGINE_ATTRIBUTE = '{\"key\":\"value\"}'",
		},

		// Valid Table-level and Partition-level SECONDARY_ENGINE_ATTRIBUTE
		{
			"CREATE TABLE t (id INT) SECONDARY_ENGINE_ATTRIBUTE='{\"key\":\"value\"}' PARTITION BY RANGE (id) (" +
				"PARTITION p0 VALUES LESS THAN (10) SECONDARY_ENGINE_ATTRIBUTE='{\"key\":\"partition_value\"}')",
			true,
			"CREATE TABLE `t` (`id` INT) SECONDARY_ENGINE_ATTRIBUTE = '{\"key\":\"value\"}' PARTITION BY RANGE (`id`) (" +
				"PARTITION `p0` VALUES LESS THAN (10) SECONDARY_ENGINE_ATTRIBUTE = '{\"key\":\"partition_value\"}')",
		},

		// Valid Column-level SECONDARY_ENGINE_ATTRIBUTE
		{
			"CREATE TABLE t (id INT SECONDARY_ENGINE_ATTRIBUTE='{\"key\":\"value\"}')",
			true,
			"CREATE TABLE `t` (`id` INT SECONDARY_ENGINE_ATTRIBUTE = '{\"key\":\"value\"}')",
		},

		// Valid: Table-level with tablespace option SECONDARY_ENGINE_ATTRIBUTE
		{
			"CREATE TABLE t (id INT) TABLESPACE ts1 SECONDARY_ENGINE_ATTRIBUTE='{\"key\":\"value\"}'",
			true,
			"CREATE TABLE `t` (`id` INT) TABLESPACE = `ts1` SECONDARY_ENGINE_ATTRIBUTE = '{\"key\":\"value\"}'",
		},

		// Valid: Index SECONDARY_ENGINE_ATTRIBUTE
		{
			"CREATE TABLE t (id INT,INDEX idx (id) INVISIBLE SECONDARY_ENGINE_ATTRIBUTE='{\"key1\":\"value1\"}')",
			true,
			"CREATE TABLE `t` (`id` INT,INDEX `idx`(`id`) INVISIBLE SECONDARY_ENGINE_ATTRIBUTE = '{\"key1\":\"value1\"}')",
		},

		// Missing value for SECONDARY_ENGINE_ATTRIBUTE at Partition-level
		{
			"CREATE TABLE t (id INT) PARTITION BY RANGE (id) (" +
				"PARTITION p0 VALUES LESS THAN (10) SECONDARY_ENGINE_ATTRIBUTE=)",
			false,
			"",
		},

		// Missing value for SECONDARY_ENGINE_ATTRIBUTE at Table-level
		{
			"CREATE TABLE t (id INT) SECONDARY_ENGINE_ATTRIBUTE=",
			false,
			"",
		},

		// Missing value for SECONDARY_ENGINE_ATTRIBUTE at Column-level
		{
			"CREATE TABLE t (id INT SECONDARY_ENGINE_ATTRIBUTE=)",
			false,
			"",
		},

		// Missing value for SECONDARY_ENGINE_ATTRIBUTE in Table-level with tablespace option
		{
			"CREATE TABLE t (id INT) TABLESPACE ts1 SECONDARY_ENGINE_ATTRIBUTE=",
			false,
			"",
		},

		// Missing value for SECONDARY_ENGINE_ATTRIBUTE at Index-level
		{
			"CREATE TABLE t (id INT, INDEX idx (id) SECONDARY_ENGINE_ATTRIBUTE=)",
			false,
			"",
		},

		// Invalid syntax for SECONDARY_ENGINE_ATTRIBUTE at Partition-level
		{
			"CREATE TABLE t (id INT) PARTITION BY RANGE (id) (" +
				"PARTITION p0 VALUES LESS THAN (10) SECONDARY_ENGINE_ATTRIBUTE)",
			false,
			"",
		},

		// Invalid syntax for SECONDARY_ENGINE_ATTRIBUTE at Table-level
		{
			"CREATE TABLE t (id INT) SECONDARY_ENGINE_ATTRIBUTE",
			false,
			"",
		},

		// Invalid syntax for SECONDARY_ENGINE_ATTRIBUTE at Column-level
		{
			"CREATE TABLE t (id INT SECONDARY_ENGINE_ATTRIBUTE)",
			false,
			"",
		},

		// Invalid syntax for SECONDARY_ENGINE_ATTRIBUTE in Table-level with tablespace option
		{
			"CREATE TABLE t (id INT) TABLESPACE ts1 SECONDARY_ENGINE_ATTRIBUTE",
			false,
			"",
		},

		// Invalid syntax for SECONDARY_ENGINE_ATTRIBUTE in Index-level
		{
			"CREATE TABLE t (id INT, INDEX idx (id) SECONDARY_ENGINE_ATTRIBUTE)",
			false,
			"",
		},

		// CREATE INDEX syntax for SECONDARY_ENGINE_ATTRIBUTE
		{
			"CREATE INDEX i ON t (a) SECONDARY_ENGINE_ATTRIBUTE = '{}'",
			true,
			"CREATE INDEX `i` ON `t` (`a`) SECONDARY_ENGINE_ATTRIBUTE = '{}'",
		},

		// CREATE INDEX syntax for SECONDARY_ENGINE_ATTRIBUTE
		{
			"CREATE INDEX i ON t (a) SECONDARY_ENGINE_ATTRIBUTE '{}'",
			true,
			"CREATE INDEX `i` ON `t` (`a`) SECONDARY_ENGINE_ATTRIBUTE = '{}'",
		},

		// Invalid CREATE INDEX syntax for SECONDARY_ENGINE_ATTRIBUTE
		{
			"CREATE INDEX i ON t (a) SECONDARY_ENGINE_ATTRIBUTE",
			false,
			"",
		},
	}

	RunTest(t, table, false)
}
