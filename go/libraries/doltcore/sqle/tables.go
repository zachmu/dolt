// Copyright 2019 Liquidata, Inc.
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

package sqle

import (
	"context"
	"errors"
	"fmt"
	"github.com/liquidata-inc/dolt/go/cmd/dolt/errhand"
	"github.com/liquidata-inc/dolt/go/libraries/doltcore/row"
	"io"

	"github.com/src-d/go-mysql-server/sql"

	"github.com/liquidata-inc/dolt/go/libraries/doltcore/doltdb"
	"github.com/liquidata-inc/dolt/go/libraries/doltcore/schema"
)

// DoltTable implements the sql.Table interface and gives access to dolt table rows and schema.
type DoltTable struct {
	name  string
	table *doltdb.Table
	sch   schema.Schema
	db    *Database
}

// Implements sql.IndexableTable
func (t *DoltTable) WithIndexLookup(lookup sql.IndexLookup) sql.Table {
	dil, ok := lookup.(*doltIndexLookup)
	if !ok {
		panic(fmt.Sprintf("Unrecognized indexLookup %T", lookup))
	}

	return &IndexedDoltTable{
		table:       t,
		indexLookup: dil,
	}
}

// Implements sql.IndexableTable
func (t *DoltTable) IndexKeyValues(*sql.Context, []string) (sql.PartitionIndexKeyValueIter, error) {
	return nil, errors.New("creating new indexes not supported")
}

// Implements sql.IndexableTable
func (t *DoltTable) IndexLookup() sql.IndexLookup {
	panic("IndexLookup called on DoltTable, should be on IndexedDoltTable")
}

// Name returns the name of the table.
func (t *DoltTable) Name() string {
	return t.name
}

// Not sure what the purpose of this method is, so returning the name for now.
func (t *DoltTable) String() string {
	return t.name
}

// Schema returns the schema for this table.
func (t *DoltTable) Schema() sql.Schema {
	// TODO: fix panics
	sch, err := t.table.GetSchema(context.TODO())

	if err != nil {
		panic(err)
	}

	// TODO: fix panics
	sqlSch, err := doltSchemaToSqlSchema(t.name, sch)

	if err != nil {
		panic(err)
	}

	return sqlSch
}

// Returns the partitions for this table. We return a single partition, but could potentially get more performance by
// returning multiple.
func (t *DoltTable) Partitions(*sql.Context) (sql.PartitionIter, error) {
	return &doltTablePartitionIter{}, nil
}

// Returns the table rows for the partition given (all rows of the table).
func (t *DoltTable) PartitionRows(ctx *sql.Context, _ sql.Partition) (sql.RowIter, error) {
	return newRowIterator(t, ctx)
}

// Insert adds the given row to the table and updates the database root.
// Returns true if the row did not exist and was inserted successfully.
func (t *DoltTable) Insert(ctx *sql.Context, sqlRow sql.Row) (bool, error) {
	dRow, err := SqlRowToDoltRow(t.table.Format(), sqlRow, t.sch)
	if err != nil {
		return false, err
	}

	taggedVals := row.TaggedValues{}
	for _, pkTag := range t.sch.GetPKCols().Tags {
		dCol, ok := dRow.GetColVal(pkTag)
		if !ok {
			return false, errors.New("error: primary key missing from values")
		}
		dColVal, err := dCol.Value(ctx)
		if err != nil {
			return false, errhand.BuildDError("error: failed to read primary key value").AddCause(err).Build()
		}
		taggedVals = taggedVals.Set(pkTag, dColVal)
	}
	_, rowExists, err := t.table.GetRowByPKVals(ctx, taggedVals, t.sch)
	if err != nil {
		return false, errhand.BuildDError("error: failed to read table").AddCause(err).Build()
	}
	if rowExists {
		return false, nil
	}

	typesMap, err := t.table.GetRowData(ctx)
	if err != nil {
		return false, errhand.BuildDError("error: failed to get row data.").AddCause(err).Build()
	}
	mapEditor := typesMap.Edit()
	updated, err := mapEditor.Set(dRow.NomsMapKey(t.sch), dRow.NomsMapValue(t.sch)).Map(ctx)
	if err != nil {
		return false, errhand.BuildDError("error: failed to modify table").AddCause(err).Build()
	}
	newTable, err := t.table.UpdateRows(ctx, updated)
	if err != nil {
		return false, errhand.BuildDError("error: failed to update rows").AddCause(err).Build()
	}
	newRoot, err := t.db.root.PutTable(ctx, t.db.root.VRW(), t.name, newTable)
	if err != nil {
		return false, errhand.BuildDError("error: failed to write table back to database").AddCause(err).Build()
	}
	t.table = newTable
	t.db.root = newRoot
	return true, nil
}

// doltTablePartitionIter, an object that knows how to return the single partition exactly once.
type doltTablePartitionIter struct {
	sql.PartitionIter
	i int
}

// Close is required by the sql.PartitionIter interface. Does nothing.
func (itr *doltTablePartitionIter) Close() error {
	return nil
}

// Next returns the next partition if there is one, or io.EOF if there isn't.
func (itr *doltTablePartitionIter) Next() (sql.Partition, error) {
	if itr.i > 0 {
		return nil, io.EOF
	}
	itr.i++

	return &doltTablePartition{}, nil
}

// A table partition, currently an unused layer of abstraction but required for the framework.
type doltTablePartition struct {
	sql.Partition
}

const partitionName = "single"

// Key returns the key for this partition, which must uniquely identity the partition. We have only a single partition
// per table, so we use a constant.
func (p doltTablePartition) Key() []byte {
	return []byte(partitionName)
}
