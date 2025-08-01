// Copyright 2017 PingCAP, Inc.
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

package expression_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/pingcap/tidb/pkg/parser"
	"github.com/pingcap/tidb/pkg/parser/charset"
	"github.com/pingcap/tidb/pkg/parser/mysql"
	plannercore "github.com/pingcap/tidb/pkg/planner/core"
	"github.com/pingcap/tidb/pkg/planner/core/resolve"
	"github.com/pingcap/tidb/pkg/session"
	"github.com/pingcap/tidb/pkg/sessionctx/vardef"
	"github.com/pingcap/tidb/pkg/sessiontxn"
	"github.com/pingcap/tidb/pkg/testkit"
	"github.com/pingcap/tidb/pkg/types"
	"github.com/pingcap/tidb/pkg/util/printer"
	"github.com/stretchr/testify/require"
)

func TestInferType(t *testing.T) {
	store := testkit.CreateMockStore(t)

	s := InferTypeSuite{}
	se, err := session.CreateSession4Test(store)
	require.NoError(t, err)
	testKit := testkit.NewTestKit(t, store)
	testKit.MustExec("use test")
	testKit.MustExec("drop table if exists t")
	sql := `create table t (
		c_bit bit(10),
		c_int_d int,
		c_uint_d int unsigned,
		c_bigint_d bigint,
		c_ubigint_d bigint unsigned,
		c_float_d float,
		c_ufloat_d float unsigned,
		c_double_d double,
		c_udouble_d double unsigned,
		c_decimal decimal(6, 3),
		c_udecimal decimal(10, 3) unsigned,
		c_decimal_d decimal,
		c_udecimal_d decimal unsigned,
		c_datetime datetime(2),
		c_datetime_d datetime,
		c_time time(3),
		c_time_d time,
		c_date date,
		c_timestamp timestamp(4) DEFAULT CURRENT_TIMESTAMP(4),
		c_timestamp_d timestamp DEFAULT CURRENT_TIMESTAMP,
		c_char char(20),
		c_bchar char(20) binary,
		c_varchar varchar(20),
		c_bvarchar varchar(20) binary,
		c_text_d text,
		c_btext_d text binary,
		c_binary binary(20),
		c_varbinary varbinary(20),
		c_blob_d blob,
		c_set set('a', 'b', 'c'),
		c_enum enum('a', 'b', 'c'),
		c_json JSON,
		c_year year,
		c_int_not_null int not null
	)`
	testKit.MustExec(sql)
	testKit.MustExec(`set tidb_enable_noop_functions=1;`)
	testKit.MustExec("set @a=cast('2000-01-01' as date);")
	testKit.MustExec("set @b=cast('2000-01-01 01:02:03.1234' as datetime);")
	testKit.MustExec("set @c=cast('2000-01-01 01:02:03' as datetime);")
	testKit.MustExec("set @d=cast('2000-01-01 01:02:03.1234' as datetime(4));")
	testKit.MustExec("set @e=cast('2000-01-01 01:02:03' as datetime(3));")

	var tests []typeInferTestCase
	tests = append(tests, s.createTestCase4Constants()...)
	tests = append(tests, s.createTestCase4Cast()...)
	tests = append(tests, s.createTestCase4Columns()...)
	tests = append(tests, s.createTestCase4StrFuncs()...)
	tests = append(tests, s.createTestCase4MathFuncs()...)
	tests = append(tests, s.createTestCase4ArithmeticFuncs()...)
	tests = append(tests, s.createTestCase4LogicalFuncs()...)
	tests = append(tests, s.createTestCase4ControlFuncs()...)
	tests = append(tests, s.createTestCase4Aggregations()...)
	tests = append(tests, s.createTestCase4InfoFunc()...)
	tests = append(tests, s.createTestCase4EncryptionFuncs()...)
	tests = append(tests, s.createTestCase4CompareFuncs()...)
	tests = append(tests, s.createTestCase4Miscellaneous()...)
	tests = append(tests, s.createTestCase4OpFuncs()...)
	tests = append(tests, s.createTestCase4OtherFuncs()...)
	tests = append(tests, s.createTestCase4TimeFuncs()...)
	tests = append(tests, s.createTestCase4DateAddSubFuncs()...)
	tests = append(tests, s.createTestCase4LikeFuncs()...)
	tests = append(tests, s.createTestCase4Literals()...)
	tests = append(tests, s.createTestCase4JSONFuncs()...)
	tests = append(tests, s.createTestCase4MiscellaneousFunc()...)
	tests = append(tests, s.createTestCase4GetVarFunc()...)

	sctx := testKit.Session()
	require.NoError(t, sctx.GetSessionVars().SetSystemVar(vardef.CharacterSetConnection, mysql.DefaultCharset))
	require.NoError(t, sctx.GetSessionVars().SetSystemVar(vardef.CollationConnection, mysql.DefaultCollationName))

	ctx := context.Background()
	par := parser.New()
	for _, tt := range tests {
		sql := "select " + tt.sql + " from t"
		comment := fmt.Sprintf("for %s", sql)
		stmt, err := par.ParseOneStmt(sql, "", "")
		require.NoError(t, err, comment)

		err = sessiontxn.NewTxn(context.Background(), se)
		require.NoError(t, err)

		ret := &plannercore.PreprocessorReturn{}
		nodeW := resolve.NewNodeW(stmt)
		err = plannercore.Preprocess(context.Background(), sctx, nodeW, plannercore.WithPreprocessorReturn(ret))
		require.NoError(t, err, comment)
		p, err := plannercore.BuildLogicalPlanForTest(ctx, sctx, nodeW, ret.InfoSchema)
		require.NoError(t, err, comment)
		tp := p.Schema().Columns[0].RetType
		require.Equal(t, tt.tp, tp.GetType(), comment)
		require.Equal(t, tt.chs, tp.GetCharset(), comment)
		require.Equal(t, tt.flag, tp.GetFlag(), comment)
		require.Equal(t, tt.flen, tp.GetFlen(), comment)
		require.Equal(t, tt.decimal, tp.GetDecimal(), comment)
	}
}

type typeInferTestCase struct {
	sql     string
	tp      byte
	chs     string
	flag    uint
	flen    int
	decimal int
}

type InferTypeSuite struct{}

func (s *InferTypeSuite) createTestCase4Constants() []typeInferTestCase {
	return []typeInferTestCase{
		{"1", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 1, 0},
		{"-1", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 2, 0},
		{"1.23", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 5, 2},
		{"-1.23", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 6, 2},
		{"123e5", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 8, types.UnspecifiedLength},
		{"-123e5", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 9, types.UnspecifiedLength},
		{"123e-5", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 7, types.UnspecifiedLength},
		{"-123e-5", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 8, types.UnspecifiedLength},
		{"NULL", mysql.TypeNull, charset.CharsetBin, mysql.BinaryFlag, 0, 0},
		{"TRUE", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag | mysql.NotNullFlag, 1, 0},
		{"FALSE", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag | mysql.NotNullFlag, 1, 0},
		{"'1234'", mysql.TypeVarString, charset.CharsetUTF8MB4, 0 | mysql.NotNullFlag, 4, types.UnspecifiedLength},
		{"_utf8'1234'", mysql.TypeVarString, charset.CharsetUTF8, 0 | mysql.NotNullFlag | mysql.UnderScoreCharsetFlag, 4, types.UnspecifiedLength},
		{"_binary'1234'", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag | mysql.UnderScoreCharsetFlag, 4, types.UnspecifiedLength},
		{"b'0001'", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 3, 0},
		{"b'000100001'", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 6, 0},
		{"b'0000000000010000'", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 6, 0},
		{"x'10'", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag | mysql.NotNullFlag, 3, 0},
		{"x'ff10'", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag | mysql.NotNullFlag, 6, 0},
		{"x'0000000000000000ff10'", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag | mysql.NotNullFlag, 30, 0},
	}
}

func (s *InferTypeSuite) createTestCase4Cast() []typeInferTestCase {
	return []typeInferTestCase{
		{"CAST(c_int_d AS BINARY)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 11, -1},
		{"CAST(c_int_d AS BINARY(5))", mysql.TypeString, charset.CharsetBin, mysql.BinaryFlag, 5, -1},
		{"CAST(c_int_d AS CHAR)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 11, -1},
		{"CAST(c_int_d AS CHAR(5))", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 5, -1},
		{"CAST(c_int_d AS DATE)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_int_d AS DATETIME)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"CAST(c_int_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_int_d AS DECIMAL(10))", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_int_d AS DECIMAL(10,3))", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 3}, // TODO: flen should be 12
		{"CAST(c_int_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_bit AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_uint_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_bigint_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_ubigint_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_float_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_ufloat_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_double_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_udouble_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_datetime AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_datetime_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_time AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_time_d AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_char AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_bchar AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_varchar AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(\"123456789.0123456789\" AS DECIMAL)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag, 10, 0},          // TODO: flen should be 11.
		{"CAST(\"123456789.0123456789\" AS DECIMAL(10,0))", mysql.TypeNewDecimal, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag, 10, 0},    // TODO: flen should be 11.
		{"CAST(\"123456789.0123456789\" AS DECIMAL(5,5))", mysql.TypeNewDecimal, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag, 5, 5},      // TODO: flen should be 7.
		{"CAST(\"123456789.0123456789\" AS DECIMAL(6,5))", mysql.TypeNewDecimal, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag, 6, 5},      // TODO: flen should be 8.
		{"CAST(\"-123456789.0123456789\" AS DECIMAL(64,30))", mysql.TypeNewDecimal, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag, 64, 30}, // TODO: flen should be 66.
		{"CAST(\"-123456789.0123456789\" AS DECIMAL(10,0))", mysql.TypeNewDecimal, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag, 10, 0},   // TODO: flen should be 11.
		{"CAST(\"-123456789.0123456789\" AS DECIMAL(5,5))", mysql.TypeNewDecimal, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag, 5, 5},     // TODO: flen should be 7.
		{"CAST(c_int_d AS JSON)", mysql.TypeJSON, charset.CharsetUTF8MB4, mysql.BinaryFlag | mysql.ParseToJSONFlag, 12582912 / 3, 0},
		{"CAST(c_int_d AS SIGNED)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 22, 0},         // TODO: flen should be 11.
		{"CAST(c_int_d AS SIGNED INTEGER)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 22, 0}, // TODO: flen should be 11.
		{"CAST(c_int_d AS TIME)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"CAST(c_int_d AS UNSIGNED)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 22, 0},         // TODO: flen should be 11.
		{"CAST(c_int_d AS UNSIGNED INTEGER)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 22, 0}, // TODO: flen should be 11.
		// We need to test WrapCast*, however this function is only called when function argument needs to be converted.
		{"c_int_not_null + 1.1", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 12, 1},
		{"c_int_not_null + 1.1E0", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, -1, -1},
	}
}

func (s *InferTypeSuite) createTestCase4Columns() []typeInferTestCase {
	return []typeInferTestCase{
		{"c_bit        ", mysql.TypeBit, charset.CharsetBin, mysql.UnsignedFlag, 10, 0},
		{"c_year       ", mysql.TypeYear, charset.CharsetBin, mysql.UnsignedFlag | mysql.ZerofillFlag, 4, 0},
		{"c_int_d      ", mysql.TypeLong, charset.CharsetBin, 0, 11, 0},
		{"c_uint_d     ", mysql.TypeLong, charset.CharsetBin, mysql.UnsignedFlag, 10, 0},
		{"c_bigint_d   ", mysql.TypeLonglong, charset.CharsetBin, 0, 20, 0},
		{"c_ubigint_d  ", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag, 20, 0},
		{"c_float_d    ", mysql.TypeFloat, charset.CharsetBin, 0, 12, types.UnspecifiedLength},
		{"c_ufloat_d   ", mysql.TypeFloat, charset.CharsetBin, mysql.UnsignedFlag, 12, types.UnspecifiedLength},
		{"c_double_d   ", mysql.TypeDouble, charset.CharsetBin, 0, 22, types.UnspecifiedLength},
		{"c_udouble_d  ", mysql.TypeDouble, charset.CharsetBin, mysql.UnsignedFlag, 22, types.UnspecifiedLength},
		{"c_decimal    ", mysql.TypeNewDecimal, charset.CharsetBin, 0, 6, 3},                   // TODO: flen should be 8
		{"c_udecimal   ", mysql.TypeNewDecimal, charset.CharsetBin, mysql.UnsignedFlag, 10, 3}, // TODO: flen should be 11
		{"c_decimal_d  ", mysql.TypeNewDecimal, charset.CharsetBin, 0, 10, 0},
		{"c_udecimal_d ", mysql.TypeNewDecimal, charset.CharsetBin, mysql.UnsignedFlag, 10, 0},
		{"c_datetime   ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"c_datetime_d ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"c_time       ", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"c_time_d     ", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"c_timestamp  ", mysql.TypeTimestamp, charset.CharsetBin, mysql.BinaryFlag, 24, 4},
		{"c_timestamp_d", mysql.TypeTimestamp, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"c_char       ", mysql.TypeString, charset.CharsetUTF8MB4, 0, 20, 0},
		{"c_bchar      ", mysql.TypeString, charset.CharsetUTF8MB4, 0, 20, 0},
		{"c_varchar    ", mysql.TypeVarchar, charset.CharsetUTF8MB4, 0, 20, 0}, // TODO: tp should be TypeVarString
		{"c_bvarchar   ", mysql.TypeVarchar, charset.CharsetUTF8MB4, 0, 20, 0}, // TODO: tp should be TypeVarString
		{"c_text_d     ", mysql.TypeBlob, charset.CharsetUTF8MB4, 0, 65535, 0}, // TODO: BlobFlag
		{"c_btext_d    ", mysql.TypeBlob, charset.CharsetUTF8MB4, 0, 65535, 0}, // TODO: BlobFlag
		{"c_binary     ", mysql.TypeString, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"c_varbinary  ", mysql.TypeVarchar, charset.CharsetBin, mysql.BinaryFlag, 20, 0}, // TODO: tp should be TypeVarString
		{"c_blob_d     ", mysql.TypeBlob, charset.CharsetBin, mysql.BinaryFlag, 65535, 0}, // TODO: BlobFlag
		{"c_set        ", mysql.TypeSet, charset.CharsetUTF8MB4, 0, 5, 0},                 // TODO: SetFlag
		{"c_enum       ", mysql.TypeEnum, charset.CharsetUTF8MB4, 0, 1, 0},                // TODO: EnumFlag
	}
}

func (s *InferTypeSuite) createTestCase4StrFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"strcmp(c_char, c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"space(c_int_d)", mysql.TypeLongBlob, mysql.DefaultCharset, 0, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"CONCAT(c_binary, c_int_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 40, types.UnspecifiedLength},
		{"CONCAT(c_bchar, c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"CONCAT(c_bchar, 0x80)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 23, types.UnspecifiedLength},
		{"CONCAT('T', 'i', 'DB')", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 4, types.UnspecifiedLength},
		{"CONCAT('T', 'i', 'DB', c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 24, types.UnspecifiedLength},
		{"CONCAT_WS('-', 'T', 'i', 'DB')", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 6, types.UnspecifiedLength},
		{"CONCAT_WS(',', 'TiDB', c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 25, types.UnspecifiedLength},
		{"CONCAT(c_bchar, 0x80)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 23, types.UnspecifiedLength},
		{"left(c_int_d, c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"right(c_int_d, c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"lower(c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"lower(c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"upper(c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"upper(c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"replace(1234, 2, 55)", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 20, types.UnspecifiedLength},
		{"replace(c_binary, 1, 2)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"to_base64(c_binary)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 28, types.UnspecifiedLength},
		{"substr(c_int_d, c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"substr(c_binary, c_int_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"uuid()", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 36, types.UnspecifiedLength},
		{"bit_length(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"substring_index(c_int_d, '.', 1)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"substring_index(c_binary, '.', 1)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"hex(c_char)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 160, types.UnspecifiedLength},
		{"hex(c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 22, types.UnspecifiedLength},
		{"unhex(c_int_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 6, types.UnspecifiedLength},
		{"unhex(c_char)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 40, types.UnspecifiedLength},
		{"ltrim(c_char)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"ltrim(c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"rtrim(c_char)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"rtrim(c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"trim(c_char)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"trim(c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"ascii(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"ord(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{`c_int_d like 'abc%'`, mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"tidb_version()", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, len(printer.GetTiDBInfo()), types.UnspecifiedLength},
		{"tidb_is_ddl_owner()", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, mysql.MaxIntWidth, 0},
		{"password(c_char)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.PWDHashLen + 1, types.UnspecifiedLength},
		{"elt(c_int_d, c_char, c_char, c_char)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"elt(c_int_d, c_char, c_char, c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"elt(c_int_d, c_char, c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"elt(c_int_d, c_char, c_double_d, c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 370, types.UnspecifiedLength},
		{"elt(c_int_d, c_char, c_double_d, c_int_d, c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 370, types.UnspecifiedLength},

		{"locate(c_char, c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"locate(c_binary, c_binary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"locate(c_char, c_binary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"locate(c_binary, c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"locate(c_char, c_char, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"locate(c_char, c_binary, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"locate(c_binary, c_char, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"locate(c_binary, c_binary, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},

		{"lpad('TiDB',   12,      'go'    )", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 48, types.UnspecifiedLength},
		{"lpad(c_binary, 12,      'go'    )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 12, types.UnspecifiedLength},
		{"lpad(c_char,   c_int_d, c_binary)", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"lpad(c_char,   c_int_d, c_char  )", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"rpad('TiDB',   12,      'go'    )", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 48, types.UnspecifiedLength},
		{"rpad(c_binary, 12,      'go'    )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 12, types.UnspecifiedLength},
		{"rpad(c_char,   c_int_d, c_binary)", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"rpad(c_char,   c_int_d, c_char  )", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, mysql.MaxBlobWidth, types.UnspecifiedLength},

		{"from_base64(c_int_d      )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 60, types.UnspecifiedLength},
		{"from_base64(c_bigint_d   )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 60, types.UnspecifiedLength},
		// the length of the result is greater than MySQL
		{"from_base64(c_float_d    )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 261, types.UnspecifiedLength},
		// the length of the result is greater than MySQL
		{"from_base64(c_double_d   )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 1110, types.UnspecifiedLength},
		{"from_base64(c_decimal    )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 27, types.UnspecifiedLength},
		{"from_base64(c_datetime   )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 66, types.UnspecifiedLength},
		{"from_base64(c_time_d     )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 30, types.UnspecifiedLength},
		{"from_base64(c_timestamp_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 57, types.UnspecifiedLength},
		{"from_base64(c_char       )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 60, types.UnspecifiedLength},
		{"from_base64(c_varchar    )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 60, types.UnspecifiedLength},
		{"from_base64(c_text_d     )", mysql.TypeMediumBlob, charset.CharsetBin, mysql.BinaryFlag, 196605, types.UnspecifiedLength},
		{"from_base64(c_binary     )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 60, types.UnspecifiedLength},
		{"from_base64(c_varbinary  )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 60, types.UnspecifiedLength},
		{"from_base64(c_blob_d     )", mysql.TypeMediumBlob, charset.CharsetBin, mysql.BinaryFlag, 196605, types.UnspecifiedLength},
		{"from_base64(c_set        )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 15, types.UnspecifiedLength},
		{"from_base64(c_enum       )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 3, types.UnspecifiedLength},

		{"bin(c_int_d      )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_bigint_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_float_d    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_double_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_decimal    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_datetime   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_time_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_char       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_varchar    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_text_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_binary     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_varbinary  )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_blob_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_set        )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"bin(c_enum       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},

		{"char_length(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_float_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_time_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_varchar)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_binary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_varbinary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_blob_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_set)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"char_length(c_enum)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},

		{"character_length(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_float_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_time_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_varchar)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_binary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_varbinary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_blob_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_set)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"character_length(c_enum)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},

		{"char(c_int_d      )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_bigint_d   )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_float_d    )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_double_d   )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_decimal    )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_datetime   )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_time_d     )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_timestamp_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_char       )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_varchar    )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_text_d     )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_binary     )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_varbinary  )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_blob_d     )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_set        )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_enum       )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 4, types.UnspecifiedLength},
		{"char(c_int_d      , c_int_d       using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_bigint_d   , c_bigint_d    using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_float_d    , c_float_d     using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_double_d   , c_double_d    using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_decimal    , c_decimal     using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_datetime   , c_datetime    using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_time_d     , c_time_d      using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_timestamp_d, c_timestamp_d using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_char       , c_char        using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_varchar    , c_varchar     using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_text_d     , c_text_d      using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_binary     , c_binary      using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_varbinary  , c_varbinary   using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_blob_d     , c_blob_d      using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_set        , c_set         using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},
		{"char(c_enum       , c_enum        using utf8)", mysql.TypeVarString, charset.CharsetUTF8, 0, 8, types.UnspecifiedLength},

		{"instr(c_char, c_binary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"instr(c_char, c_char    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"instr(c_char, c_time_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},

		{"reverse(c_int_d      )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"reverse(c_bigint_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"reverse(c_float_d    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 87, types.UnspecifiedLength},
		{"reverse(c_double_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 370, types.UnspecifiedLength},
		{"reverse(c_decimal    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 9, types.UnspecifiedLength},
		{"reverse(c_char       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"reverse(c_varchar    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"reverse(c_text_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 65535, types.UnspecifiedLength},
		{"reverse(c_binary     )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"reverse(c_varbinary  )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"reverse(c_blob_d     )", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 65535, types.UnspecifiedLength},
		{"reverse(c_set        )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 5, types.UnspecifiedLength},
		{"reverse(c_enum       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 1, types.UnspecifiedLength},

		{"oct(c_int_d      )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_bigint_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_float_d    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_double_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_decimal    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_datetime   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_time_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_char       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_varchar    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_text_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_binary     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_varbinary  )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_blob_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_set        )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"oct(c_enum       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},

		{"find_in_set(c_int_d      , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_bigint_d   , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_float_d    , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_double_d   , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_decimal    , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_datetime   , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_time_d     , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_timestamp_d, c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_char       , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_varchar    , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_text_d     , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_binary     , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_varbinary  , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_blob_d     , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_set        , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"find_in_set(c_enum       , c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},

		{"make_set(c_int_d      , c_text_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 65535, types.UnspecifiedLength},
		{"make_set(c_bigint_d   , c_text_d, c_binary)", mysql.TypeMediumBlob, charset.CharsetBin, mysql.BinaryFlag, 65556, types.UnspecifiedLength},
		{"make_set(1   , c_text_d, 0x40)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 65535, types.UnspecifiedLength},

		{"quote(c_int_d      )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 42, types.UnspecifiedLength},
		{"quote(c_bigint_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 42, types.UnspecifiedLength},
		{"quote(c_float_d    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 176, types.UnspecifiedLength},
		{"quote(c_double_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 742, types.UnspecifiedLength},

		{"convert(c_double_d using utf8mb4)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"convert(c_binary using utf8mb4)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"convert(c_binary using 'binary')", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"convert(c_text_d using 'binary')", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},

		{"insert(c_varchar, c_int_d, c_int_d, c_varchar)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"insert(c_varchar, c_int_d, c_int_d, 0x40)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"insert(c_varchar, c_int_d, c_int_d, c_binary)", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"insert(c_binary, c_int_d, c_int_d, c_varchar)", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"insert(c_binary, c_int_d, c_int_d, c_binary)", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},

		{"export_set(c_double_d, c_text_d, c_text_d)", mysql.TypeMediumBlob, charset.CharsetUTF8MB4, 0, 16777212, types.UnspecifiedLength},
		{"export_set(c_double_d, c_text_d, c_text_d, c_text_d)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, 33291780, types.UnspecifiedLength},
		{"export_set(c_double_d, c_text_d, c_text_d, c_text_d, c_int_d)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, 33291780, types.UnspecifiedLength},

		{"format(c_double_d, c_double_d)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"format(c_double_d, c_double_d, c_binary)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, mysql.MaxBlobWidth, types.UnspecifiedLength},

		{"field(c_double_d, c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
	}
}

func (s *InferTypeSuite) createTestCase4MathFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"cos(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sin(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"tan(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"exp(c_int_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"exp(c_float_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"exp(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"exp(c_decimal)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"exp(c_datetime)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"exp(c_time_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"exp(c_timestamp_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"exp(c_binary)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"pi()", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 8, 6},
		{"~c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"!c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_int_d & c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"c_int_d | c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"c_int_d ^ c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"c_int_d << c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"c_int_d >> c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"log2(c_int_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"log10(c_int_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"log(c_int_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"log(2, c_int_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"degrees(c_int_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"atan(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"atan(c_double_d,c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"asin(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"acos(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},

		{"cot(c_int_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"cot(c_float_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"cot(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"cot(c_decimal)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"cot(c_datetime)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"cot(c_time_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"cot(c_timestamp_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"cot(c_binary)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},

		{"floor(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"floor(c_uint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"floor(c_bigint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"floor(c_ubigint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"floor(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"floor(c_udecimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"floor(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(c_udouble_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(c_float_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(c_ufloat_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(c_datetime)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(c_timestamp_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(c_time_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(c_enum)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(c_text_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(18446744073709551615)", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"floor(18446744073709551615.1)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 23, 0},

		{"ceil(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"ceil(c_uint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"ceil(c_bigint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"ceil(c_ubigint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"ceil(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"floor(c_udecimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"ceil(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(c_udouble_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceil(c_float_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"floor(c_ufloat_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceil(c_datetime)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceil(c_timestamp_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceil(c_time_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceil(c_enum)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceil(c_text_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceil(18446744073709551615)", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"ceil(18446744073709551615.1)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 23, 0},

		{"ceiling(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"ceiling(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"ceiling(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceiling(c_float_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceiling(c_datetime)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceiling(c_time_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceiling(c_enum)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceiling(c_text_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"ceiling(18446744073709551615)", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"ceiling(18446744073709551615.1)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 23, 0},

		{"conv(c_char, c_int_d, c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"conv(c_int_d, c_int_d, c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},

		{"abs(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"abs(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"abs(c_float_d    )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_double_d   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_decimal    )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 6, 3},
		{"abs(c_datetime   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_time_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_timestamp_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_char       )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_varchar    )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_text_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_binary     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_varbinary  )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_blob_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_set        )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"abs(c_enum       )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},

		{"round(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"round(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"round(c_float_d    )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_double_d   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_decimal    )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 6, 0}, // flen Should be 5.
		{"round(c_datetime   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_time_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_timestamp_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_char       )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_varchar    )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_text_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_binary     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_varbinary  )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_blob_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_set        )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"round(c_enum       )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},

		{"truncate(c_int_d,      1)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"truncate(c_int_d,     -5)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"truncate(c_int_d,    100)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"truncate(c_double_d,   1)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"truncate(c_double_d,   5)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"truncate(c_double_d, 100)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},

		{"rand(       )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"rand(c_int_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},

		{"pow(c_int_d,   c_int_d   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"pow(c_float_d, c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"pow(c_int_d,   c_bigint_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},

		{"sign(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"sign(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},

		{"sqrt(c_int_d      )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_bigint_d   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_float_d    )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_double_d   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_decimal    )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_datetime   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_time_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_timestamp_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_char       )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_varchar    )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_text_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_binary     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_varbinary  )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_blob_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_set        )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sqrt(c_enum       )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},

		{"CRC32(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"CRC32(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},

		{"radians(c_int_d      )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_bigint_d   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_float_d    )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength}, // Should be 17.
		{"radians(c_double_d   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength}, // Should be 17.
		{"radians(c_decimal    )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength}, // Should be 5.
		{"radians(c_datetime   )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_time_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_timestamp_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_char       )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_varchar    )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_text_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_binary     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_varbinary  )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_blob_d     )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_set        )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"radians(c_enum       )", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
	}
}

func (s *InferTypeSuite) createTestCase4ArithmeticFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"c_int_d + c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d + c_bigint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d + c_char", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_int_d + c_time_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d + c_double_d", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_int_d + c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"c_datetime + c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 24, 3},
		{"c_bigint_d + c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 24, 3},
		{"c_double_d + c_decimal", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_double_d + c_char", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_double_d + c_enum", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},

		{"c_int_d - c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d - c_bigint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d - c_char", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_int_d - c_time_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d - c_double_d", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_int_d - c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"c_datetime - c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 24, 3},
		{"c_bigint_d - c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 24, 3},
		{"c_double_d - c_decimal", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_double_d - c_char", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_double_d - c_enum", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},

		{"c_int_d * c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d * c_bigint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d * c_char", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_int_d * c_time_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d * c_double_d", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_int_d * c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 16, 3},
		{"c_datetime * c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 28, 5},
		{"c_bigint_d * c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 26, 3},
		{"c_double_d * c_decimal", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_double_d * c_char", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},
		{"c_double_d * c_enum", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, types.UnspecifiedLength, types.UnspecifiedLength},

		{"c_int_d / c_int_d", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 15, 4},
		{"c_int_d / c_bigint_d", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 15, 4},
		{"c_int_d / c_char", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"c_int_d / c_time_d", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 15, 4},
		{"c_int_d / c_double_d", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"c_int_d / c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 4},
		{"c_datetime / c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 29, 6}, // TODO: flen should be 25.
		{"c_bigint_d / c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 28, 4}, // TODO: flen should be 28.
		{"c_double_d / c_decimal", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"c_double_d / c_char", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"c_double_d / c_enum", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"2/3", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 6, 4},
		{"-2/3", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 6, 4},

		{"c_int_d DIV c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_uint_d DIV c_uint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d DIV c_bigint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_ubigint_d DIV c_ubigint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_ubigint_d DIV c_bigint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d DIV c_uint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_ubigint_d DIV c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d DIV c_char", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d DIV c_time_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d DIV c_double_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d DIV c_udouble_d", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d DIV c_decimal", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d DIV c_udecimal", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_decimal DIV c_udecimal", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_datetime DIV c_decimal", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_bigint_d DIV c_decimal", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_double_d DIV c_decimal", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_double_d DIV c_char", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_double_d DIV c_enum", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},

		{"c_int_d MOD c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_uint_d MOD c_uint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d MOD c_bigint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_ubigint_d MOD c_ubigint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_ubigint_d MOD c_bigint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d MOD c_uint_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_ubigint_d MOD c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d MOD c_char", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d MOD c_time_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"c_int_d MOD c_double_d", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"c_int_d MOD c_udouble_d", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"c_int_d MOD c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 11, 3},
		{"c_udecimal MOD c_int_d", mysql.TypeNewDecimal, charset.CharsetBin, mysql.UnsignedFlag | mysql.BinaryFlag, 11, 3},
		{"c_decimal MOD c_udecimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 3},
		{"c_datetime MOD c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 22, 3},
		{"c_bigint_d MOD c_decimal", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 20, 3},
		{"c_double_d MOD c_decimal", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"c_double_d MOD c_char", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"c_double_d MOD c_enum", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
	}
}

func (s *InferTypeSuite) createTestCase4LogicalFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"c_int_d and c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_int_d xor c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},

		{"c_int_d && c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_int_d || c_int_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
	}
}

func (s *InferTypeSuite) createTestCase4ControlFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"ifnull(c_int_d, c_int_d)", mysql.TypeLong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"ifnull(c_int_d, c_decimal)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"ifnull(c_int_d, c_char)", mysql.TypeString, charset.CharsetUTF8MB4, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"ifnull(c_int_d, c_binary)", mysql.TypeString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"ifnull(c_char, c_binary)", mysql.TypeString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"ifnull(null, null)", mysql.TypeNull, charset.CharsetBin, mysql.BinaryFlag, 0, 0},
		{"ifnull(c_double_d, c_timestamp_d)", mysql.TypeVarchar, charset.CharsetUTF8MB4, 0, 22, types.UnspecifiedLength},
		{"ifnull(c_json, c_decimal)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, math.MaxUint32, types.UnspecifiedLength},
		{"ifnull(c_time, c_date)", mysql.TypeDatetime, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp + 3 + 1, 3},
		{"ifnull(c_time_d, c_date)", mysql.TypeDatetime, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp, 0},
		{"if(c_int_d, c_decimal, c_int_d)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"if(c_int_d, c_char, c_int_d)", mysql.TypeString, charset.CharsetUTF8MB4, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"if(c_int_d, c_binary, c_int_d)", mysql.TypeString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"if(c_int_d, c_bchar, c_int_d)", mysql.TypeString, charset.CharsetUTF8MB4, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"if(c_int_d, c_char, c_decimal)", mysql.TypeString, charset.CharsetUTF8MB4, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"if(c_int_d, c_datetime, c_int_d)", mysql.TypeVarchar, charset.CharsetUTF8MB4, 0, 22, types.UnspecifiedLength},
		{"if(c_int_d, c_int_d, c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"if(c_int_d, c_time_d, c_datetime)", mysql.TypeDatetime, charset.CharsetUTF8MB4, mysql.BinaryFlag, 22, 2}, // TODO: should not be BinaryFlag
		{"if(c_int_d, c_time, c_json)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, math.MaxUint32, types.UnspecifiedLength},
		{"if(null, null, null)", mysql.TypeNull, charset.CharsetBin, mysql.BinaryFlag, 0, 0},
		{"case when c_int_d then c_char else c_varchar end", mysql.TypeVarchar, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"case when c_int_d > 1 then c_double_d else c_bchar end", mysql.TypeString, charset.CharsetUTF8MB4, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"case when c_int_d > 2 then c_double_d when c_int_d < 1 then c_decimal else c_double_d end", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"case when c_double_d > 2 then c_decimal else 1 end", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 6, 3},
		{"case when c_time is not null then c_time else c_date end", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp + 3 + 1, 3},
		{"case when c_time_d is not null then c_time_d else c_date end", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, 0},
		{"case when null then null else null end", mysql.TypeNull, charset.CharsetBin, mysql.BinaryFlag, 0, types.UnspecifiedLength},
	}
}

func (s *InferTypeSuite) createTestCase4Aggregations() []typeInferTestCase {
	return []typeInferTestCase{
		{"sum(c_int_d)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 32, 0},
		{"sum(c_float_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sum(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sum(c_decimal)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 28, 3},
		{"sum(cast(c_decimal as decimal(65,3)))", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 65, 3},
		{"sum(1.0)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 26, 1},
		{"sum(1.2e2)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"sum(c_char)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"avg(c_int_d)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 15, 4},
		{"avg(c_float_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"avg(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"avg(c_decimal)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 10, 7},
		{"avg(cast(c_decimal as decimal(65,3)))", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 65, 7},
		{"avg(1.0)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 8, 5},
		{"avg(1.2e2)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"avg(c_char)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxRealWidth, types.UnspecifiedLength},
		{"group_concat(c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxBlobWidth, 0},
		{"count(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 21, 0},
	}
}

func (s *InferTypeSuite) createTestCase4InfoFunc() []typeInferTestCase {
	return []typeInferTestCase{
		{"last_insert_id(       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag | mysql.NotNullFlag, mysql.MaxIntWidth, 0},
		{"last_insert_id(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"found_rows()", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"database()", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 64, types.UnspecifiedLength},
		{"current_user()", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"current_role()", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 64, types.UnspecifiedLength},
		{"user()", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 64, types.UnspecifiedLength},
		{"connection_id()", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag | mysql.NotNullFlag, mysql.MaxIntWidth, 0},
		{"version()", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 64, types.UnspecifiedLength},
		{"current_resource_group()", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 64, types.UnspecifiedLength},
	}
}

func (s *InferTypeSuite) createTestCase4EncryptionFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"md5(c_int_d      )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_bigint_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_float_d    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_double_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_decimal    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_datetime   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_time_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_char       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_varchar    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_text_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_binary     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_varbinary  )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_blob_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_set        )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5(c_enum       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 32, types.UnspecifiedLength},
		{"md5('1234'       )", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 32, types.UnspecifiedLength},
		{"md5(1234         )", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 32, types.UnspecifiedLength},

		{"sha(c_int_d      )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_bigint_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_float_d    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_double_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_decimal    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_datetime   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_time_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_char       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_varchar    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_text_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_binary     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_varbinary  )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_blob_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_set        )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha(c_enum       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha('1234'       )", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 40, types.UnspecifiedLength},
		{"sha(1234         )", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 40, types.UnspecifiedLength},

		{"sha1(c_int_d      )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_bigint_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_float_d    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_double_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_decimal    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_datetime   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_time_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_char       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_varchar    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_text_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_binary     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_varbinary  )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_blob_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_set        )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1(c_enum       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sha1('1234'       )", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 40, types.UnspecifiedLength},
		{"sha1(1234         )", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 40, types.UnspecifiedLength},

		{"sha2(c_int_d      , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_bigint_d   , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_float_d    , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_double_d   , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_decimal    , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_datetime   , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_time_d     , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_timestamp_d, 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_char       , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_varchar    , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_text_d     , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_binary     , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_varbinary  , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_blob_d     , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_set        , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_enum       , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2('1234'       , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 128, types.UnspecifiedLength},
		{"sha2(1234         , 0)", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 128, types.UnspecifiedLength},

		{"sha2(c_int_d      , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_bigint_d   , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_float_d    , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_double_d   , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_decimal    , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_datetime   , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_time_d     , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_timestamp_d, '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_char       , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_varchar    , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_text_d     , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_binary     , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_varbinary  , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_blob_d     , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_set        , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2(c_enum       , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 128, types.UnspecifiedLength},
		{"sha2('1234'       , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 128, types.UnspecifiedLength},
		{"sha2(1234         , '256')", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 128, types.UnspecifiedLength},

		{"sm3(c_int_d      )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_bigint_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_float_d    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_double_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_decimal    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_datetime   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_time_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_char       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_varchar    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_text_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_binary     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_varbinary  )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_blob_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_set        )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3(c_enum       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 40, types.UnspecifiedLength},
		{"sm3('1234'       )", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 40, types.UnspecifiedLength},
		{"sm3(1234         )", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 40, types.UnspecifiedLength},

		{"AES_ENCRYPT(c_int_d, 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 32, types.UnspecifiedLength},
		{"AES_ENCRYPT(c_char, 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 32, types.UnspecifiedLength},
		{"AES_ENCRYPT(c_varchar, 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 32, types.UnspecifiedLength},
		{"AES_ENCRYPT(c_binary, 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 32, types.UnspecifiedLength},
		{"AES_ENCRYPT(c_varbinary, 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 32, types.UnspecifiedLength},
		{"AES_ENCRYPT('', 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 16, types.UnspecifiedLength},
		{"AES_ENCRYPT('111111', 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 16, types.UnspecifiedLength},
		{"AES_ENCRYPT('111111111111111', 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 16, types.UnspecifiedLength},
		{"AES_ENCRYPT('1111111111111111', 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 32, types.UnspecifiedLength},
		{"AES_ENCRYPT('11111111111111111', 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 32, types.UnspecifiedLength},
		{"AES_DECRYPT('1111111111111111', 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 16, types.UnspecifiedLength},
		{"AES_DECRYPT('11111111111111112222222222222222', 'key')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 32, types.UnspecifiedLength},

		{"COMPRESS(c_int_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 33, types.UnspecifiedLength},
		{"COMPRESS(c_char)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 33, types.UnspecifiedLength},
		{"COMPRESS(c_bchar)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 33, types.UnspecifiedLength},
		{"COMPRESS(c_varchar)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 33, types.UnspecifiedLength},
		{"COMPRESS(c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 33, types.UnspecifiedLength},
		{"COMPRESS(c_varbinary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 33, types.UnspecifiedLength},
		{"COMPRESS('')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 13, types.UnspecifiedLength},
		{"COMPRESS('abcde')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 18, types.UnspecifiedLength},

		{"UNCOMPRESS(c_int_d)", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"UNCOMPRESS(c_char)", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"UNCOMPRESS(c_bchar)", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"UNCOMPRESS(c_varchar)", mysql.TypeLongBlob, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxBlobWidth, types.UnspecifiedLength},
		{"UNCOMPRESSED_LENGTH(c_varchar)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"UNCOMPRESSED_LENGTH(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},

		{"RANDOM_BYTES(5)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 1024, types.UnspecifiedLength},
		{"RANDOM_BYTES('123')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 1024, types.UnspecifiedLength},
		{"RANDOM_BYTES('abc')", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 1024, types.UnspecifiedLength},
	}
}

func (s *InferTypeSuite) createTestCase4CompareFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"coalesce(c_int_d, c_int_d)", mysql.TypeLong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"coalesce(c_int_d, c_decimal)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"coalesce(c_int_d, c_char)", mysql.TypeString, charset.CharsetUTF8MB4, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"coalesce(c_int_d, c_binary)", mysql.TypeString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"coalesce(c_char, c_binary)", mysql.TypeString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"coalesce(null, null)", mysql.TypeNull, charset.CharsetBin, mysql.BinaryFlag, 0, 0},
		{"coalesce(c_double_d, c_timestamp_d)", mysql.TypeVarchar, charset.CharsetUTF8MB4, 0, 22, types.UnspecifiedLength},
		{"coalesce(c_json, c_decimal)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, 0, math.MaxUint32, types.UnspecifiedLength},
		{"coalesce(c_time, c_date)", mysql.TypeDatetime, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp + 3 + 1, 3},
		{"coalesce(c_time_d, c_date)", mysql.TypeDatetime, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp, 0},

		{"isnull(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"isnull(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.NotNullFlag | mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},

		{"nullif(c_int_d      , 123)", mysql.TypeLong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"nullif(c_bigint_d   , 123)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"nullif(c_float_d    , 123)", mysql.TypeFloat, charset.CharsetBin, mysql.BinaryFlag, 12, types.UnspecifiedLength},
		{"nullif(c_double_d   , 123)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"nullif(c_decimal    , 123)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 6, 3},
		{"nullif(c_datetime   , 123)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"nullif(c_time_d     , 123)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"nullif(c_timestamp_d, 123)", mysql.TypeTimestamp, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"nullif(c_char       , 123)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"nullif(c_varchar    , 123)", mysql.TypeVarchar, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},            // TODO: tp should be TypeVarString
		{"nullif(c_text_d     , 123)", mysql.TypeBlob, charset.CharsetUTF8MB4, 0, 65535, types.UnspecifiedLength},            // TODO: tp should be TypeMediumBlob
		{"nullif(c_binary     , 123)", mysql.TypeString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},  // TODO: tp should be TypeVarString
		{"nullif(c_varbinary  , 123)", mysql.TypeVarchar, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength}, // TODO: tp should be TypeVarString
		{"nullif(c_blob_d     , 123)", mysql.TypeBlob, charset.CharsetBin, mysql.BinaryFlag, 65535, types.UnspecifiedLength}, // TODO: tp should be TypeVarString

		{"interval(c_int_d, c_int_d, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"interval(c_int_d, c_float_d, c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},

		{"greatest(c_bigint_d, c_ubigint_d, c_int_d)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"greatest(c_ubigint_d, c_ubigint_d, c_uint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"greatest(c_uint_d, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"least(c_bigint_d, c_ubigint_d, c_int_d)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"least(c_ubigint_d, c_ubigint_d, c_uint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"least(c_uint_d, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},

		// bigint unsigned + int = Decimal. (compatible with mysql)
		{"greatest(c_ubigint_d, c_ubigint_d, c_int_d)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"least(c_ubigint_d, c_ubigint_d, c_int_d)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},

		// bigint unsgined + bigint = Decimal. (compatible with mysql)
		{"greatest(c_bigint_d, c_ubigint_d, c_int_d)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"least(c_bigint_d, c_ubigint_d, c_int_d)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},

		// xxxint unsgined + xxxint unsigned = bigint unsigned. (not compatible with mysql, mysql is int if xxxint unsigned is smallint/mediumint)
		{"greatest(c_ubigint_d, c_ubigint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},
		{"least(c_ubigint_d, c_ubigint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, mysql.MaxIntWidth, 0},

		{"greatest(c_uint_d, c_uint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},
		{"least(c_uint_d, c_uint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 10, 0},

		// xxxint unsgined + xxxint = bigint. (not compatible with mysql, mysql is int if xxxint is smallint/mediumint)
		{"greatest(c_int_d, c_uint_d, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"least(c_int_d, c_uint_d, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
	}
}

func (s *InferTypeSuite) createTestCase4Miscellaneous() []typeInferTestCase {
	return []typeInferTestCase{
		{"sleep(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 21, 0},
		{"sleep(c_float_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 21, 0},
		{"sleep(c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 21, 0},
		{"sleep(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 21, 0},
		{"sleep(c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 21, 0},
		{"sleep(c_time_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 21, 0},
		{"sleep(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 21, 0},
		{"sleep(c_binary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 21, 0},

		{"inet_aton(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 21, 0},
		{"inet_aton(c_float_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 21, 0},
		{"inet_aton(c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 21, 0},
		{"inet_aton(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 21, 0},
		{"inet_aton(c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 21, 0},
		{"inet_aton(c_time_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 21, 0},
		{"inet_aton(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 21, 0},
		{"inet_aton(c_binary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.UnsignedFlag, 21, 0},

		{"inet_ntoa(c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 93, 0},
		{"inet_ntoa(c_float_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 93, 0},
		{"inet_ntoa(c_double_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 93, 0},
		{"inet_ntoa(c_decimal)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 93, 0},
		{"inet_ntoa(c_datetime)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 93, 0},
		{"inet_ntoa(c_time_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 93, 0},
		{"inet_ntoa(c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 93, 0},
		{"inet_ntoa(c_binary)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 93, 0},

		{"inet6_aton(c_int_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 16, 0},
		{"inet6_aton(c_float_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 16, 0},
		{"inet6_aton(c_double_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 16, 0},
		{"inet6_aton(c_decimal)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 16, 0},
		{"inet6_aton(c_datetime)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 16, 0},
		{"inet6_aton(c_time_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 16, 0},
		{"inet6_aton(c_timestamp_d)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 16, 0},
		{"inet6_aton(c_binary)", mysql.TypeVarString, charset.CharsetBin, mysql.BinaryFlag, 16, 0},

		{"inet6_ntoa(c_int_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 117, 0},
		{"inet6_ntoa(c_float_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 117, 0},
		{"inet6_ntoa(c_double_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 117, 0},
		{"inet6_ntoa(c_decimal)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 117, 0},
		{"inet6_ntoa(c_datetime)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 117, 0},
		{"inet6_ntoa(c_time_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 117, 0},
		{"inet6_ntoa(c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 117, 0},
		{"inet6_ntoa(c_binary)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 117, 0},

		{"is_ipv4(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4(c_float_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4(c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4(c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4(c_time_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4(c_binary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},

		{"is_ipv4_compat(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_compat(c_float_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_compat(c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_compat(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_compat(c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_compat(c_time_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_compat(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_compat(c_binary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},

		{"is_ipv4_mapped(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_mapped(c_float_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_mapped(c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_mapped(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_mapped(c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_mapped(c_time_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_mapped(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv4_mapped(c_binary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},

		{"is_ipv6(c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv6(c_float_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv6(c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv6(c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv6(c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv6(c_time_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv6(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"is_ipv6(c_binary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},

		{"any_value(c_int_d)", mysql.TypeLong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"any_value(c_bigint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"any_value(c_float_d)", mysql.TypeFloat, charset.CharsetBin, mysql.BinaryFlag, 12, types.UnspecifiedLength},
		{"any_value(c_double_d)", mysql.TypeDouble, charset.CharsetBin, mysql.BinaryFlag, 22, types.UnspecifiedLength},
		{"any_value(c_decimal)", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 6, 3}, // TODO: flen should be 8.
		{"any_value(c_datetime)", mysql.TypeDatetime, charset.CharsetUTF8MB4, 0, 22, 2},
		{"any_value(c_time_d)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"any_value(c_timestamp_d)", mysql.TypeTimestamp, charset.CharsetUTF8MB4, 0, 19, 0},
		{"any_value(c_char)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"any_value(c_bchar)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"any_value(c_varchar)", mysql.TypeVarchar, charset.CharsetUTF8MB4, 0, 20, types.UnspecifiedLength},
		{"any_value(c_text_d)", mysql.TypeBlob, charset.CharsetUTF8MB4, 0, 65535, types.UnspecifiedLength},
		{"any_value(c_binary)", mysql.TypeString, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"any_value(c_varbinary)", mysql.TypeVarchar, charset.CharsetBin, mysql.BinaryFlag, 20, types.UnspecifiedLength},
		{"any_value(c_blob_d)", mysql.TypeBlob, charset.CharsetBin, mysql.BinaryFlag, 65535, types.UnspecifiedLength},
		{"any_value(c_set)", mysql.TypeSet, charset.CharsetUTF8MB4, 0, 5, types.UnspecifiedLength},
		{"any_value(c_enum)", mysql.TypeEnum, charset.CharsetUTF8MB4, 0, 1, types.UnspecifiedLength},
	}
}

func (s *InferTypeSuite) createTestCase4OpFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"c_int_d      is true", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_decimal    is true", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_double_d   is true", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_float_d    is true", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_datetime   is true", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_time_d     is true", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_enum       is true", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_text_d     is true", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"18446        is true", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag | mysql.NotNullFlag, 1, 0},
		{"1844674.1    is true", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag | mysql.NotNullFlag, 1, 0},

		{"c_int_d      is false", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_decimal    is false", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_double_d   is false", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_float_d    is false", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_datetime   is false", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_time_d     is false", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_enum       is false", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_text_d     is false", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"18446        is false", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag | mysql.NotNullFlag, 1, 0},
		{"1844674.1    is false", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag | mysql.NotNullFlag, 1, 0},
	}
}

func (s *InferTypeSuite) createTestCase4OtherFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"1 in (c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"1 in (c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"1 in (c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"1 in (c_float_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"1 in (c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"1 in (c_time_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"1 in (c_enum)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"1 in (c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},

		{"bit_count(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"bit_count(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},

		{`@varname`, mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxFieldVarCharLength, types.UnspecifiedFsp},
	}
}

func (s *InferTypeSuite) createTestCase4TimeFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{`time_format('150:02:28', '%r%r%r%r')`, mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 44, types.UnspecifiedLength},
		{`time_format(123456, '%r%r%r%r')`, mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 44, types.UnspecifiedLength},
		{`time_format('bad string', '%r%r%r%r')`, mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 44, types.UnspecifiedLength},
		{`time_format(null, '%r%r%r%r')`, mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 44, types.UnspecifiedLength},

		{`date_format(null, '%r%r%r%r')`, mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 44, types.UnspecifiedLength},
		{`date_format('2017-06-15', '%r%r%r%r')`, mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 44, types.UnspecifiedLength},
		{`date_format(151113102019.12, '%r%r%r%r')`, mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 44, types.UnspecifiedLength},

		{"timestampadd(HOUR, c_int_d, c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp, types.UnspecifiedLength},
		{"timestampadd(minute, c_double_d, c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp, types.UnspecifiedLength},
		{"timestampadd(SeconD, c_int_d, c_char)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp, types.UnspecifiedLength},
		{"timestampadd(SeconD, c_varchar, c_time_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp, types.UnspecifiedLength},
		{"timestampadd(SeconD, c_int_d, c_datetime)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp, types.UnspecifiedLength},
		{"timestampadd(SeconD, c_double_d, c_bchar)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp, types.UnspecifiedLength},
		{"timestampadd(SeconD, c_int_d, c_blob_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthNoFsp, types.UnspecifiedLength},
		{"timestampadd(microsecond, c_int_d, c_blob_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeWidthWithFsp, types.UnspecifiedLength},

		{"to_seconds(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"to_days(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},

		{"unix_timestamp(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"unix_timestamp(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"unix_timestamp(c_float_d    )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 6},
		{"unix_timestamp(c_double_d   )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 6},
		{"unix_timestamp(c_decimal    )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 15, 3},
		{"unix_timestamp(c_decimal_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"unix_timestamp(c_datetime   )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 14, 2},
		{"unix_timestamp(c_datetime_d )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"unix_timestamp(c_time       )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 15, 3},
		{"unix_timestamp(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"unix_timestamp(c_timestamp  )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 16, 4},
		{"unix_timestamp(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"unix_timestamp(c_char       )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 6},
		{"unix_timestamp(c_varchar    )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 6},
		{"unix_timestamp(c_text_d     )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 6},
		{"unix_timestamp(c_binary     )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 6},
		{"unix_timestamp(c_varbinary  )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 6},
		{"unix_timestamp(c_blob_d     )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 6},
		{"unix_timestamp(c_set        )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 6},
		{"unix_timestamp(c_enum       )", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 18, 6},
		{"unix_timestamp(null         )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 11, 0},
		{"unix_timestamp('12:12:12.123')", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 15, 3},
		{"unix_timestamp('12:12:12.1234')", mysql.TypeNewDecimal, charset.CharsetBin, mysql.BinaryFlag, 16, 4},
		// TODO: Add string literal tests for UNIX_TIMESTAMP. UNIX_TIMESTAMP respects the fsp in string literals.

		{"timestampdiff(MONTH, c_datetime, c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"timestampdiff(QuarteR, c_char, c_varchar)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"timestampdiff(second, c_int_d, c_bchar)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"timestampdiff(YEAR, c_blob_d, c_bigint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},

		{"addtime(c_int_d, c_time_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_datetime_d, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"addtime(c_datetime, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"addtime(c_timestamp, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 24, 4},
		{"addtime(c_timestamp_d, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"addtime(c_time, c_time)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"addtime(c_time_d, c_time)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"addtime(c_char, c_time_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_char, c_datetime)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_char, c_int_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_date, c_datetime)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_date, c_timestamp)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_date, c_time)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},

		{"subtime(c_int_d, c_time_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_datetime_d, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"subtime(c_datetime, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"subtime(c_timestamp, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 24, 4},
		{"subtime(c_timestamp_d, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"subtime(c_time, c_time)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"subtime(c_time_d, c_time)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"subtime(c_char, c_time_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_char, c_datetime)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_char, c_int_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_date, c_datetime)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_date, c_timestamp)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_date, c_time)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},

		{"timestamp(c_int_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"timestamp(c_float_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_double_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_decimal)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"timestamp(c_udecimal)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"timestamp(c_decimal_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"timestamp(c_udecimal_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"timestamp(c_datetime)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"timestamp(c_datetime_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"timestamp(c_timestamp)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 24, 4},
		{"timestamp(c_time)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"timestamp(c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"timestamp(c_bchar)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_char)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_varchar)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_text_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_btext_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_blob_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_set)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_enum)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},

		{"timestamp(c_int_d, c_float_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_datetime, c_timestamp)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 24, 4},
		{"timestamp(c_timestamp, c_char)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 26, 6},
		{"timestamp(c_int_d, c_datetime)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"timestamp('2000-01-27','23:13:41')", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 19, 0},
		{"timestamp('2000-01-27','23:13:41.123')", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 23, 3},

		{"addtime(c_int_d, c_time_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_datetime_d, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"addtime(c_datetime, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"addtime(c_timestamp, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 24, 4},
		{"addtime(c_timestamp_d, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"addtime(c_time, c_time)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"addtime(c_time_d, c_time)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"addtime(c_char, c_time_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_char, c_datetime)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_char, c_int_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_date, c_datetime)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_date, c_timestamp)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"addtime(c_date, c_time)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},

		{"subtime(c_int_d, c_time_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_datetime_d, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"subtime(c_datetime, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"subtime(c_timestamp, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 24, 4},
		{"subtime(c_timestamp_d, c_time_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"subtime(c_time, c_time)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"subtime(c_time_d, c_time)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"subtime(c_char, c_time_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_char, c_datetime)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_char, c_int_d)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_date, c_datetime)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_date, c_timestamp)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},
		{"subtime(c_date, c_time)", mysql.TypeString, charset.CharsetUTF8MB4, 0, 26, types.UnspecifiedLength},

		{"hour(c_int_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_bigint_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_float_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_double_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_decimal  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_datetime )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_time     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_timestamp)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_char     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_varchar  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_text_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_binary   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_varbinary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_blob_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_set      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_enum     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},

		{"minute(c_int_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_bigint_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_float_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_double_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_decimal  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_datetime )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_time     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_timestamp)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_char     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_varchar  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_text_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_binary   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_varbinary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_blob_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_set      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_enum     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},

		{"second(c_int_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_bigint_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_float_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_double_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_decimal  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_datetime )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_time     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_timestamp)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_char     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_varchar  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_text_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_binary   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_varbinary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_blob_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_set      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_enum     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},

		{"microsecond(c_int_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_bigint_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_float_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_double_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_decimal  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_datetime )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_time     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_timestamp)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_char     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_varchar  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_text_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_binary   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_varbinary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_blob_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_set      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_enum     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},

		{"datediff(c_char, c_datetime)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"datediff(c_int_d, c_timestamp)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"datediff(c_double_d, c_timestamp)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"datediff(c_bchar, c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"datediff(c_varchar, c_varbinary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},
		{"datediff(c_float_d, c_time)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 20, 0},

		{"dayofmonth(c_int_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_bigint_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_float_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_double_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_decimal  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_datetime )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_time     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_timestamp)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_char     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_varchar  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_text_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_binary   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_varbinary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_blob_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_set      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"dayofmonth(c_enum     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},

		{"dayofyear(c_int_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_bigint_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_float_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_double_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_decimal  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_datetime )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_time     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_timestamp)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_char     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_varchar  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_text_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_binary   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_varbinary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_blob_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_set      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"dayofyear(c_enum     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},

		{"dayofweek(c_bigint_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_float_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_double_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_decimal  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_datetime )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_time     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_timestamp)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_char     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_varchar  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_text_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_binary   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_varbinary)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_blob_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_set      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"dayofweek(c_enum     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},

		{"hour(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},
		{"hour(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 3, 0},

		{"minute(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"minute(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},

		{"second(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"second(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},

		{"microsecond(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"microsecond(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},

		{"last_day(c_datetime)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, 0},
		{"last_day(c_datetime_d)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, 0},
		{"last_day(c_timestamp)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, 0},
		{"last_day(c_timestamp_d)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, 0},
		{"last_day(c_char)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, 0},
		{"last_day(c_varchar)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, 0},
		{"last_day(c_varchar)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, 0},
		{"last_day(c_text_d)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, 0},
		{"last_day(c_blob_d)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, 0},

		{"week(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},

		{"week(c_int_d      , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_bigint_d   , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_float_d    , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_double_d   , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_decimal    , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_datetime   , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_time_d     , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_timestamp_d, c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_char       , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_varchar    , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_text_d     , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_binary     , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_varbinary  , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_blob_d     , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_set        , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"week(c_enum       , c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},

		{"weekofyear(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"weekofyear(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},

		{"yearweek(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"yearweek(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},

		{"year(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},
		{"year(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 4, 0},

		{"month(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},
		{"month(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 2, 0},

		{"monthName(c_int_d      )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_bigint_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_float_d    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_double_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_decimal    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_datetime   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_time_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_char       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_varchar    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_text_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_binary     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_varbinary  )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_blob_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_set        )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"monthName(c_enum       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},

		{"dayName(c_int_d      )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_bigint_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_float_d    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_double_d   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_decimal    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_datetime   )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_time_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_timestamp_d)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_char       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_varchar    )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_text_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_binary     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_varbinary  )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_blob_d     )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_set        )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},
		{"dayName(c_enum       )", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, 10, types.UnspecifiedLength},

		{"now()  ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 19, 0},
		{"now(0) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 19, 0},
		{"now(1) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 21, 1},
		{"now(2) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 22, 2},
		{"now(3) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 23, 3},
		{"now(4) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 24, 4},
		{"now(5) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 25, 5},
		{"now(6) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 26, 6},

		{"utc_timestamp()  ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 19, 0},
		{"utc_timestamp(0) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 19, 0},
		{"utc_timestamp(1) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 21, 1},
		{"utc_timestamp(2) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 22, 2},
		{"utc_timestamp(3) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 23, 3},
		{"utc_timestamp(4) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 24, 4},
		{"utc_timestamp(5) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 25, 5},
		{"utc_timestamp(6) ", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 26, 6},

		{"utc_time()  ", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 8, 0},
		{"utc_time(0) ", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 8, 0},
		{"utc_time(1) ", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 10, 1},
		{"utc_time(2) ", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 11, 2},
		{"utc_time(3) ", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 12, 3},
		{"utc_time(4) ", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 13, 4},
		{"utc_time(5) ", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 14, 5},
		{"utc_time(6) ", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 15, 6},

		{"utc_date()  ", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 10, 0},
		{"curdate()", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 10, 0},
		{"sysdate(4)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 24, 4},
		{"current_date() ", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 10, 0},

		{"date(c_int_d      )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_bigint_d   )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_float_d    )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_double_d   )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_decimal    )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_datetime   )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_time_d     )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_timestamp_d)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_char       )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_varchar    )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_text_d     )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_binary     )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_varbinary  )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_blob_d     )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_set        )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"date(c_enum       )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},

		{"from_days(c_int_d      )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_bigint_d   )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_float_d    )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_double_d   )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_decimal    )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_datetime   )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_time_d     )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_timestamp_d)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_char       )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_varchar    )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_text_d     )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_binary     )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_varbinary  )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_blob_d     )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_set        )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"from_days(c_enum       )", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},

		{"weekday(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"weekday(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},

		{"quarter(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"quarter(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},

		{"current_time()", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 8, types.MinFsp},
		{"current_time(0)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 8, types.MinFsp},
		{"current_time(6)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 15, types.MaxFsp},

		{"sec_to_time(c_int_d      )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"sec_to_time(c_bigint_d   )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"sec_to_time(c_float_d    )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"sec_to_time(c_double_d   )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"sec_to_time(c_decimal    )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"sec_to_time(c_decimal_d  )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"sec_to_time(c_datetime   )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 13, 2},
		{"sec_to_time(c_time       )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"sec_to_time(c_time_d     )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"sec_to_time(c_timestamp  )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 15, 4},
		{"sec_to_time(c_timestamp_d)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"sec_to_time(c_char       )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"sec_to_time(c_varchar    )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"sec_to_time(c_text_d     )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"sec_to_time(c_binary     )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"sec_to_time(c_varbinary  )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"sec_to_time(c_blob_d     )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"sec_to_time(c_set        )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"sec_to_time(c_enum       )", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},

		{"time_to_sec(c_int_d      )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_bigint_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_float_d    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_double_d   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_decimal    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_decimal_d  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_datetime   )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_time       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_time_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_timestamp  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_timestamp_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_char       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_varchar    )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_text_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_binary     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_varbinary  )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_blob_d     )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_set        )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"time_to_sec(c_enum       )", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 10, 0},

		{"str_to_date(c_varchar, '%Y:%m:%d')", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"str_to_date(c_varchar, '%Y:%m:%d %H:%i:%s')", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"str_to_date(c_varchar, '%Y:%m:%d %H:%i:%s.%f')", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"str_to_date(c_varchar, '%H:%i:%s')", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"str_to_date(c_varchar, '%H:%i:%s.%f')", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		{"period_add(c_int_d      , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_bigint_d   , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_float_d    , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_double_d   , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_decimal    , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_datetime   , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_time_d     , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_timestamp_d, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_char       , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_varchar    , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_text_d     , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_binary     , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_varbinary  , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_blob_d     , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_set        , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_add(c_enum       , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},

		{"period_diff(c_int_d      , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_bigint_d   , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_float_d    , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_double_d   , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_decimal    , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_datetime   , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_time_d     , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_timestamp_d, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_char       , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_varchar    , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_text_d     , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_binary     , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_varbinary  , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_blob_d     , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_set        , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},
		{"period_diff(c_enum       , c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 6, 0},

		{"maketime(c_int_d, c_int_d, c_double_d)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"maketime(c_int_d, c_int_d, c_decimal)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"maketime(c_int_d, c_int_d, c_decimal_d)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"maketime(c_int_d, c_int_d, c_char)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"maketime(c_int_d, c_int_d, c_varchar)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 17, 6},
		{"maketime(c_int_d, c_int_d, 1.2345)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 15, 4},

		{"get_format(DATE, 'USA')", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, 17, types.UnspecifiedLength},

		{"convert_tz(c_time_d, c_text_d, c_text_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"convert_tz('2004-01-01 12:00:00.123', c_text_d, c_text_d)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		{"from_unixtime(20170101.999)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 23, 3},
		{"from_unixtime(20170101.1234567)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, mysql.MaxDatetimeWidthWithFsp, int(types.MaxFsp)},
		{"from_unixtime('20170101.999')", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, mysql.MaxDatetimeWidthWithFsp, int(types.MaxFsp)},
		{"from_unixtime(20170101.123, '%H')", mysql.TypeVarString, charset.CharsetUTF8MB4, 0 | mysql.NotNullFlag, -1, types.UnspecifiedLength},

		{"extract(day from c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"extract(hour from c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
	}
}

func (s *InferTypeSuite) createTestCase4DateAddSubFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		// Legacy.
		{"date_add('20121212', interval 1 day)", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, mysql.MaxDatetimeFullWidth, types.MinFsp},
		{"date_sub('20121212', interval 1 day)", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.NotNullFlag, mysql.MaxDatetimeFullWidth, types.MinFsp},
		{"date_add(c_datetime, interval 1 day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_sub(c_datetime, interval 1 day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval 1 hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_sub(c_datetime, interval 1 hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_date, interval 1 day)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_sub(c_date, interval 1 day)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval 1 hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_sub(c_date, interval 1 hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// date + microsecond.
		{"date_add(c_date, interval c_char microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_varchar microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_int_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_bigint_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_double_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_decimal microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// date + second_microsecond.
		{"date_add(c_date, interval c_char second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_varchar second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_int_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_bigint_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_double_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_decimal second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// date + minute_microsecond.
		{"date_add(c_date, interval c_char minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_varchar minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_int_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_bigint_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_double_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_decimal minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// date + hour_microsecond.
		{"date_add(c_date, interval c_char hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_varchar hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_int_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_bigint_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_double_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_decimal hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// date + day_microsecond.
		{"date_add(c_date, interval c_char day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_varchar day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_int_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_bigint_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_double_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_decimal day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// date + second.
		{"date_add(c_date, interval c_char second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_varchar second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_int_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_double_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_date, interval c_decimal second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// date + minute_second.
		{"date_add(c_date, interval c_char minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_varchar minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_int_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_double_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_decimal minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// date + hour_second.
		{"date_add(c_date, interval c_char hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_varchar hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_int_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_double_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_decimal hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// date + day_second.
		{"date_add(c_date, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// date + minute.
		{"date_add(c_date, interval c_char minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_varchar minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_int_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_double_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_decimal minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// date + hour_minute.
		{"date_add(c_date, interval c_char hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_varchar hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_int_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_double_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_decimal hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// date + day_minute.
		{"date_add(c_date, interval c_char day_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_varchar day_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_int_d day_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d day_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_double_d day_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_decimal day_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// date + hour.
		{"date_add(c_date, interval c_char hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_varchar hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_int_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_double_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_decimal hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// date + day_hour.
		{"date_add(c_date, interval c_char day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_varchar day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_int_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_double_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_date, interval c_decimal day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// date + day.
		{"date_add(c_date, interval c_char day)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_varchar day)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_int_d day)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d day)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_double_d day)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_decimal day)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},

		// date + week.
		{"date_add(c_date, interval c_char week)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_varchar week)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_int_d week)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d week)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_double_d week)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_decimal week)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},

		// date + month.
		{"date_add(c_date, interval c_char month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_varchar month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_int_d month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_double_d month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_decimal month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},

		// date + year_month.
		{"date_add(c_date, interval c_char year_month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_varchar year_month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_int_d year_month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d year_month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_double_d year_month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_decimal year_month)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},

		// date + quarter.
		{"date_add(c_date, interval c_char quarter)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_varchar quarter)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_int_d quarter)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d quarter)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_double_d quarter)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_decimal quarter)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},

		// date + year.
		{"date_add(c_date, interval c_char year)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_varchar year)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_int_d year)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_bigint_d year)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_double_d year)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},
		{"date_add(c_date, interval c_decimal year)", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDateWidth, types.MinFsp},

		// time(3) + microsecond.
		{"date_add(c_time, interval c_char microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_varchar microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_int_d microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_bigint_d microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_double_d microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_decimal microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		// time(3) + second_microsecond.
		{"date_add(c_time, interval c_char second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_varchar second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_int_d second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_bigint_d second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_double_d second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_decimal second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		// time(3) + minute_microsecond.
		{"date_add(c_time, interval c_char minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_varchar minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_int_d minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_bigint_d minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_double_d minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_decimal minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		// time(3) + hour_microsecond.
		{"date_add(c_time, interval c_char hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_varchar hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_int_d hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_bigint_d hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_double_d hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_decimal hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		// time(3) + day_microsecond.
		{"date_add(c_time, interval c_char day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_varchar day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_int_d day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_bigint_d day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_double_d day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_decimal day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		// time(3) + second.
		{"date_add(c_time, interval c_char second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_varchar second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_int_d second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_bigint_d second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_double_d second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time, interval c_decimal second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},

		// time(3) + minute_second.
		{"date_add(c_time, interval c_char minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_varchar minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_int_d minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_bigint_d minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_double_d minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_decimal minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},

		// time(3) + hour_second.
		{"date_add(c_time, interval c_char hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_varchar hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_int_d hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_bigint_d hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_double_d hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_decimal hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},

		// time(3) + day_second.
		{"date_add(c_time, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// time(3) + minute.
		{"date_add(c_time, interval c_char minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_varchar minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_int_d minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_bigint_d minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_double_d minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_decimal minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},

		// time(3) + hour_minute.
		{"date_add(c_time, interval c_char hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_varchar hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_int_d hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_bigint_d hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_double_d hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_decimal hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},

		// time(3) + day_minute.
		{"date_add(c_time, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// time(3) + hour.
		{"date_add(c_time, interval c_char hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_varchar hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_int_d hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_bigint_d hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_double_d hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},
		{"date_add(c_time, interval c_decimal hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},

		// time(3) + day_hour.
		{"date_add(c_time, interval c_char day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_varchar day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_int_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_bigint_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_double_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_decimal day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// time(3) + day.
		{"date_add(c_time, interval c_char day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_varchar day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_int_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_bigint_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_double_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_decimal day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// time(3) + week.
		{"date_add(c_time, interval c_char week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_varchar week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_int_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_bigint_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_double_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_decimal week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// time(3) + month.
		{"date_add(c_time, interval c_char month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_varchar month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_int_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_bigint_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_double_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_decimal month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// time(3) + year_month.
		{"date_add(c_time, interval c_char year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_varchar year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_int_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_bigint_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_double_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_decimal year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// time(3) + quarter.
		{"date_add(c_time, interval c_char quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_varchar quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_int_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_bigint_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_double_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_decimal quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// time(3) + year.
		{"date_add(c_time, interval c_char year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_varchar year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_int_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_bigint_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_double_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
		{"date_add(c_time, interval c_decimal year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// time + microsecond.
		{"date_add(c_time_d, interval c_char microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_varchar microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_int_d microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_bigint_d microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_double_d microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_decimal microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		// time + second_microsecond.
		{"date_add(c_time_d, interval c_char second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_varchar second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_int_d second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_bigint_d second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_double_d second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_decimal second_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		// time + minute_microsecond.
		{"date_add(c_time_d, interval c_char minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_varchar minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_int_d minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_bigint_d minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_double_d minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_decimal minute_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		// time + hour_microsecond.
		{"date_add(c_time_d, interval c_char hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_varchar hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_int_d hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_bigint_d hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_double_d hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_decimal hour_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		// time + day_microsecond.
		{"date_add(c_time_d, interval c_char day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_varchar day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_int_d day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_bigint_d day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_double_d day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_decimal day_microsecond)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},

		// time + second.
		{"date_add(c_time_d, interval c_char second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_varchar second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_int_d second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthWithFsp, types.MaxFsp},
		{"date_add(c_time_d, interval c_decimal second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, 14, 3},

		// time + minute_second.
		{"date_add(c_time_d, interval c_char minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal minute_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},

		// time + hour_second.
		{"date_add(c_time_d, interval c_char hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal hour_second)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},

		// time + day_second.
		{"date_add(c_time_d, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// time + minute.
		{"date_add(c_time_d, interval c_char minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},

		// time + hour_minute.
		{"date_add(c_time_d, interval c_char hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal hour_minute)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},

		// time + day_minute.
		{"date_add(c_time_d, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// time + hour.
		{"date_add(c_time_d, interval c_char hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal hour)", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDurationWidthNoFsp, types.MinFsp},

		// time + day_hour.
		{"date_add(c_time_d, interval c_char day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// time + day.
		{"date_add(c_time_d, interval c_char day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// time + week.
		{"date_add(c_time_d, interval c_char week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// time + month.
		{"date_add(c_time_d, interval c_char month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// time + year_month.
		{"date_add(c_time_d, interval c_char year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// time + quarter.
		{"date_add(c_time_d, interval c_char quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// time + year.
		{"date_add(c_time_d, interval c_char year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_varchar year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_int_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_bigint_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_double_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_time_d, interval c_decimal year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime(2) + microsecond.
		{"date_add(c_datetime, interval c_char microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_varchar microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_int_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_bigint_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_double_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_decimal microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// datetime(2) + second_microsecond.
		{"date_add(c_datetime, interval c_char second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_varchar second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_int_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_bigint_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_double_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_decimal second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// datetime(2) + minute_microsecond.
		{"date_add(c_datetime, interval c_char minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_varchar minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_int_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_bigint_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_double_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_decimal minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// datetime(2) + hour_microsecond.
		{"date_add(c_datetime, interval c_char hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_varchar hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_int_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_bigint_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_double_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_decimal hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// datetime(2) + day_microsecond.
		{"date_add(c_datetime, interval c_char day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_varchar day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_int_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_bigint_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_double_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_decimal day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// datetime(2) + second.
		{"date_add(c_datetime, interval c_char second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_varchar second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_int_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime, interval c_decimal second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// datetime(2) + minute_second.
		{"date_add(c_datetime, interval c_char minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + hour_second.
		{"date_add(c_datetime, interval c_char hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + day_second.
		{"date_add(c_datetime, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + minute.
		{"date_add(c_datetime, interval c_char minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + hour_minute.
		{"date_add(c_datetime, interval c_char hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + day_minute.
		{"date_add(c_datetime, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + hour.
		{"date_add(c_datetime, interval c_char hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + day_hour.
		{"date_add(c_datetime, interval c_char day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + day.
		{"date_add(c_datetime, interval c_char day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + week.
		{"date_add(c_datetime, interval c_char week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + month.
		{"date_add(c_datetime, interval c_char month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + year_month.
		{"date_add(c_datetime, interval c_char year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + quarter.
		{"date_add(c_datetime, interval c_char quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime(2) + year.
		{"date_add(c_datetime, interval c_char year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_varchar year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_int_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_bigint_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_double_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},
		{"date_add(c_datetime, interval c_decimal year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 22, 2},

		// datetime + microsecond.
		{"date_add(c_datetime_d, interval c_char microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_varchar microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_int_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_bigint_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_double_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_decimal microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// datetime + second_microsecond.
		{"date_add(c_datetime_d, interval c_char second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_varchar second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_int_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_bigint_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_double_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_decimal second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// datetime + minute_microsecond.
		{"date_add(c_datetime_d, interval c_char minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_varchar minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_int_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_bigint_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_double_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_decimal minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// datetime + hour_microsecond.
		{"date_add(c_datetime_d, interval c_char hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_varchar hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_int_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_bigint_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_double_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_decimal hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// datetime + day_microsecond.
		{"date_add(c_datetime_d, interval c_char day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_varchar day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_int_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_bigint_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_double_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_decimal day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// datetime + second.
		{"date_add(c_datetime_d, interval c_char second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_varchar second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_int_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_datetime_d, interval c_decimal second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// datetime + minute_second.
		{"date_add(c_datetime_d, interval c_char minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + hour_second.
		{"date_add(c_datetime_d, interval c_char hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + day_second.
		{"date_add(c_datetime_d, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + minute.
		{"date_add(c_datetime_d, interval c_char minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + hour_minute.
		{"date_add(c_datetime_d, interval c_char hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + day_minute.
		{"date_add(c_datetime_d, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + hour.
		{"date_add(c_datetime_d, interval c_char hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + day_hour.
		{"date_add(c_datetime_d, interval c_char day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + day.
		{"date_add(c_datetime_d, interval c_char day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + week.
		{"date_add(c_datetime_d, interval c_char week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + month.
		{"date_add(c_datetime_d, interval c_char month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + year_month.
		{"date_add(c_datetime_d, interval c_char year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + quarter.
		{"date_add(c_datetime_d, interval c_char quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// datetime + year.
		{"date_add(c_datetime_d, interval c_char year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_varchar year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_int_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_bigint_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_double_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_datetime_d, interval c_decimal year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + microsecond.
		{"date_add(c_timestamp_d, interval c_char microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_varchar microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_int_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_double_d microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_decimal microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// timestamp + second_microsecond.
		{"date_add(c_timestamp_d, interval c_char second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_varchar second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_int_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_double_d second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_decimal second_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// timestamp + minute_microsecond.
		{"date_add(c_timestamp_d, interval c_char minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_varchar minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_int_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_double_d minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_decimal minute_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// timestamp + hour_microsecond.
		{"date_add(c_timestamp_d, interval c_char hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_varchar hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_int_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_double_d hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_decimal hour_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// timestamp + day_microsecond.
		{"date_add(c_timestamp_d, interval c_char day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_varchar day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_int_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_double_d day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_decimal day_microsecond)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},

		// timestamp + second.
		{"date_add(c_timestamp_d, interval c_char second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_varchar second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_int_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthWithFsp, types.MaxFsp},
		{"date_add(c_timestamp_d, interval c_decimal second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},

		// timestamp + minute_second.
		{"date_add(c_timestamp_d, interval c_char minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal minute_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + hour_second.
		{"date_add(c_timestamp_d, interval c_char hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal hour_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + day_second.
		{"date_add(c_timestamp_d, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + minute.
		{"date_add(c_timestamp_d, interval c_char minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + hour_minute.
		{"date_add(c_timestamp_d, interval c_char hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal hour_minute)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + day_minute.
		{"date_add(c_timestamp_d, interval c_char day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal day_second)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + hour.
		{"date_add(c_timestamp_d, interval c_char hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + day_hour.
		{"date_add(c_timestamp_d, interval c_char day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal day_hour)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + day.
		{"date_add(c_timestamp_d, interval c_char day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal day)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + week.
		{"date_add(c_timestamp_d, interval c_char week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal week)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + month.
		{"date_add(c_timestamp_d, interval c_char month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + year_month.
		{"date_add(c_timestamp_d, interval c_char year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal year_month)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + quarter.
		{"date_add(c_timestamp_d, interval c_char quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal quarter)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// timestamp + year.
		{"date_add(c_timestamp_d, interval c_char year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_varchar year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_int_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_bigint_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_double_d year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},
		{"date_add(c_timestamp_d, interval c_decimal year)", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxDatetimeWidthNoFsp, types.MinFsp},

		// Rest + *.
		{"date_add(c_bit, interval c_char microsecond)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeFullWidth, types.MinFsp},
		{"date_add(c_int_d, interval c_varchar second)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeFullWidth, types.MinFsp},
		{"date_add(c_double_d, interval c_int_d day_microsecond)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeFullWidth, types.MinFsp},
		{"date_add(c_decimal, interval c_bigint_d hour)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeFullWidth, types.MinFsp},
		{"date_add(c_varchar, interval c_double_d day)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeFullWidth, types.MinFsp},
		{"date_add(c_binary, interval c_decimal year)", mysql.TypeVarString, charset.CharsetUTF8MB4, 0, mysql.MaxDatetimeFullWidth, types.MinFsp},
	}
}

func (s *InferTypeSuite) createTestCase4LikeFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"c_int_d       rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_bigint_d    rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_float_d     rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_double_d    rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_decimal     rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_datetime    rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_time_d      rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_timestamp_d rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_char        rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_varchar     rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_text_d      rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_binary      rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_varbinary   rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_blob_d      rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_set         rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_enum        rlike c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},

		{"c_int_d       regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_bigint_d    regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_float_d     regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_double_d    regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_decimal     regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_datetime    regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_time_d      regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_timestamp_d regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_char        regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_varchar     regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_text_d      regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_binary      regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_varbinary   regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_blob_d      regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_set         regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
		{"c_enum        regexp c_text_d", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, 1, 0},
	}
}

func (s *InferTypeSuite) createTestCase4Literals() []typeInferTestCase {
	return []typeInferTestCase{
		{"time       '00:00:00'", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 10, 0},
		{"time       '00'", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 10, 0},
		{"time       '3 00:00:00'", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 10, 0},
		{"time       '3 00:00:00.1234'", mysql.TypeDuration, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 15, 4},
		{"timestamp  '2017-01-01 01:01:01'", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, mysql.MaxDatetimeWidthNoFsp, 0},
		{"timestamp  '2017-01-00000000001 01:01:01.001'", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 23, 3},
		{"date '2017-01-01'", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag | mysql.NotNullFlag, 10, 0},
	}
}

func (s *InferTypeSuite) createTestCase4JSONFuncs() []typeInferTestCase {
	return []typeInferTestCase{
		{"json_type(c_json)", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.BinaryFlag, 51, types.UnspecifiedLength},
		{"json_unquote(c_json)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxLongBlobWidth, types.UnspecifiedLength},
		{"json_extract(c_json, '')", mysql.TypeJSON, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxBlobWidth, 0},
		{"json_set(c_json, '', 0)", mysql.TypeJSON, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxBlobWidth, 0},
		{"json_insert(c_json, '', 0)", mysql.TypeJSON, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxBlobWidth, 0},
		{"json_replace(c_json, '', 0)", mysql.TypeJSON, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxBlobWidth, 0},
		{"json_remove(c_json, '')", mysql.TypeJSON, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxBlobWidth, 0},
		{"json_merge(c_json, c_json)", mysql.TypeJSON, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxBlobWidth, 0},
		{"json_object('k', 'v')", mysql.TypeJSON, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxBlobWidth, 0},
		{"json_array('k', 'v')", mysql.TypeJSON, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxBlobWidth, 0},
		{"json_pretty(c_json)", mysql.TypeLongBlob, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxBlobWidth * 4, types.UnspecifiedLength},
		{"json_contains(c_json, 'a')", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"json_contains_path(c_json, 'one', '$.a')", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{"json_quote('k')", mysql.TypeVarString, charset.CharsetUTF8MB4, mysql.BinaryFlag | mysql.NotNullFlag, 8, types.UnspecifiedLength},
		{"json_valid(c_json)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag | mysql.IsBooleanFlag, mysql.MaxIntWidth, 0},
		{`json_search(c_json, 'one', '"a"')`, mysql.TypeJSON, charset.CharsetUTF8MB4, mysql.BinaryFlag, mysql.MaxBlobWidth, 0},
		{`json_depth(c_json)`, mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
		{`json_length(c_json)`, mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, mysql.MaxIntWidth, 0},
	}
}

func (s *InferTypeSuite) createTestCase4MiscellaneousFunc() []typeInferTestCase {
	return []typeInferTestCase{
		{"get_lock(c_char, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"get_lock(c_char, c_bigint_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"get_lock(c_char, c_float_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"get_lock(c_char, c_double_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"get_lock(c_char, c_decimal)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"get_lock(c_varchar, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"get_lock(c_text_d, c_int_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},

		{"release_lock(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"release_lock(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"release_lock(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"release_lock(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"release_lock(c_char)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"release_lock(c_varchar)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
		{"release_lock(c_text_d)", mysql.TypeLonglong, charset.CharsetBin, mysql.BinaryFlag, 1, 0},
	}
}

func (s *InferTypeSuite) createTestCase4GetVarFunc() []typeInferTestCase {
	return []typeInferTestCase{
		{"@a", mysql.TypeDate, charset.CharsetBin, mysql.BinaryFlag, 10, 0},
		{"@b", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"@c", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 19, 0},
		{"@d", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 24, 4},
		{"@e", mysql.TypeDatetime, charset.CharsetBin, mysql.BinaryFlag, 23, 3},
	}
}
