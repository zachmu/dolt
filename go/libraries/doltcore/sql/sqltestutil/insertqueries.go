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

package sqltestutil

import (
	"github.com/liquidata-inc/dolt/go/libraries/doltcore/row"
	"github.com/liquidata-inc/dolt/go/libraries/doltcore/schema"
	"github.com/liquidata-inc/dolt/go/store/types"
)

// Structure for a test of a insert query
type InsertTest struct {
	// The name of this test. Names should be unique and descriptive.
	Name string
	// The insert query to run
	InsertQuery string
	// The select query to run to verify the results
	SelectQuery string
	// The schema of the result of the query, nil if an error is expected
	ExpectedSchema schema.Schema
	// The rows this query should return, nil if an error is expected
	ExpectedRows []row.Row
	// An expected error string
	ExpectedErr string
	// Setup logic to run before executing this test, after initial tables have been created and populated
	AdditionalSetup SetupFn
	// Whether to skip this test on SqlEngine (go-mysql-server) execution.
	// Over time, this should become false for every query.
	SkipOnSqlEngine bool
}

// BasicSelectTests cover basic select statement features and error handling
var BasicInsertTests = []InsertTest{
	{
		Name:           "insert no columns",
		InsertQuery:    "insert into people values (2, 'Bart', 'Simpson', false, 10, 9, '00000000-0000-0000-0000-000000000002', 222)",
		SelectQuery:    "select * from people where id = 2",
		ExpectedRows:   CompressRows(PeopleTestSchema, Bart),
		ExpectedSchema: CompressSchema(PeopleTestSchema),
	},
	{
		Name:        "insert no columns too few values",
		InsertQuery: "insert into people values (2, 'Bart', 'Simpson', false, 10, 9, '00000000-0000-0000-0000-000000000002')",
		ExpectedErr: "too few values",
	},
	{
		Name:        "insert no columns too many values",
		InsertQuery: "insert into people values (2, 'Bart', 'Simpson', false, 10, 9, '00000000-0000-0000-0000-000000000002', 222, 'abc')",
		ExpectedErr: "too many values",
	},
	{
		Name:           "insert full columns",
		InsertQuery:    "insert into people (id, first, last, is_married, age, rating, uuid, num_episodes) values (2, 'Bart', 'Simpson', false, 10, 9, '00000000-0000-0000-0000-000000000002', 222)",
		SelectQuery:    "select * from people where id = 2",
		ExpectedRows:   CompressRows(PeopleTestSchema, Bart),
		ExpectedSchema: CompressSchema(PeopleTestSchema),
	},
	{
		Name:           "insert full columns mixed order",
		InsertQuery:    "insert into people (num_episodes, uuid, rating, age, is_married, last, first, id) values (222, '00000000-0000-0000-0000-000000000002', 9, 10, false, 'Simpson', 'Bart', 2)",
		SelectQuery:    "select * from people where id = 2",
		ExpectedRows:   CompressRows(PeopleTestSchema, Bart),
		ExpectedSchema: CompressSchema(PeopleTestSchema),
	},
	{
		Name:           "insert partial columns",
		InsertQuery:    "insert into people (id, first, last) values (2, 'Bart', 'Simpson')",
		SelectQuery:    "select id, first, last from people where id = 2",
		ExpectedRows:   Rs(NewResultSetRow(types.Int(2), types.String("Bart"), types.String("Simpson"))),
		ExpectedSchema: NewResultSetSchema("id", types.IntKind, "first", types.StringKind, "last", types.StringKind),
	},
	{
		Name:           "insert partial columns mixed order",
		InsertQuery:    "insert into people (last, first, id) values ('Simpson', 'Bart', 2)",
		SelectQuery:    "select id, first, last from people where id = 2",
		ExpectedRows:   Rs(NewResultSetRow(types.Int(2), types.String("Bart"), types.String("Simpson"))),
		ExpectedSchema: NewResultSetSchema("id", types.IntKind, "first", types.StringKind, "last", types.StringKind),
	},
	{
		Name:        "insert missing non-nullable column",
		InsertQuery: "insert into people (id, first) values (2, 'Bart')",
		ExpectedErr: "column <last> received nil but is non-nullable",
	},
	{
		Name:        "insert partial columns mismatch too many values",
		InsertQuery: "insert into people (id, first, last) values (2, 'Bart', 'Simpson', false)",
		ExpectedErr: "too many values",
	},
	{
		Name:        "insert partial columns mismatch too few values",
		InsertQuery: "insert into people (id, first, last) values (2, 'Bart')",
		ExpectedErr: "too few values",
	},
	{
		Name:           "insert partial columns functions",
		InsertQuery:    "insert into people (id, first, last) values (2, UPPER('Bart'), 'Simpson')",
		SelectQuery:    "select id, first, last from people where id = 2",
		ExpectedRows:   Rs(NewResultSetRow(types.Int(2), types.String("BART"), types.String("Simpson"))),
		ExpectedSchema: NewResultSetSchema("id", types.IntKind, "first", types.StringKind, "last", types.StringKind),
	},
	{
		Name:           "insert partial columns multiple values",
		InsertQuery:    "insert into people (id, first, last) values (0, 'Bart', 'Simpson'), (1, 'Homer', 'Simpson')",
		SelectQuery:    "select id, first, last from people where id < 2 order by id",
		ExpectedRows:   Rs(NewResultSetRow(types.Int(0), types.String("Bart"), types.String("Simpson")),
			NewResultSetRow(types.Int(1), types.String("Homer"), types.String("Simpson"))),
		ExpectedSchema: NewResultSetSchema("id", types.IntKind, "first", types.StringKind, "last", types.StringKind),
	},
	{
		Name:        "insert partial columns existing pk",
		InsertQuery: "insert into people (id, first, last) values (2, 'Bart', 'Simpson'), (2, 'Bart', 'Simpson')",
		ExpectedErr: "duplicate primary key",
	},
}