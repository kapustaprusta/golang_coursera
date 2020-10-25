package main

import (
	"database/sql"
	"fmt"
	"net/http"
)

// DbExplorer ...
type DbExplorer struct {
	tablesNames []string
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

		explorer.tablesNames = append(explorer.tablesNames, string(tableName.([]byte)))
	}
	rows.Close()

	for _, tableName := range explorer.tablesNames {
		rows, err = db.Query(fmt.Sprintf("SHOW FULL COLUMNS FROM %s;", tableName))
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
				fmt.Println(string(*val))
			}
		}
		rows.Close()
	}

	return explorer, nil
}

// ServeHTTP ...
func (e *DbExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {

}
