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

package expression

import (
	"github.com/pingcap/errors"
	"github.com/pingcap/tidb/pkg/types"
	"github.com/pingcap/tidb/pkg/util/chunk"
)

func genVecFromConstExpr(ctx EvalContext, expr Expression, targetType types.EvalType, input *chunk.Chunk, result *chunk.Column) error {
	n := 1
	if input != nil {
		n = input.NumRows()
		if n == 0 {
			result.Reset(targetType)
			return nil
		}
	}
	switch targetType {
	case types.ETInt:
		v, isNull, err := expr.EvalInt(ctx, chunk.Row{})
		if err != nil {
			return err
		}
		if isNull {
			result.ResizeInt64(n, true)
			return nil
		}
		result.ResizeInt64(n, false)
		i64s := result.Int64s()
		for i := range i64s {
			i64s[i] = v
		}
	case types.ETReal:
		v, isNull, err := expr.EvalReal(ctx, chunk.Row{})
		if err != nil {
			return err
		}
		if isNull {
			result.ResizeFloat64(n, true)
			return nil
		}
		result.ResizeFloat64(n, false)
		f64s := result.Float64s()
		for i := range f64s {
			f64s[i] = v
		}
	case types.ETDecimal:
		v, isNull, err := expr.EvalDecimal(ctx, chunk.Row{})
		if err != nil {
			return err
		}
		if isNull {
			result.ResizeDecimal(n, true)
			return nil
		}
		result.ResizeDecimal(n, false)
		ds := result.Decimals()
		for i := range ds {
			ds[i] = *v
		}
	case types.ETDatetime, types.ETTimestamp:
		v, isNull, err := expr.EvalTime(ctx, chunk.Row{})
		if err != nil {
			return err
		}
		if isNull {
			result.ResizeTime(n, true)
			return nil
		}
		result.ResizeTime(n, false)
		ts := result.Times()
		for i := range ts {
			ts[i] = v
		}
	case types.ETDuration:
		v, isNull, err := expr.EvalDuration(ctx, chunk.Row{})
		if err != nil {
			return err
		}
		if isNull {
			result.ResizeGoDuration(n, true)
			return nil
		}
		result.ResizeGoDuration(n, false)
		ds := result.GoDurations()
		for i := range ds {
			ds[i] = v.Duration
		}
	case types.ETJson:
		result.ReserveJSON(n)
		v, isNull, err := expr.EvalJSON(ctx, chunk.Row{})
		if err != nil {
			return err
		}
		if isNull {
			for range n {
				result.AppendNull()
			}
		} else {
			for range n {
				result.AppendJSON(v)
			}
		}
	case types.ETVectorFloat32:
		result.ReserveVectorFloat32(n)
		v, isNull, err := expr.EvalVectorFloat32(ctx, chunk.Row{})
		if err != nil {
			return err
		}
		if isNull {
			for range n {
				result.AppendNull()
			}
		} else {
			for range n {
				result.AppendVectorFloat32(v)
			}
		}
	case types.ETString:
		result.ReserveString(n)
		v, isNull, err := expr.EvalString(ctx, chunk.Row{})
		if err != nil {
			return err
		}
		if isNull {
			for range n {
				result.AppendNull()
			}
		} else {
			for range n {
				result.AppendString(v)
			}
		}
	default:
		return errors.Errorf("unsupported type %s during evaluation", targetType)
	}
	return nil
}
