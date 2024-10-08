// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package delegate

import (
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/sql/lexbase"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

// ShowSequences returns all the schemas in the given or current database.
// Privileges: None.
//
//	Notes: postgres does not have a SHOW SEQUENCES statement.
func (d *delegator) delegateShowSequences(n *tree.ShowSequences) (tree.Statement, error) {
	name, err := d.getSpecifiedOrCurrentDatabase(n.Database)
	if err != nil {
		return nil, err
	}

	getSequencesQuery := fmt.Sprintf(`
	  SELECT sequence_schema, sequence_name
	    FROM %[1]s.information_schema.sequences
	   WHERE sequence_catalog = %[2]s
	ORDER BY sequence_name`,
		name.String(), // note: (tree.Name).String() != string(name)
		lexbase.EscapeSQLString(string(name)),
	)
	return d.parse(getSequencesQuery)
}
