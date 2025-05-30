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

package tidb_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"math"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	gmysql "github.com/go-sql-driver/mysql"
	"github.com/pingcap/tidb/pkg/errno"
	"github.com/pingcap/tidb/pkg/lightning/backend"
	"github.com/pingcap/tidb/pkg/lightning/backend/encode"
	"github.com/pingcap/tidb/pkg/lightning/backend/kv"
	"github.com/pingcap/tidb/pkg/lightning/backend/tidb"
	"github.com/pingcap/tidb/pkg/lightning/common"
	"github.com/pingcap/tidb/pkg/lightning/config"
	"github.com/pingcap/tidb/pkg/lightning/errormanager"
	"github.com/pingcap/tidb/pkg/lightning/log"
	"github.com/pingcap/tidb/pkg/lightning/verification"
	"github.com/pingcap/tidb/pkg/meta/model"
	"github.com/pingcap/tidb/pkg/parser/ast"
	"github.com/pingcap/tidb/pkg/parser/charset"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	"github.com/pingcap/tidb/pkg/table"
	"github.com/pingcap/tidb/pkg/table/tables"
	"github.com/pingcap/tidb/pkg/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/zap"
)

type mysqlSuite struct {
	dbHandle   *sql.DB
	mockDB     sqlmock.Sqlmock
	mgr        backend.EngineManager
	backend    backend.Backend
	encBuilder encode.EncodingBuilder
	tbl        table.Table
}

func createMysqlSuite(t *testing.T) *mysqlSuite {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	tys := []byte{
		mysql.TypeLong, mysql.TypeLong, mysql.TypeTiny, mysql.TypeInt24, mysql.TypeFloat, mysql.TypeDouble,
		mysql.TypeDouble, mysql.TypeDouble, mysql.TypeVarchar, mysql.TypeBlob, mysql.TypeBit, mysql.TypeNewDecimal, mysql.TypeEnum,
	}
	cols := make([]*model.ColumnInfo, 0, len(tys))
	for i, ty := range tys {
		col := &model.ColumnInfo{ID: int64(i + 1), Name: ast.NewCIStr(fmt.Sprintf("c%d", i)), State: model.StatePublic, Offset: i, FieldType: *types.NewFieldType(ty)}
		cols = append(cols, col)
	}
	tblInfo := &model.TableInfo{ID: 1, Columns: cols, PKIsHandle: false, State: model.StatePublic}
	tbl, err := tables.TableFromMeta(kv.NewPanickingAllocators(tblInfo.SepAutoInc()), tblInfo)
	require.NoError(t, err)
	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.ReplaceOnDup
	cfg.Conflict.Threshold = math.MaxInt64
	cfg.Conflict.MaxRecordRows = 100
	backendObj := tidb.NewTiDBBackend(context.Background(), db, cfg, errormanager.New(nil, cfg, log.L()))
	return &mysqlSuite{
		dbHandle:   db,
		mockDB:     mock,
		mgr:        backend.MakeEngineManager(backendObj),
		backend:    backendObj,
		encBuilder: tidb.NewEncodingBuilder(),
		tbl:        tbl,
	}
}

func (s *mysqlSuite) TearDownTest(t *testing.T) {
	s.backend.Close()
	require.NoError(t, s.mockDB.ExpectationsWereMet())
}

func TestWriteRowsReplaceOnDup(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	s.mockDB.
		ExpectExec("\\QREPLACE INTO `foo`.`bar`(`b`,`d`,`e`,`f`,`g`,`h`,`i`,`j`,`k`,`l`,`m`,`n`,`o`) VALUES(-9223372036854775808,NULL,7.5,5e-324,1.7976931348623157e+308,0,'甲乙丙\\r\\n\\0\\Z''\"\\\\`',x'000000abcdef',2557891634,'12.5',51)\\E").
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	logger := log.L()

	engine, err := s.mgr.OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)

	dataRows := s.encBuilder.MakeEmptyRows()
	dataChecksum := verification.MakeKVChecksum(0, 0, 0)
	indexRows := s.encBuilder.MakeEmptyRows()
	indexChecksum := verification.MakeKVChecksum(0, 0, 0)

	cols := s.tbl.Cols()
	perms := make([]int, 0, len(s.tbl.Cols())+1)
	for i := range cols {
		perms = append(perms, i)
	}
	perms = append(perms, -1)
	// skip column a,c due to ignore-columns
	perms[0] = -1
	perms[2] = -1
	encoder, err := s.encBuilder.NewEncoder(context.Background(), &encode.EncodingConfig{
		SessionOptions: encode.SessionOptions{SQLMode: 0, Timestamp: 1234567890},
		Table:          s.tbl,
		Logger:         logger,
	})
	require.NoError(t, err)
	row, err := encoder.Encode([]types.Datum{
		types.NewUintDatum(18446744073709551615),
		types.NewIntDatum(-9223372036854775808),
		types.NewUintDatum(0),
		{},
		types.NewFloat32Datum(7.5),
		types.NewFloat64Datum(5e-324),
		types.NewFloat64Datum(1.7976931348623157e+308),
		types.NewFloat64Datum(0.0),
		// In Go, the floating-point literal '-0.0' is the same as '0.0', it does not produce a negative zero.
		// types.NewFloat64Datum(-0.0),
		types.NewStringDatum("甲乙丙\r\n\x00\x1a'\"\\`"),
		types.NewBinaryLiteralDatum(types.NewBinaryLiteralFromUint(0xabcdef, 6)),
		types.NewMysqlBitDatum(types.NewBinaryLiteralFromUint(0x98765432, 4)),
		types.NewDecimalDatum(types.NewDecFromFloatForTest(12.5)),
		types.NewMysqlEnumDatum(types.Enum{Name: "ENUM_NAME", Value: 51}),
	}, 1, perms, 0)
	require.NoError(t, err)
	row.ClassifyAndAppend(&dataRows, &dataChecksum, &indexRows, &indexChecksum)

	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"b", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o"}, dataRows)
	require.NoError(t, err)
	st, err := writer.Close(ctx)
	require.NoError(t, err)
	require.Nil(t, st)
}

func TestWriteRowsIgnoreOnDup(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	s.mockDB.
		ExpectExec("\\QINSERT IGNORE INTO `foo`.`bar`(`a`) VALUES(1)\\E").
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	logger := log.L()

	encBuilder := tidb.NewEncodingBuilder()
	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.IgnoreOnDup
	cfg.Conflict.Threshold = math.MaxInt64
	cfg.Conflict.MaxRecordRows = 0
	ignoreBackend := tidb.NewTiDBBackend(ctx, s.dbHandle, cfg, errormanager.New(nil, cfg, logger))
	engine, err := backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)

	dataRows := encBuilder.MakeEmptyRows()
	dataChecksum := verification.MakeKVChecksum(0, 0, 0)
	indexRows := encBuilder.MakeEmptyRows()
	indexChecksum := verification.MakeKVChecksum(0, 0, 0)

	encoder, err := encBuilder.NewEncoder(ctx, &encode.EncodingConfig{Table: s.tbl, Logger: logger})
	require.NoError(t, err)
	row, err := encoder.Encode([]types.Datum{
		types.NewIntDatum(1),
	}, 1, []int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}, 0)
	require.NoError(t, err)
	row.ClassifyAndAppend(&dataRows, &dataChecksum, &indexRows, &indexChecksum)

	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.NoError(t, err)
	_, err = writer.Close(ctx)
	require.NoError(t, err)

	// test conflict.strategy == ignore and not 0 conflict.max-record-rows will use ErrorOnDup

	cfg.Conflict.MaxRecordRows = 10
	ignoreBackend = tidb.NewTiDBBackend(ctx, s.dbHandle, cfg, errormanager.New(nil, cfg, logger))
	engine, err = backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)

	dataRows = encBuilder.MakeEmptyRows()
	dataChecksum = verification.MakeKVChecksum(0, 0, 0)
	indexRows = encBuilder.MakeEmptyRows()
	indexChecksum = verification.MakeKVChecksum(0, 0, 0)

	row, err = encoder.Encode([]types.Datum{
		types.NewIntDatum(2),
	}, 1, []int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}, 0)
	require.NoError(t, err)
	row.ClassifyAndAppend(&dataRows, &dataChecksum, &indexRows, &indexChecksum)

	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(2)\\E").
		WillReturnResult(sqlmock.NewResult(1, 1))

	writer, err = engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.NoError(t, err)
	_, err = writer.Close(ctx)
	require.NoError(t, err)

	// test encode rows with _tidb_rowid
	encoder, err = encBuilder.NewEncoder(ctx, &encode.EncodingConfig{Table: s.tbl, Logger: logger})
	require.NoError(t, err)
	rowWithID, err := encoder.Encode([]types.Datum{
		types.NewIntDatum(1),
		types.NewIntDatum(1), // _tidb_rowid field
	}, 1, []int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, 1}, 0)
	require.NoError(t, err)
	// tidbRow is stringer.
	require.Equal(t, "(1,1)", fmt.Sprint(rowWithID))
}

func TestWriteRowsErrorOnDup(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(1)\\E").
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	logger := log.L()

	encBuilder := tidb.NewEncodingBuilder()
	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.ErrorOnDup
	cfg.Conflict.Threshold = math.MaxInt64
	cfg.Conflict.MaxRecordRows = 0
	ignoreBackend := tidb.NewTiDBBackend(ctx, s.dbHandle, cfg, errormanager.New(nil, cfg, logger))
	engine, err := backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)

	dataRows := encBuilder.MakeEmptyRows()
	dataChecksum := verification.MakeKVChecksum(0, 0, 0)
	indexRows := encBuilder.MakeEmptyRows()
	indexChecksum := verification.MakeKVChecksum(0, 0, 0)

	encoder, err := encBuilder.NewEncoder(ctx, &encode.EncodingConfig{Table: s.tbl, Logger: logger})
	require.NoError(t, err)
	row, err := encoder.Encode([]types.Datum{
		types.NewIntDatum(1),
	}, 1, []int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}, 0)
	require.NoError(t, err)

	row.ClassifyAndAppend(&dataRows, &dataChecksum, &indexRows, &indexChecksum)

	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.NoError(t, err)
	st, err := writer.Close(ctx)
	require.NoError(t, err)
	require.Nil(t, st)
}

// TODO: temporarily disable this test before we fix strict mode
//
//nolint:unused,deadcode
func testStrictMode(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	ft := *types.NewFieldType(mysql.TypeVarchar)
	ft.SetCharset(charset.CharsetUTF8MB4)
	col0 := &model.ColumnInfo{ID: 1, Name: ast.NewCIStr("s0"), State: model.StatePublic, Offset: 0, FieldType: ft}
	ft = *types.NewFieldType(mysql.TypeString)
	ft.SetCharset(charset.CharsetASCII)
	col1 := &model.ColumnInfo{ID: 2, Name: ast.NewCIStr("s1"), State: model.StatePublic, Offset: 1, FieldType: ft}
	tblInfo := &model.TableInfo{ID: 1, Columns: []*model.ColumnInfo{col0, col1}, PKIsHandle: false, State: model.StatePublic}
	tbl, err := tables.TableFromMeta(kv.NewPanickingAllocators(tblInfo.SepAutoInc()), tblInfo)
	require.NoError(t, err)

	ctx := context.Background()

	encBuilder := tidb.NewEncodingBuilder()
	logger := log.L()
	encoder, err := encBuilder.NewEncoder(ctx, &encode.EncodingConfig{
		SessionOptions: encode.SessionOptions{SQLMode: mysql.ModeStrictAllTables},
		Table:          tbl,
		Logger:         log.L(),
	})
	require.NoError(t, err)

	_, err = encoder.Encode([]types.Datum{
		types.NewStringDatum("test"),
	}, 1, []int{0, -1, -1}, 0)
	require.NoError(t, err)

	_, err = encoder.Encode([]types.Datum{
		types.NewStringDatum("\xff\xff\xff\xff"),
	}, 1, []int{0, -1, -1}, 0)
	require.Error(t, err)
	require.Regexp(t, `incorrect utf8 value .* for column s0$`, err.Error())

	// oepn a new encode because column count changed.
	encoder, err = encBuilder.NewEncoder(ctx, &encode.EncodingConfig{
		SessionOptions: encode.SessionOptions{SQLMode: mysql.ModeStrictAllTables},
		Table:          tbl,
		Logger:         logger,
	})
	require.NoError(t, err)
	_, err = encoder.Encode([]types.Datum{
		types.NewStringDatum(""),
		types.NewStringDatum("非 ASCII 字符串"),
	}, 1, []int{0, 1, -1}, 0)
	require.Error(t, err)
	require.Regexp(t, "incorrect ascii value .* for column s1$", err.Error())
}

func TestFetchRemoteTableModels_4_0(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	s.mockDB.ExpectBegin()
	s.mockDB.ExpectQuery("\\QSELECT table_name, column_name, column_type, generation_expression, extra FROM information_schema.columns WHERE table_schema = ? AND table_name IN (?) ORDER BY table_name, ordinal_position;\\E").
		WithArgs("test", "t").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "column_type", "generation_expression", "extra"}).
			AddRow("t", "id", "bigint(20) unsigned", "", "auto_increment"))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`t` NEXT_ROW_ID").
		WillReturnRows(sqlmock.NewRows([]string{"DB_NAME", "TABLE_NAME", "COLUMN_NAME", "NEXT_GLOBAL_ROW_ID"}).
			AddRow("test", "t", "id", "10942694589135710585"))
	s.mockDB.ExpectCommit()

	targetInfoGetter := tidb.NewTargetInfoGetter(s.dbHandle)
	tableInfos, err := targetInfoGetter.FetchRemoteTableModels(context.Background(), "test", []string{"t"})
	require.NoError(t, err)
	ft := types.FieldType{}
	ft.SetFlag(mysql.AutoIncrementFlag | mysql.UnsignedFlag)
	require.Equal(t, map[string]*model.TableInfo{
		"t": {
			Name:       ast.NewCIStr("t"),
			State:      model.StatePublic,
			PKIsHandle: true,
			Columns: []*model.ColumnInfo{
				{
					Name:      ast.NewCIStr("id"),
					Offset:    0,
					State:     model.StatePublic,
					FieldType: ft,
				},
			},
		},
	}, tableInfos)
}

func TestFetchRemoteTableModels_4_x_auto_increment(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	s.mockDB.ExpectBegin()
	s.mockDB.ExpectQuery("\\QSELECT table_name, column_name, column_type, generation_expression, extra FROM information_schema.columns WHERE table_schema = ? AND table_name IN (?) ORDER BY table_name, ordinal_position;\\E").
		WithArgs("test", "t").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "column_type", "generation_expression", "extra"}).
			AddRow("t", "id", "bigint(20)", "", ""))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`t` NEXT_ROW_ID").
		WillReturnRows(sqlmock.NewRows([]string{"DB_NAME", "TABLE_NAME", "COLUMN_NAME", "NEXT_GLOBAL_ROW_ID", "ID_TYPE"}).
			AddRow("test", "t", "id", int64(1), "AUTO_INCREMENT"))
	s.mockDB.ExpectCommit()

	targetInfoGetter := tidb.NewTargetInfoGetter(s.dbHandle)
	tableInfos, err := targetInfoGetter.FetchRemoteTableModels(context.Background(), "test", []string{"t"})
	require.NoError(t, err)
	ft := types.FieldType{}
	ft.SetFlag(mysql.AutoIncrementFlag)
	require.Equal(t, map[string]*model.TableInfo{
		"t": {
			Name:       ast.NewCIStr("t"),
			State:      model.StatePublic,
			PKIsHandle: true,
			Columns: []*model.ColumnInfo{
				{
					Name:      ast.NewCIStr("id"),
					Offset:    0,
					State:     model.StatePublic,
					FieldType: ft,
				},
			},
		},
	}, tableInfos)
}

func TestFetchRemoteTableModels_4_x_auto_random(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	s.mockDB.ExpectBegin()
	s.mockDB.ExpectQuery("\\QSELECT table_name, column_name, column_type, generation_expression, extra FROM information_schema.columns WHERE table_schema = ? AND table_name IN (?) ORDER BY table_name, ordinal_position;\\E").
		WithArgs("test", "t").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "column_type", "generation_expression", "extra"}).
			AddRow("t", "id", "bigint(20)", "1 + 2", ""))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`t` NEXT_ROW_ID").
		WillReturnRows(sqlmock.NewRows([]string{"DB_NAME", "TABLE_NAME", "COLUMN_NAME", "NEXT_GLOBAL_ROW_ID", "ID_TYPE"}).
			AddRow("test", "t", "id", int64(1), "AUTO_RANDOM"))
	s.mockDB.ExpectCommit()

	targetInfoGetter := tidb.NewTargetInfoGetter(s.dbHandle)
	tableInfos, err := targetInfoGetter.FetchRemoteTableModels(context.Background(), "test", []string{"t"})
	require.NoError(t, err)
	ft := types.FieldType{}
	ft.SetFlag(mysql.PriKeyFlag)
	require.Equal(t, map[string]*model.TableInfo{
		"t": {
			Name:           ast.NewCIStr("t"),
			State:          model.StatePublic,
			PKIsHandle:     true,
			AutoRandomBits: 1,
			Columns: []*model.ColumnInfo{
				{
					Name:                ast.NewCIStr("id"),
					Offset:              0,
					State:               model.StatePublic,
					FieldType:           ft,
					GeneratedExprString: "1 + 2",
				},
			},
		},
	}, tableInfos)
}

func TestFetchRemoteTableModelsDropTableHalfway(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	s.mockDB.ExpectBegin()
	s.mockDB.ExpectQuery("\\QSELECT table_name, column_name, column_type, generation_expression, extra FROM information_schema.columns WHERE table_schema = ? AND table_name IN (?,?) ORDER BY table_name, ordinal_position;\\E").
		WithArgs("test", "tbl01", "tbl02").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "column_type", "generation_expression", "extra"}).
			AddRow("tbl01", "id", "bigint(20)", "", "auto_increment").
			AddRow("tbl01", "val", "varchar(255)", "", "").
			AddRow("tbl02", "id", "bigint(20)", "", "auto_increment").
			AddRow("tbl02", "val", "varchar(255)", "", ""))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`tbl01` NEXT_ROW_ID").
		WillReturnRows(sqlmock.NewRows([]string{"DB_NAME", "TABLE_NAME", "COLUMN_NAME", "NEXT_GLOBAL_ROW_ID", "ID_TYPE"}).
			AddRow("test", "tbl01", "id", int64(1), "_TIDB_ROWID").
			AddRow("test", "tbl01", "id", int64(1), "AUTO_INCREMENT"))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`tbl02` NEXT_ROW_ID").
		WillReturnError(mysql.NewErr(mysql.ErrNoSuchTable, "test", "tbl02"))
	s.mockDB.ExpectCommit()

	infoGetter := tidb.NewTargetInfoGetter(s.dbHandle)
	tableInfos, err := infoGetter.FetchRemoteTableModels(
		context.Background(),
		"test",
		[]string{"tbl01", "tbl02"},
	)
	require.NoError(t, err)
	ft := types.FieldType{}
	ft.SetFlag(mysql.AutoIncrementFlag)
	require.Equal(t, map[string]*model.TableInfo{
		"tbl01": {
			Name:       ast.NewCIStr("tbl01"),
			State:      model.StatePublic,
			PKIsHandle: true,
			Columns: []*model.ColumnInfo{
				{
					Name:      ast.NewCIStr("id"),
					Offset:    0,
					State:     model.StatePublic,
					FieldType: ft,
				},
				{
					Name:   ast.NewCIStr("val"),
					Offset: 1,
					State:  model.StatePublic,
				},
			},
		},
	}, tableInfos)
}

func TestFetchRemoteTableModelsConcurrency(t *testing.T) {
	backupConcurrency := tidb.FetchRemoteTableModelsConcurrency
	tidb.FetchRemoteTableModelsConcurrency = 2
	backupBatchSize := tidb.FetchRemoteTableModelsBatchSize
	tidb.FetchRemoteTableModelsBatchSize = 3
	t.Cleanup(func() {
		tidb.FetchRemoteTableModelsConcurrency = backupConcurrency
		tidb.FetchRemoteTableModelsBatchSize = backupBatchSize
	})

	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	s.mockDB.MatchExpectationsInOrder(false)

	s.mockDB.ExpectBegin()
	s.mockDB.ExpectQuery("\\QSELECT table_name, column_name, column_type, generation_expression, extra FROM information_schema.columns WHERE table_schema = ? AND table_name IN (?,?,?) ORDER BY table_name, ordinal_position;\\E").
		WithArgs("test", "t1", "t2", "t3").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "column_type", "generation_expression", "extra"}).
			AddRow("t1", "id", "bigint(20)", "", "").
			AddRow("t2", "id", "bigint(20)", "", ""))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`t1` NEXT_ROW_ID").
		WillReturnRows(sqlmock.NewRows([]string{"DB_NAME", "TABLE_NAME", "COLUMN_NAME", "NEXT_GLOBAL_ROW_ID", "ID_TYPE"}).
			AddRow("test", "t1", "id", int64(1), "_TIDB_ROWID"))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`t2` NEXT_ROW_ID").
		WillReturnRows(sqlmock.NewRows([]string{"DB_NAME", "TABLE_NAME", "COLUMN_NAME", "NEXT_GLOBAL_ROW_ID", "ID_TYPE"}).
			AddRow("test", "t2", "id", int64(1), "_TIDB_ROWID"))
	s.mockDB.ExpectCommit()

	s.mockDB.ExpectBegin()
	s.mockDB.ExpectQuery("\\QSELECT table_name, column_name, column_type, generation_expression, extra FROM information_schema.columns WHERE table_schema = ? AND table_name IN (?,?,?) ORDER BY table_name, ordinal_position;\\E").
		WithArgs("test", "t4", "t5", "t6").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "column_type", "generation_expression", "extra"}).
			AddRow("t4", "id", "bigint(20)", "", "").
			AddRow("t6", "id", "bigint(20)", "", ""))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`t4` NEXT_ROW_ID").
		WillReturnRows(sqlmock.NewRows([]string{"DB_NAME", "TABLE_NAME", "COLUMN_NAME", "NEXT_GLOBAL_ROW_ID", "ID_TYPE"}).
			AddRow("test", "t4", "id", int64(1), "_TIDB_ROWID"))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`t6` NEXT_ROW_ID").
		WillReturnRows(sqlmock.NewRows([]string{"DB_NAME", "TABLE_NAME", "COLUMN_NAME", "NEXT_GLOBAL_ROW_ID", "ID_TYPE"}).
			AddRow("test", "t6", "id", int64(1), "_TIDB_ROWID"))
	s.mockDB.ExpectCommit()

	s.mockDB.ExpectBegin()
	s.mockDB.ExpectQuery("\\QSELECT table_name, column_name, column_type, generation_expression, extra FROM information_schema.columns WHERE table_schema = ? AND table_name IN (?,?,?) ORDER BY table_name, ordinal_position;\\E").
		WithArgs("test", "t7", "t8", "t9").
		WillReturnRows(sqlmock.NewRows([]string{"table_name", "column_name", "column_type", "generation_expression", "extra"}).
			AddRow("t8", "id", "bigint(20)", "", "").
			AddRow("t9", "id", "bigint(20)", "", ""))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`t8` NEXT_ROW_ID").
		WillReturnRows(sqlmock.NewRows([]string{"DB_NAME", "TABLE_NAME", "COLUMN_NAME", "NEXT_GLOBAL_ROW_ID", "ID_TYPE"}).
			AddRow("test", "t8", "id", int64(1), "_TIDB_ROWID"))
	s.mockDB.ExpectQuery("SHOW TABLE `test`.`t9` NEXT_ROW_ID").
		WillReturnRows(sqlmock.NewRows([]string{"DB_NAME", "TABLE_NAME", "COLUMN_NAME", "NEXT_GLOBAL_ROW_ID", "ID_TYPE"}).
			AddRow("test", "t9", "id", int64(1), "_TIDB_ROWID"))
	s.mockDB.ExpectCommit()

	infoGetter := tidb.NewTargetInfoGetter(s.dbHandle)
	tableInfos, err := infoGetter.FetchRemoteTableModels(
		context.Background(),
		"test",
		[]string{"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9"},
	)
	require.NoError(t, err)
	require.Len(t, tableInfos, 6)
	require.Contains(t, tableInfos, "t1")
	require.Contains(t, tableInfos, "t2")
	require.Contains(t, tableInfos, "t4")
	require.Contains(t, tableInfos, "t6")
	require.Contains(t, tableInfos, "t8")
	require.Contains(t, tableInfos, "t9")
}

func TestWriteRowsErrorNoRetry(t *testing.T) {
	nonRetryableError := sql.ErrNoRows
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)

	// batch insert, fail and rollback.
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(1),(2),(3),(4),(5)\\E").
		WillReturnError(nonRetryableError)

	// disable error record, should not expect retry statements one by one.
	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.ErrorOnDup
	cfg.Conflict.Threshold = 0
	cfg.Conflict.MaxRecordRows = 0
	ignoreBackend := tidb.NewTiDBBackend(context.Background(), s.dbHandle, cfg, errormanager.New(s.dbHandle, cfg, log.L()))
	encBuilder := tidb.NewEncodingBuilder()
	dataRows := encodeRowsTiDB(t, encBuilder, s.tbl)
	ctx := context.Background()
	engine, err := backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)
	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.Error(t, err)
	require.False(t, common.IsRetryableError(err), "err: %v", err)
}

func TestWriteRowsErrorDowngradingAll(t *testing.T) {
	nonRetryableError := sql.ErrNoRows
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	// First, batch insert, fail and rollback.
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(1),(2),(3),(4),(5)\\E").
		WillReturnError(nonRetryableError)
	// Then, insert row-by-row due to the non-retryable error.
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(1)\\E").
		WillReturnError(nonRetryableError)
	s.mockDB.
		ExpectExec("INSERT INTO `tidb_lightning_errors`\\.type_error_v2.*").
		WithArgs(sqlmock.AnyArg(), "`foo`.`bar`", "7.csv", int64(0), nonRetryableError.Error(), "(1)").
		WillReturnResult(driver.ResultNoRows)
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(2)\\E").
		WillReturnError(nonRetryableError)
	s.mockDB.
		ExpectExec("INSERT INTO `tidb_lightning_errors`\\.type_error_v2.*").
		WithArgs(sqlmock.AnyArg(), "`foo`.`bar`", "8.csv", int64(0), nonRetryableError.Error(), "(2)").
		WillReturnResult(driver.ResultNoRows)
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(3)\\E").
		WillReturnError(nonRetryableError)
	s.mockDB.
		ExpectExec("INSERT INTO `tidb_lightning_errors`\\.type_error_v2.*").
		WithArgs(sqlmock.AnyArg(), "`foo`.`bar`", "9.csv", int64(0), nonRetryableError.Error(), "(3)").
		WillReturnResult(driver.ResultNoRows)
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(4)\\E").
		WillReturnError(nonRetryableError)
	s.mockDB.
		ExpectExec("INSERT INTO `tidb_lightning_errors`\\.type_error_v2.*").
		WithArgs(sqlmock.AnyArg(), "`foo`.`bar`", "10.csv", int64(0), nonRetryableError.Error(), "(4)").
		WillReturnResult(driver.ResultNoRows)
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(5)\\E").
		WillReturnError(nonRetryableError)
	s.mockDB.
		ExpectExec("INSERT INTO `tidb_lightning_errors`\\.type_error_v2.*").
		WithArgs(sqlmock.AnyArg(), "`foo`.`bar`", "11.csv", int64(0), nonRetryableError.Error(), "(5)").
		WillReturnResult(driver.ResultNoRows)

	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.IgnoreOnDup
	cfg.Conflict.Threshold = 100
	cfg.Conflict.MaxRecordRows = 10
	cfg.App.TaskInfoSchemaName = "tidb_lightning_errors"
	cfg.App.MaxError.Type = *atomic.NewInt64(10)
	ignoreBackend := tidb.NewTiDBBackend(context.Background(), s.dbHandle, cfg, errormanager.New(s.dbHandle, cfg, log.L()))
	encBuilder := tidb.NewEncodingBuilder()
	dataRows := encodeRowsTiDB(t, encBuilder, s.tbl)
	ctx := context.Background()
	engine, err := backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)
	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.NoError(t, err)
}

func TestWriteRowsErrorDowngradingExceedThreshold(t *testing.T) {
	nonRetryableError := sql.ErrNoRows
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	// First, batch insert, fail and rollback.
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(1),(2),(3),(4),(5)\\E").
		WillReturnError(nonRetryableError)
	// Then, insert row-by-row due to the non-retryable error.
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(1)\\E").
		WillReturnError(nonRetryableError)
	s.mockDB.
		ExpectExec("INSERT INTO `tidb_lightning_errors`\\.type_error_v2.*").
		WithArgs(sqlmock.AnyArg(), "`foo`.`bar`", "7.csv", int64(0), nonRetryableError.Error(), "(1)").
		WillReturnResult(driver.ResultNoRows)
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(2)\\E").
		WillReturnError(nonRetryableError)
	s.mockDB.
		ExpectExec("INSERT INTO `tidb_lightning_errors`\\.type_error_v2.*").
		WithArgs(sqlmock.AnyArg(), "`foo`.`bar`", "8.csv", int64(0), nonRetryableError.Error(), "(2)").
		WillReturnResult(driver.ResultNoRows)
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(3)\\E").
		WillReturnError(nonRetryableError)
	s.mockDB.
		ExpectExec("INSERT INTO `tidb_lightning_errors`\\.type_error_v2.*").
		WithArgs(sqlmock.AnyArg(), "`foo`.`bar`", "9.csv", int64(0), nonRetryableError.Error(), "(3)").
		WillReturnResult(driver.ResultNoRows)
	// the forth row will exceed the error threshold, won't record this error
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(4)\\E").
		WillReturnError(nonRetryableError)

	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.IgnoreOnDup
	cfg.Conflict.Threshold = 100
	cfg.Conflict.MaxRecordRows = 10
	cfg.App.TaskInfoSchemaName = "tidb_lightning_errors"
	cfg.App.MaxError.Type = *atomic.NewInt64(3)
	ignoreBackend := tidb.NewTiDBBackend(context.Background(), s.dbHandle, cfg, errormanager.New(s.dbHandle, cfg, log.L()))
	encBuilder := tidb.NewEncodingBuilder()
	dataRows := encodeRowsTiDB(t, encBuilder, s.tbl)
	ctx := context.Background()
	engine, err := backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)
	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.Error(t, err)
	st, err := writer.Close(ctx)
	require.NoError(t, err)
	require.Nil(t, st)
}

func TestWriteRowsRecordOneError(t *testing.T) {
	dupErr := &gmysql.MySQLError{Number: errno.ErrDupEntry, Message: "Duplicate entry '2' for key 'PRIMARY'"}
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	// First, batch insert, fail and rollback.
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(1),(2),(3),(4),(5)\\E").
		WillReturnError(dupErr)
	// Then, insert row-by-row due to the non-retryable error.
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(1)\\E").
		WillReturnResult(sqlmock.NewResult(1, 1))
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(2)\\E").
		WillReturnError(dupErr)
	s.mockDB.
		ExpectExec("INSERT INTO `tidb_lightning_errors`\\.conflict_records_v2.*").
		WithArgs(sqlmock.AnyArg(), "`foo`.`bar`", "8.csv", int64(0), dupErr.Error(), 0, "(2)").
		WillReturnResult(driver.ResultNoRows)

	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.ErrorOnDup
	cfg.Conflict.Threshold = 0
	cfg.Conflict.MaxRecordRows = 0
	cfg.App.TaskInfoSchemaName = "tidb_lightning_errors"
	ignoreBackend := tidb.NewTiDBBackend(context.Background(), s.dbHandle, cfg, errormanager.New(s.dbHandle, cfg, log.L()))
	encBuilder := tidb.NewEncodingBuilder()
	dataRows := encodeRowsTiDB(t, encBuilder, s.tbl)
	ctx := context.Background()
	engine, err := backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)
	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.ErrorContains(t, err, "Duplicate entry '2' for key 'PRIMARY'")
	st, err := writer.Close(ctx)
	require.NoError(t, err)
	require.Nil(t, st)
}

func TestDuplicateThreshold(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	// batch insert 5 rows, but 4 are already in the table
	s.mockDB.
		ExpectExec("\\QINSERT IGNORE INTO `foo`.`bar`(`a`) VALUES(1),(2),(3),(4),(5)\\E").
		WillReturnResult(sqlmock.NewResult(5, 1))

	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.IgnoreOnDup
	cfg.Conflict.Threshold = 5
	cfg.Conflict.MaxRecordRows = 0
	ignoreBackend := tidb.NewTiDBBackend(context.Background(), s.dbHandle, cfg, errormanager.New(s.dbHandle, cfg, log.L()))
	encBuilder := tidb.NewEncodingBuilder()
	dataRows := encodeRowsTiDB(t, encBuilder, s.tbl)
	ctx := context.Background()
	engine, err := backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)
	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.NoError(t, err)
	st, err := writer.Close(ctx)
	require.NoError(t, err)
	require.Nil(t, st)

	// batch insert 5 rows, all are already in the table, and reach conflict.threshold

	s.mockDB.
		ExpectExec("\\QINSERT IGNORE INTO `foo`.`bar`(`a`) VALUES(1),(2),(3),(4),(5)\\E").
		WillReturnResult(sqlmock.NewResult(5, 0))
	writer, err = engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.ErrorContains(t, err, "The number of conflict errors exceeds the threshold configured by `conflict.threshold`: '5'")
	st, err = writer.Close(ctx)
	require.NoError(t, err)
	require.Nil(t, st)
}

func encodeRowsTiDB(t *testing.T, encBuilder encode.EncodingBuilder, tbl table.Table) encode.Rows {
	dataRows := encBuilder.MakeEmptyRows()
	dataChecksum := verification.MakeKVChecksum(0, 0, 0)
	indexRows := encBuilder.MakeEmptyRows()
	indexChecksum := verification.MakeKVChecksum(0, 0, 0)

	rowCases := []struct {
		row        []types.Datum
		rowID      int64
		colMapping []int
		path       string
		offset     int64
	}{
		{
			row:        []types.Datum{types.NewIntDatum(1)},
			rowID:      1,
			colMapping: []int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
			path:       "7.csv",
			offset:     0,
		},
		{
			row:        []types.Datum{types.NewIntDatum(2)},
			rowID:      1,
			colMapping: []int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
			path:       "8.csv",
		},
		{
			row:        []types.Datum{types.NewIntDatum(3)},
			rowID:      1,
			colMapping: []int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
			path:       "9.csv",
		},
		{
			row:        []types.Datum{types.NewIntDatum(4)},
			rowID:      1,
			colMapping: []int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
			path:       "10.csv",
		},
		{
			row:        []types.Datum{types.NewIntDatum(5)},
			rowID:      1,
			colMapping: []int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
			path:       "11.csv",
		},
	}
	for _, rc := range rowCases {
		encoder, err := encBuilder.NewEncoder(context.Background(), &encode.EncodingConfig{
			SessionOptions: encode.SessionOptions{LogicalImportPrepStmt: true},
			Path:           rc.path,
			Table:          tbl,
			Logger:         log.L(),
		})
		require.NoError(t, err)
		row, err := encoder.Encode(rc.row, rc.rowID, rc.colMapping, rc.offset)
		require.NoError(t, err)
		row.ClassifyAndAppend(&dataRows, &dataChecksum, &indexRows, &indexChecksum)
	}

	rawRow := make([]types.Datum, 0)
	for range 15 {
		rawRow = append(rawRow, types.NewIntDatum(0))
	}
	encoder, err := encBuilder.NewEncoder(context.Background(), &encode.EncodingConfig{
		Path:   "12.csv",
		Table:  tbl,
		Logger: log.L(),
	})
	require.NoError(t, err)
	_, err = encoder.Encode(rawRow, 1, []int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1}, 0)
	require.NotNil(t, err)
	require.Contains(t, err.Error(), "column count mismatch, at most")
	return dataRows
}

func TestEncodeRowForRecord(t *testing.T) {
	s := createMysqlSuite(t)

	// for a correct row, the will encode a correct result
	row := tidb.EncodeRowForRecord(context.Background(), s.tbl, mysql.ModeStrictTransTables, []types.Datum{
		types.NewIntDatum(5),
		types.NewStringDatum("test test"),
		types.NewBinaryLiteralDatum(types.NewBinaryLiteralFromUint(0xabcdef, 6)),
	}, []int{0, -1, -1, -1, -1, -1, -1, -1, 1, 2, -1, -1, -1, -1})
	require.Equal(t, row, "(5,'test test',x'000000abcdef')")

	// the following row will result in column count mismatch error, there for encode
	// result will fallback to a "," separated string list.
	row = tidb.EncodeRowForRecord(context.Background(), s.tbl, mysql.ModeStrictTransTables, []types.Datum{
		types.NewIntDatum(5),
		types.NewStringDatum("test test"),
		types.NewBinaryLiteralDatum(types.NewBinaryLiteralFromUint(0xabcdef, 6)),
	}, []int{0, -1, -1, -1, -1, -1, -1, -1, 1, 2, 3, -1, -1, -1})
	require.Equal(t, row, "(5, \"test test\", \x00\x00\x00\xab\xcd\xef)")
}

// TestLogicalImportBatch tests that each INSERT statement is limited by both
// logical-import-batch-size and logical-import-batch-rows configurations. Here
// we ensure each INSERT statement has up to 5 rows *and* ~30 bytes of values.
func TestLogicalImportBatch(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)

	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(1),(2),(4),(8),(16)\\E").
		WillReturnResult(sqlmock.NewResult(5, 5))
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(32),(64),(128),(256),(512)\\E").
		WillReturnResult(sqlmock.NewResult(5, 5))
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(1024),(2048),(4096),(8192)\\E").
		WillReturnResult(sqlmock.NewResult(4, 4))
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(16384),(32768),(65536),(131072)\\E").
		WillReturnResult(sqlmock.NewResult(4, 4))
	s.mockDB.
		ExpectExec("\\QINSERT INTO `foo`.`bar`(`a`) VALUES(262144)\\E").
		WillReturnResult(sqlmock.NewResult(1, 1))

	ctx := context.Background()
	logger := log.L()

	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.ErrorOnDup
	cfg.TikvImporter.LogicalImportBatchSize = 30
	cfg.TikvImporter.LogicalImportBatchRows = 5
	ignoreBackend := tidb.NewTiDBBackend(ctx, s.dbHandle, cfg, errormanager.New(nil, cfg, logger))
	encBuilder := tidb.NewEncodingBuilder()
	encoder, err := encBuilder.NewEncoder(context.Background(), &encode.EncodingConfig{
		Path:   "1.csv",
		Table:  s.tbl,
		Logger: log.L(),
	})
	require.NoError(t, err)

	dataRows := encBuilder.MakeEmptyRows()
	dataChecksum := verification.MakeKVChecksum(0, 0, 0)
	indexRows := encBuilder.MakeEmptyRows()
	indexChecksum := verification.MakeKVChecksum(0, 0, 0)
	for i := int64(0); i < 19; i++ { // encode rows 1, 2, 4, 8, ..., 262144.
		row, err := encoder.Encode(
			[]types.Datum{types.NewIntDatum(1 << i)},
			i,
			[]int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
			8*i,
		)
		require.NoError(t, err)
		row.ClassifyAndAppend(&dataRows, &dataChecksum, &indexRows, &indexChecksum)
	}

	engine, err := backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)
	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.NoError(t, err)
}

// TestLogicalImportBatch tests that each INSERT statement is limited by both
// logical-import-batch-size and logical-import-batch-rows configurations. Here
// we ensure each INSERT statement has up to 5 rows *and* ~30 bytes of values.
// In this test, we enable the `logical-import-prep-stmt` option,
// to verify prepared once and executed multiple times.
func TestLogicalImportBatchPrepStmt(t *testing.T) {
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)

	query1 := "\\QINSERT INTO `foo`.`bar`(`a`) VALUES(?),(?),(?),(?),(?)\\E"
	query2 := "\\QINSERT INTO `foo`.`bar`(`a`) VALUES(?),(?),(?),(?)\\E"
	query3 := "\\QINSERT INTO `foo`.`bar`(`a`) VALUES(?)\\E"

	// Expect the query to be prepared
	stmt1 := s.mockDB.ExpectPrepare(query1)
	// Set the expectation for the query execution with specific arguments
	stmt1.ExpectExec().
		WithArgs(1, 2, 4, 8, 16).
		WillReturnResult(sqlmock.NewResult(1, 5)) // Simulate an insert with 5 rows affected
	stmt1.ExpectExec().
		//WithArgs(32, 64, 128, 256, 512).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 5)) // Simulate an insert with 5 rows affected
	// Expect the query to be prepared
	stmt2 := s.mockDB.ExpectPrepare(query2)
	// Set the expectation for the query execution with specific arguments
	stmt2.ExpectExec().
		WithArgs(1024, 2048, 4096, 8192).
		WillReturnResult(sqlmock.NewResult(1, 4)) // Simulate an insert with 4 rows affected
	stmt2.ExpectExec().
		WithArgs(16384, 32768, 65536, 131072).
		WillReturnResult(sqlmock.NewResult(1, 4)) // Simulate an insert with 4 rows affected
	// Expect the query to be prepared
	stmt3 := s.mockDB.ExpectPrepare(query3)
	// Set the expectation for the query execution with specific arguments
	stmt3.ExpectExec().
		WithArgs(262144).
		WillReturnResult(sqlmock.NewResult(1, 1)) // Simulate an insert with 1 rows affected

	ctx := context.Background()
	logger := log.L()

	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.ErrorOnDup
	cfg.TikvImporter.LogicalImportBatchSize = 30
	cfg.TikvImporter.LogicalImportBatchRows = 5
	cfg.TikvImporter.LogicalImportPrepStmt = true
	ignoreBackend := tidb.NewTiDBBackend(ctx, s.dbHandle, cfg, errormanager.New(nil, cfg, logger))
	encBuilder := tidb.NewEncodingBuilder()
	// add logicalImportPrepStmt to the encoding config
	encoder, err := encBuilder.NewEncoder(context.Background(), &encode.EncodingConfig{
		SessionOptions: encode.SessionOptions{LogicalImportPrepStmt: true},
		Path:           "1.csv",
		Table:          s.tbl,
		Logger:         log.L(),
	})
	require.NoError(t, err)

	dataRows := encBuilder.MakeEmptyRows()
	dataChecksum := verification.MakeKVChecksum(0, 0, 0)
	indexRows := encBuilder.MakeEmptyRows()
	indexChecksum := verification.MakeKVChecksum(0, 0, 0)
	for i := int64(0); i < 19; i++ { // encode rows 1, 2, 4, 8, ..., 262144.
		row, err := encoder.Encode(
			[]types.Datum{types.NewIntDatum(1 << i)},
			i,
			[]int{0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
			8*i,
		)
		require.NoError(t, err)
		row.ClassifyAndAppend(&dataRows, &dataChecksum, &indexRows, &indexChecksum)
	}
	logger.Error("dataRows", zap.Any("dataRows", dataRows))

	engine, err := backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)
	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.NoError(t, err)
}

// TestWriteRowsRecordOneErrorPrepStmt tests that when LogicalImportPrepStmt is true and the batch insert fails,
// it will fallback to a single row insert,
// the error will be recorded in tidb_lightning_errors.conflict_records_v2.
func TestWriteRowsRecordOneErrorPrepStmt(t *testing.T) {
	dupErr := &gmysql.MySQLError{Number: errno.ErrDupEntry, Message: "Duplicate entry '2' for key 'PRIMARY'"}
	s := createMysqlSuite(t)
	defer s.TearDownTest(t)
	// First, batch insert, fail and rollback.
	query1 := "\\QINSERT INTO `foo`.`bar`(`a`) VALUES(?),(?),(?),(?),(?)\\E"
	query2 := "\\QINSERT INTO `foo`.`bar`(`a`) VALUES(?)\\E"
	// Expect any INSERT statement to be prepared
	stmt1 := s.mockDB.ExpectPrepare(query1)
	// Expect the batch query execution to fail
	stmt1.ExpectExec().
		WithArgs(1, 2, 3, 4, 5).
		WillReturnError(dupErr)
	// Expect single-row inserts
	stmt2 := s.mockDB.ExpectPrepare(query2)
	stmt2.ExpectExec().
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(1, 1))
	stmt2.ExpectExec().
		WithArgs(2).
		WillReturnError(dupErr)
	s.mockDB.
		ExpectExec("INSERT INTO `tidb_lightning_errors`\\.conflict_records_v2.*").
		WithArgs(sqlmock.AnyArg(), "`foo`.`bar`", "8.csv", int64(0), dupErr.Error(), 0, "(2)").
		WillReturnResult(driver.ResultNoRows)

	cfg := config.NewConfig()
	cfg.Conflict.Strategy = config.ErrorOnDup
	cfg.Conflict.Threshold = 0
	cfg.Conflict.MaxRecordRows = 0
	cfg.App.TaskInfoSchemaName = "tidb_lightning_errors"
	cfg.TikvImporter.LogicalImportPrepStmt = true
	ignoreBackend := tidb.NewTiDBBackend(context.Background(), s.dbHandle, cfg, errormanager.New(s.dbHandle, cfg, log.L()))
	encBuilder := tidb.NewEncodingBuilder()
	dataRows := encodeRowsTiDB(t, encBuilder, s.tbl)
	ctx := context.Background()
	engine, err := backend.MakeEngineManager(ignoreBackend).OpenEngine(ctx, &backend.EngineConfig{}, "`foo`.`bar`", 1)
	require.NoError(t, err)
	writerCfg := &backend.LocalWriterConfig{}
	writerCfg.TiDB.TableName = "`foo`.`bar`"
	writer, err := engine.LocalWriter(ctx, writerCfg)
	require.NoError(t, err)
	err = writer.AppendRows(ctx, []string{"a"}, dataRows)
	require.ErrorContains(t, err, "Duplicate entry '2' for key 'PRIMARY'")
	st, err := writer.Close(ctx)
	require.NoError(t, err)
	require.Nil(t, st)
}
