// Code generated by execgen; DO NOT EDIT.
// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package colexecagg

import (
	"unsafe"

	"github.com/cockroachdb/apd/v3"
	"github.com/cockroachdb/cockroach/pkg/col/coldata"
	"github.com/cockroachdb/cockroach/pkg/col/typeconv"
	"github.com/cockroachdb/cockroach/pkg/sql/colexecerror"
	"github.com/cockroachdb/cockroach/pkg/sql/colmem"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/duration"
	"github.com/cockroachdb/errors"
)

// Workaround for bazel auto-generated code. goimports does not automatically
// pick up the right packages when run within the bazel sandbox.
var (
	_ tree.AggType
	_ apd.Context
	_ duration.Duration
	_ = typeconv.TypeFamilyToCanonicalTypeFamily
)

func newSumWindowAggAlloc(
	allocator *colmem.Allocator, t *types.T, allocSize int64,
) (aggregateFuncAlloc, error) {
	allocBase := aggAllocBase{allocator: allocator, allocSize: allocSize}
	switch t.Family() {
	case types.IntFamily:
		switch t.Width() {
		case 16:
			return &sumInt16WindowAggAlloc{aggAllocBase: allocBase}, nil
		case 32:
			return &sumInt32WindowAggAlloc{aggAllocBase: allocBase}, nil
		case -1:
		default:
			return &sumInt64WindowAggAlloc{aggAllocBase: allocBase}, nil
		}
	case types.DecimalFamily:
		switch t.Width() {
		case -1:
		default:
			return &sumDecimalWindowAggAlloc{aggAllocBase: allocBase}, nil
		}
	case types.FloatFamily:
		switch t.Width() {
		case -1:
		default:
			return &sumFloat64WindowAggAlloc{aggAllocBase: allocBase}, nil
		}
	case types.IntervalFamily:
		switch t.Width() {
		case -1:
		default:
			return &sumIntervalWindowAggAlloc{aggAllocBase: allocBase}, nil
		}
	}
	return nil, errors.AssertionFailedf("unsupported sum agg type %s", t.Name())
}

type sumInt16WindowAgg struct {
	unorderedAggregateFuncBase
	// curAgg holds the running total, so we can index into the slice once per
	// group, instead of on each iteration.
	curAgg apd.Decimal
	// numNonNull tracks the number of non-null values we have seen for the group
	// that is currently being aggregated.
	numNonNull uint64
}

var _ AggregateFunc = &sumInt16WindowAgg{}

func (a *sumInt16WindowAgg) Compute(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int, sel []int,
) {
	oldCurAggSize := a.curAgg.Size()
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Int16(), vec.Nulls()
	// Unnecessary memory accounting can have significant overhead for window
	// aggregate functions because Compute is called at least once for every row.
	// For this reason, we do not use PerformOperation here.
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Add(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull++
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Add(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull++
			}
		}
	}
	newCurAggSize := a.curAgg.Size()
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsageAfterAllocation(int64(newCurAggSize - oldCurAggSize))
	}
}

func (a *sumInt16WindowAgg) Flush(outputIdx int) {
	// The aggregation is finished. Flush the last value. If we haven't found
	// any non-nulls for this group so far, the output for this group should be
	// null.
	col := a.vec.Decimal()
	if a.numNonNull == 0 {
		a.nulls.SetNull(outputIdx)
	} else {
		col.Set(outputIdx, a.curAgg)
	}
}

func (a *sumInt16WindowAgg) Reset() {
	a.curAgg = zeroDecimalValue
	a.numNonNull = 0
}

type sumInt16WindowAggAlloc struct {
	aggAllocBase
	aggFuncs []sumInt16WindowAgg
}

var _ aggregateFuncAlloc = &sumInt16WindowAggAlloc{}

const sizeOfSumInt16WindowAgg = int64(unsafe.Sizeof(sumInt16WindowAgg{}))
const sumInt16WindowAggSliceOverhead = int64(unsafe.Sizeof([]sumInt16WindowAgg{}))

func (a *sumInt16WindowAggAlloc) newAggFunc() AggregateFunc {
	if len(a.aggFuncs) == 0 {
		a.allocator.AdjustMemoryUsage(sumInt16WindowAggSliceOverhead + sizeOfSumInt16WindowAgg*a.allocSize)
		a.aggFuncs = make([]sumInt16WindowAgg, a.allocSize)
	}
	f := &a.aggFuncs[0]
	f.allocator = a.allocator
	a.aggFuncs = a.aggFuncs[1:]
	return f
}

// Remove implements the slidingWindowAggregateFunc interface (see
// window_aggregator_tmpl.go).
func (a *sumInt16WindowAgg) Remove(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int,
) {
	oldCurAggSize := a.curAgg.Size()
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Int16(), vec.Nulls()
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Sub(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull--
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Sub(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull--
			}
		}
	}
	newCurAggSize := a.curAgg.Size()
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsage(int64(newCurAggSize - oldCurAggSize))
	}
}

type sumInt32WindowAgg struct {
	unorderedAggregateFuncBase
	// curAgg holds the running total, so we can index into the slice once per
	// group, instead of on each iteration.
	curAgg apd.Decimal
	// numNonNull tracks the number of non-null values we have seen for the group
	// that is currently being aggregated.
	numNonNull uint64
}

var _ AggregateFunc = &sumInt32WindowAgg{}

func (a *sumInt32WindowAgg) Compute(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int, sel []int,
) {
	oldCurAggSize := a.curAgg.Size()
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Int32(), vec.Nulls()
	// Unnecessary memory accounting can have significant overhead for window
	// aggregate functions because Compute is called at least once for every row.
	// For this reason, we do not use PerformOperation here.
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Add(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull++
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Add(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull++
			}
		}
	}
	newCurAggSize := a.curAgg.Size()
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsageAfterAllocation(int64(newCurAggSize - oldCurAggSize))
	}
}

func (a *sumInt32WindowAgg) Flush(outputIdx int) {
	// The aggregation is finished. Flush the last value. If we haven't found
	// any non-nulls for this group so far, the output for this group should be
	// null.
	col := a.vec.Decimal()
	if a.numNonNull == 0 {
		a.nulls.SetNull(outputIdx)
	} else {
		col.Set(outputIdx, a.curAgg)
	}
}

func (a *sumInt32WindowAgg) Reset() {
	a.curAgg = zeroDecimalValue
	a.numNonNull = 0
}

type sumInt32WindowAggAlloc struct {
	aggAllocBase
	aggFuncs []sumInt32WindowAgg
}

var _ aggregateFuncAlloc = &sumInt32WindowAggAlloc{}

const sizeOfSumInt32WindowAgg = int64(unsafe.Sizeof(sumInt32WindowAgg{}))
const sumInt32WindowAggSliceOverhead = int64(unsafe.Sizeof([]sumInt32WindowAgg{}))

func (a *sumInt32WindowAggAlloc) newAggFunc() AggregateFunc {
	if len(a.aggFuncs) == 0 {
		a.allocator.AdjustMemoryUsage(sumInt32WindowAggSliceOverhead + sizeOfSumInt32WindowAgg*a.allocSize)
		a.aggFuncs = make([]sumInt32WindowAgg, a.allocSize)
	}
	f := &a.aggFuncs[0]
	f.allocator = a.allocator
	a.aggFuncs = a.aggFuncs[1:]
	return f
}

// Remove implements the slidingWindowAggregateFunc interface (see
// window_aggregator_tmpl.go).
func (a *sumInt32WindowAgg) Remove(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int,
) {
	oldCurAggSize := a.curAgg.Size()
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Int32(), vec.Nulls()
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Sub(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull--
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Sub(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull--
			}
		}
	}
	newCurAggSize := a.curAgg.Size()
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsage(int64(newCurAggSize - oldCurAggSize))
	}
}

type sumInt64WindowAgg struct {
	unorderedAggregateFuncBase
	// curAgg holds the running total, so we can index into the slice once per
	// group, instead of on each iteration.
	curAgg apd.Decimal
	// numNonNull tracks the number of non-null values we have seen for the group
	// that is currently being aggregated.
	numNonNull uint64
}

var _ AggregateFunc = &sumInt64WindowAgg{}

func (a *sumInt64WindowAgg) Compute(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int, sel []int,
) {
	oldCurAggSize := a.curAgg.Size()
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Int64(), vec.Nulls()
	// Unnecessary memory accounting can have significant overhead for window
	// aggregate functions because Compute is called at least once for every row.
	// For this reason, we do not use PerformOperation here.
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Add(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull++
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Add(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull++
			}
		}
	}
	newCurAggSize := a.curAgg.Size()
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsageAfterAllocation(int64(newCurAggSize - oldCurAggSize))
	}
}

func (a *sumInt64WindowAgg) Flush(outputIdx int) {
	// The aggregation is finished. Flush the last value. If we haven't found
	// any non-nulls for this group so far, the output for this group should be
	// null.
	col := a.vec.Decimal()
	if a.numNonNull == 0 {
		a.nulls.SetNull(outputIdx)
	} else {
		col.Set(outputIdx, a.curAgg)
	}
}

func (a *sumInt64WindowAgg) Reset() {
	a.curAgg = zeroDecimalValue
	a.numNonNull = 0
}

type sumInt64WindowAggAlloc struct {
	aggAllocBase
	aggFuncs []sumInt64WindowAgg
}

var _ aggregateFuncAlloc = &sumInt64WindowAggAlloc{}

const sizeOfSumInt64WindowAgg = int64(unsafe.Sizeof(sumInt64WindowAgg{}))
const sumInt64WindowAggSliceOverhead = int64(unsafe.Sizeof([]sumInt64WindowAgg{}))

func (a *sumInt64WindowAggAlloc) newAggFunc() AggregateFunc {
	if len(a.aggFuncs) == 0 {
		a.allocator.AdjustMemoryUsage(sumInt64WindowAggSliceOverhead + sizeOfSumInt64WindowAgg*a.allocSize)
		a.aggFuncs = make([]sumInt64WindowAgg, a.allocSize)
	}
	f := &a.aggFuncs[0]
	f.allocator = a.allocator
	a.aggFuncs = a.aggFuncs[1:]
	return f
}

// Remove implements the slidingWindowAggregateFunc interface (see
// window_aggregator_tmpl.go).
func (a *sumInt64WindowAgg) Remove(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int,
) {
	oldCurAggSize := a.curAgg.Size()
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Int64(), vec.Nulls()
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Sub(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull--
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					var tmpDec apd.Decimal //gcassert:noescape
					tmpDec.SetInt64(int64(v))
					if _, err := tree.ExactCtx.Sub(&a.curAgg, &a.curAgg, &tmpDec); err != nil {
						colexecerror.ExpectedError(err)
					}
				}

				a.numNonNull--
			}
		}
	}
	newCurAggSize := a.curAgg.Size()
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsage(int64(newCurAggSize - oldCurAggSize))
	}
}

type sumDecimalWindowAgg struct {
	unorderedAggregateFuncBase
	// curAgg holds the running total, so we can index into the slice once per
	// group, instead of on each iteration.
	curAgg apd.Decimal
	// numNonNull tracks the number of non-null values we have seen for the group
	// that is currently being aggregated.
	numNonNull uint64
}

var _ AggregateFunc = &sumDecimalWindowAgg{}

func (a *sumDecimalWindowAgg) Compute(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int, sel []int,
) {
	oldCurAggSize := a.curAgg.Size()
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Decimal(), vec.Nulls()
	// Unnecessary memory accounting can have significant overhead for window
	// aggregate functions because Compute is called at least once for every row.
	// For this reason, we do not use PerformOperation here.
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					_, err := tree.ExactCtx.Add(&a.curAgg, &a.curAgg, &v)
					if err != nil {
						colexecerror.ExpectedError(err)
					}

				}

				a.numNonNull++
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					_, err := tree.ExactCtx.Add(&a.curAgg, &a.curAgg, &v)
					if err != nil {
						colexecerror.ExpectedError(err)
					}

				}

				a.numNonNull++
			}
		}
	}
	newCurAggSize := a.curAgg.Size()
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsageAfterAllocation(int64(newCurAggSize - oldCurAggSize))
	}
}

func (a *sumDecimalWindowAgg) Flush(outputIdx int) {
	// The aggregation is finished. Flush the last value. If we haven't found
	// any non-nulls for this group so far, the output for this group should be
	// null.
	col := a.vec.Decimal()
	if a.numNonNull == 0 {
		a.nulls.SetNull(outputIdx)
	} else {
		col.Set(outputIdx, a.curAgg)
	}
}

func (a *sumDecimalWindowAgg) Reset() {
	a.curAgg = zeroDecimalValue
	a.numNonNull = 0
}

type sumDecimalWindowAggAlloc struct {
	aggAllocBase
	aggFuncs []sumDecimalWindowAgg
}

var _ aggregateFuncAlloc = &sumDecimalWindowAggAlloc{}

const sizeOfSumDecimalWindowAgg = int64(unsafe.Sizeof(sumDecimalWindowAgg{}))
const sumDecimalWindowAggSliceOverhead = int64(unsafe.Sizeof([]sumDecimalWindowAgg{}))

func (a *sumDecimalWindowAggAlloc) newAggFunc() AggregateFunc {
	if len(a.aggFuncs) == 0 {
		a.allocator.AdjustMemoryUsage(sumDecimalWindowAggSliceOverhead + sizeOfSumDecimalWindowAgg*a.allocSize)
		a.aggFuncs = make([]sumDecimalWindowAgg, a.allocSize)
	}
	f := &a.aggFuncs[0]
	f.allocator = a.allocator
	a.aggFuncs = a.aggFuncs[1:]
	return f
}

// Remove implements the slidingWindowAggregateFunc interface (see
// window_aggregator_tmpl.go).
func (a *sumDecimalWindowAgg) Remove(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int,
) {
	oldCurAggSize := a.curAgg.Size()
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Decimal(), vec.Nulls()
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					_, err := tree.ExactCtx.Sub(&a.curAgg, &a.curAgg, &v)
					if err != nil {
						colexecerror.ExpectedError(err)
					}

				}

				a.numNonNull--
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					_, err := tree.ExactCtx.Sub(&a.curAgg, &a.curAgg, &v)
					if err != nil {
						colexecerror.ExpectedError(err)
					}

				}

				a.numNonNull--
			}
		}
	}
	newCurAggSize := a.curAgg.Size()
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsage(int64(newCurAggSize - oldCurAggSize))
	}
}

type sumFloat64WindowAgg struct {
	unorderedAggregateFuncBase
	// curAgg holds the running total, so we can index into the slice once per
	// group, instead of on each iteration.
	curAgg float64
	// numNonNull tracks the number of non-null values we have seen for the group
	// that is currently being aggregated.
	numNonNull uint64
}

var _ AggregateFunc = &sumFloat64WindowAgg{}

func (a *sumFloat64WindowAgg) Compute(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int, sel []int,
) {
	var oldCurAggSize uintptr
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Float64(), vec.Nulls()
	// Unnecessary memory accounting can have significant overhead for window
	// aggregate functions because Compute is called at least once for every row.
	// For this reason, we do not use PerformOperation here.
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					a.curAgg = float64(a.curAgg) + float64(v)
				}

				a.numNonNull++
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					a.curAgg = float64(a.curAgg) + float64(v)
				}

				a.numNonNull++
			}
		}
	}
	var newCurAggSize uintptr
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsageAfterAllocation(int64(newCurAggSize - oldCurAggSize))
	}
}

func (a *sumFloat64WindowAgg) Flush(outputIdx int) {
	// The aggregation is finished. Flush the last value. If we haven't found
	// any non-nulls for this group so far, the output for this group should be
	// null.
	col := a.vec.Float64()
	if a.numNonNull == 0 {
		a.nulls.SetNull(outputIdx)
	} else {
		col.Set(outputIdx, a.curAgg)
	}
}

func (a *sumFloat64WindowAgg) Reset() {
	a.curAgg = zeroFloat64Value
	a.numNonNull = 0
}

type sumFloat64WindowAggAlloc struct {
	aggAllocBase
	aggFuncs []sumFloat64WindowAgg
}

var _ aggregateFuncAlloc = &sumFloat64WindowAggAlloc{}

const sizeOfSumFloat64WindowAgg = int64(unsafe.Sizeof(sumFloat64WindowAgg{}))
const sumFloat64WindowAggSliceOverhead = int64(unsafe.Sizeof([]sumFloat64WindowAgg{}))

func (a *sumFloat64WindowAggAlloc) newAggFunc() AggregateFunc {
	if len(a.aggFuncs) == 0 {
		a.allocator.AdjustMemoryUsage(sumFloat64WindowAggSliceOverhead + sizeOfSumFloat64WindowAgg*a.allocSize)
		a.aggFuncs = make([]sumFloat64WindowAgg, a.allocSize)
	}
	f := &a.aggFuncs[0]
	f.allocator = a.allocator
	a.aggFuncs = a.aggFuncs[1:]
	return f
}

// Remove implements the slidingWindowAggregateFunc interface (see
// window_aggregator_tmpl.go).
func (a *sumFloat64WindowAgg) Remove(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int,
) {
	var oldCurAggSize uintptr
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Float64(), vec.Nulls()
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					a.curAgg = float64(a.curAgg) - float64(v)
				}

				a.numNonNull--
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)

				{

					a.curAgg = float64(a.curAgg) - float64(v)
				}

				a.numNonNull--
			}
		}
	}
	var newCurAggSize uintptr
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsage(int64(newCurAggSize - oldCurAggSize))
	}
}

type sumIntervalWindowAgg struct {
	unorderedAggregateFuncBase
	// curAgg holds the running total, so we can index into the slice once per
	// group, instead of on each iteration.
	curAgg duration.Duration
	// numNonNull tracks the number of non-null values we have seen for the group
	// that is currently being aggregated.
	numNonNull uint64
}

var _ AggregateFunc = &sumIntervalWindowAgg{}

func (a *sumIntervalWindowAgg) Compute(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int, sel []int,
) {
	var oldCurAggSize uintptr
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Interval(), vec.Nulls()
	// Unnecessary memory accounting can have significant overhead for window
	// aggregate functions because Compute is called at least once for every row.
	// For this reason, we do not use PerformOperation here.
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)
				a.curAgg = a.curAgg.Add(v)
				a.numNonNull++
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)
				a.curAgg = a.curAgg.Add(v)
				a.numNonNull++
			}
		}
	}
	var newCurAggSize uintptr
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsageAfterAllocation(int64(newCurAggSize - oldCurAggSize))
	}
}

func (a *sumIntervalWindowAgg) Flush(outputIdx int) {
	// The aggregation is finished. Flush the last value. If we haven't found
	// any non-nulls for this group so far, the output for this group should be
	// null.
	col := a.vec.Interval()
	if a.numNonNull == 0 {
		a.nulls.SetNull(outputIdx)
	} else {
		col.Set(outputIdx, a.curAgg)
	}
}

func (a *sumIntervalWindowAgg) Reset() {
	a.curAgg = zeroIntervalValue
	a.numNonNull = 0
}

type sumIntervalWindowAggAlloc struct {
	aggAllocBase
	aggFuncs []sumIntervalWindowAgg
}

var _ aggregateFuncAlloc = &sumIntervalWindowAggAlloc{}

const sizeOfSumIntervalWindowAgg = int64(unsafe.Sizeof(sumIntervalWindowAgg{}))
const sumIntervalWindowAggSliceOverhead = int64(unsafe.Sizeof([]sumIntervalWindowAgg{}))

func (a *sumIntervalWindowAggAlloc) newAggFunc() AggregateFunc {
	if len(a.aggFuncs) == 0 {
		a.allocator.AdjustMemoryUsage(sumIntervalWindowAggSliceOverhead + sizeOfSumIntervalWindowAgg*a.allocSize)
		a.aggFuncs = make([]sumIntervalWindowAgg, a.allocSize)
	}
	f := &a.aggFuncs[0]
	f.allocator = a.allocator
	a.aggFuncs = a.aggFuncs[1:]
	return f
}

// Remove implements the slidingWindowAggregateFunc interface (see
// window_aggregator_tmpl.go).
func (a *sumIntervalWindowAgg) Remove(
	vecs []*coldata.Vec, inputIdxs []uint32, startIdx, endIdx int,
) {
	var oldCurAggSize uintptr
	vec := vecs[inputIdxs[0]]
	col, nulls := vec.Interval(), vec.Nulls()
	_, _ = col.Get(endIdx-1), col.Get(startIdx)
	if nulls.MaybeHasNulls() {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = nulls.NullAt(i)
			if !isNull {
				//gcassert:bce
				v := col.Get(i)
				a.curAgg = a.curAgg.Sub(v)
				a.numNonNull--
			}
		}
	} else {
		for i := startIdx; i < endIdx; i++ {

			var isNull bool
			isNull = false
			if !isNull {
				//gcassert:bce
				v := col.Get(i)
				a.curAgg = a.curAgg.Sub(v)
				a.numNonNull--
			}
		}
	}
	var newCurAggSize uintptr
	if newCurAggSize != oldCurAggSize {
		a.allocator.AdjustMemoryUsage(int64(newCurAggSize - oldCurAggSize))
	}
}
