package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Field ...
type Field struct {
	Name    string
	Type    string // int|text|varchar(255)
	Key     string // PRI
	Extra   string // auto_increment
	Null    string // YES|NO
	Default interface{}
}

// Model ...
type Model struct {
	Tables []string           `json:"tables"`
	Fields map[string][]Field `json:"-"`
}

// DbExplorer ...
type DbExplorer struct {
	db          *sql.DB
	model       Model
	tableRegexp *regexp.Regexp
	entryRegexp *regexp.Regexp
}

// NewDbExplorer ...
func NewDbExplorer(db *sql.DB) (*DbExplorer, error) {
	rows, err := db.Query("SHOW TABLES;")
	if err != nil {
		return nil, err
	}

	var model Model
	for rows.Next() {
		var val interface{}
		err = rows.Scan(&val)
		if err != nil {
			return nil, err
		}

		table, _ := val.([]byte)
		model.Tables = append(model.Tables, string(table))
	}
	rows.Close()

	model.Fields = make(map[string][]Field)
	for _, table := range model.Tables {
		rows, err = db.Query(fmt.Sprintf("SHOW FULL COLUMNS FROM %s;", table))
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

			valName, _ := vals[0].(*sql.RawBytes)
			valType, _ := vals[1].(*sql.RawBytes)
			valIsNull, _ := vals[3].(*sql.RawBytes)
			valKey, _ := vals[4].(*sql.RawBytes)
			valDefault := vals[5]
			valExtra, _ := vals[6].(*sql.RawBytes)

			model.Fields[table] = append(model.Fields[table], Field{
				Name:    string(*valName),
				Type:    string(*valType),
				Key:     string(*valKey),
				Extra:   string(*valExtra),
				Null:    string(*valIsNull),
				Default: valDefault,
			})
		}
		rows.Close()
	}

	return &DbExplorer{
		db:          db,
		model:       model,
		tableRegexp: regexp.MustCompile(fmt.Sprintf("/(%s)", strings.Join(model.Tables, "|"))),
		entryRegexp: regexp.MustCompile(fmt.Sprintf("/(%s)/[0-9]+", strings.Join(model.Tables, "|"))),
	}, nil
}

func (e *DbExplorer) handleMain(w http.ResponseWriter, r *http.Request) {
	tables := map[string]interface{}{"tables": e.model.Tables}
	response := map[string]interface{}{"response": tables}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}

func (e *DbExplorer) getEntries(name string, limit int, offset int) (map[string]interface{}, error) {
	result, err := e.db.Query(fmt.Sprintf("SELECT * FROM %s LIMIT %d OFFSET %d;", name, limit, offset))
	if err != nil {
		return nil, err
	}

	cols, _ := result.Columns()
	vals := make([]interface{}, len(cols))
	for i := range vals {
		vals[i] = &sql.RawBytes{}
	}

	var records []map[string]interface{}
	for result.Next() {
		err = result.Scan(vals...)
		if err != nil {
			return nil, err
		}

		record := make(map[string]interface{})
		for idx, val := range vals {
			if valRaw, isOk := val.(*sql.RawBytes); isOk {
				if e.model.Fields[name][idx].Type == "int" {
					number, _ := strconv.Atoi(string(*valRaw))
					record[e.model.Fields[name][idx].Name] = number
				} else {
					if len(*valRaw) == 0 {
						record[e.model.Fields[name][idx].Name] = valRaw
					} else {
						record[e.model.Fields[name][idx].Name] = string(*valRaw)
					}
				}
			}
		}

		records = append(records, record)
	}

	return map[string]interface{}{"records": records}, nil
}

func (e *DbExplorer) addEntry(name string, vals map[string]interface{}) (map[string]interface{}, error) {
	fieldsMetaInfo := e.model.Fields[name]
	for _, metaInfo := range fieldsMetaInfo {
		if val, isExist := vals[metaInfo.Name]; isExist {
			// check type
			if metaInfo.Type == "int" {
				if _, isOk := val.(float64); !isOk {
					// check NULL
					if metaInfo.Null == "NO" {
						if val == nil {
							return nil, fmt.Errorf("field %s must not be NULL", metaInfo.Name)
						}
					}

					return nil, fmt.Errorf("field %s have invalid type", metaInfo.Name)
				}
			} else {
				if _, isOk := val.(string); !isOk {
					// check NULL
					if metaInfo.Null == "NO" {
						if val == nil {
							return nil, fmt.Errorf("field %s must not be NULL", metaInfo.Name)
						}
					}

					return nil, fmt.Errorf("field %s have invalid type", metaInfo.Name)
				}
			}

			// check KEY and Extra
			if metaInfo.Key == "PRI" && metaInfo.Extra == "auto_increment" {
				delete(vals, metaInfo.Name)
			}
		}
	}

	rowsNames := ""
	placeholders := ""
	placeholdersCount := 0
	placeholdersVals := make([]interface{}, len(vals))

	for cellName, cellVal := range vals {
		rowsNames += cellName
		placeholders += "?"
		if placeholdersCount != len(vals)-1 {
			rowsNames += ", "
			placeholders += ", "
		}

		placeholdersVals[placeholdersCount] = cellVal
		placeholdersCount++
	}

	queryStr := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);", name, rowsNames, placeholders)
	result, err := e.db.Exec(queryStr, placeholdersVals...)
	if err != nil {
		return nil, err
	}

	lastID, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{"id": lastID}, nil
}

func (e *DbExplorer) handleTable(w http.ResponseWriter, r *http.Request) {
	var result map[string]interface{}
	tableName := strings.ReplaceAll(r.URL.Path, "/", "")

	switch r.Method {
	case http.MethodGet:
		limit := 5
		if limitVals, isExist := r.URL.Query()["limit"]; isExist {
			limit, _ = strconv.Atoi(limitVals[0])
		}
		offset := 0
		if offsetVals, isExist := r.URL.Query()["offset"]; isExist {
			offset, _ = strconv.Atoi(offsetVals[0])
		}

		var err error
		result, err = e.getEntries(tableName, limit, offset)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case http.MethodPut:
		bodyRaw, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		bodyJSON := make(map[string]interface{})
		err = json.Unmarshal(bodyRaw, &bodyJSON)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		result, err = e.addEntry(tableName, bodyJSON)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	default:
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{"response": result}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(responseJSON)
}

func (e *DbExplorer) handleEntry(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.URL.Path)
}

// ServeHTTP ...
func (e *DbExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	if urlPath == "/" {
		e.handleMain(w, r)
	} else if e.tableRegexp.MatchString(urlPath) {
		e.handleTable(w, r)
	} else if e.entryRegexp.MatchString(urlPath) {
		e.handleEntry(w, r)
	} else {
		response := map[string]interface{}{"error": "unknown table"}
		responseJSON, err := json.Marshal(response)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNotFound)
		w.Write(responseJSON)
	}
}
