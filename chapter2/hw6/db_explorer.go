package main

import (
	"database/sql"
	"fmt"
	"net/http"
)

type field struct {
	name string
}

type table struct {
	name   string
	fields []field
}

// DbExplorer ...
type DbExplorer struct {
	tables []table
}

// NewDbExplorer ...
func NewDbExplorer(db *sql.DB) (*DbExplorer, error) {
	explorer := &DbExplorer{}
	rows, err := db.Query("SHOW TABLES;")
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var tableName interface{}
		err = rows.Scan(&tableName)
		if err != nil {
			return nil, err
		}

		explorer.tables = append(explorer.tables, table{
			name: string(tableName.([]byte)),
		})
	}
	rows.Close()

	for currTableIdx, currTable := range explorer.tables {
		rows, err = db.Query(fmt.Sprintf("SHOW FULL COLUMNS FROM %s;", currTable.name))
		if err != nil {
			return nil, err
		}

		cols, _ := rows.Columns()
		vals := make([]interface{}, len(cols))
		for i := range cols {
			vals[i] = &sql.RawBytes{}
		}

		for rows.Next() {
			err = rows.Scan(vals...)
			if err != nil {
				return nil, err
			}

			if val, isOk := vals[0].(*sql.RawBytes); isOk {
				explorer.tables[currTableIdx].fields = append(explorer.tables[currTableIdx].fields, field{
					name: string(*val),
				})
			}
		}
		rows.Close()
	}

	return explorer, nil
}

// ServeHTTP ...
func (e *DbExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/":
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}
