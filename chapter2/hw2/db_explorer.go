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
	Default string // NULL
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
			valDefault := vals[5].(*sql.RawBytes)
			valExtra, _ := vals[6].(*sql.RawBytes)

			model.Fields[table] = append(model.Fields[table], Field{
				Name:    string(*valName),
				Type:    string(*valType),
				Key:     string(*valKey),
				Extra:   string(*valExtra),
				Null:    string(*valIsNull),
				Default: string(*valDefault),
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

func (e *DbExplorer) createEntry(name string, vals map[string]interface{}) (map[string]interface{}, error) {
	fields := e.model.Fields[name]
	for _, field := range fields {
		if val, isExist := vals[field.Name]; isExist {
			// check type
			if field.Type == "int" {
				if _, isOk := val.(float64); !isOk {
					// check NULL
					if field.Null == "NO" {
						if val == nil {
							return nil, fmt.Errorf("field %s must not be NULL", field.Name)
						}
					}

					return nil, fmt.Errorf("field %s have invalid type", field.Name)
				}
			} else {
				if _, isOk := val.(string); !isOk {
					// check NULL
					if field.Null == "NO" {
						if val == nil {
							return nil, fmt.Errorf("field %s must not be NULL", field.Name)
						}
					}

					return nil, fmt.Errorf("field %s have invalid type", field.Name)
				}
			}

			// check KEY and Extra
			if field.Key == "PRI" && field.Extra == "auto_increment" {
				delete(vals, field.Name)
			}
		} else {
			if field.Default == "" && field.Null == "NO" {
				if field.Type == "int" {
					vals[field.Name] = 0
				} else {
					vals[field.Name] = ""
				}
			}
		}
	}

	for valName := range vals {
		isFound := false
		for _, field := range e.model.Fields[name] {
			if field.Name == valName {
				isFound = true
				break
			}
		}

		if !isFound {
			delete(vals, valName)
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

	primaryKey := ""
	for _, fieldMetaInfo := range e.model.Fields[name] {
		if fieldMetaInfo.Key == "PRI" {
			primaryKey = fieldMetaInfo.Name
			break
		}
	}

	return map[string]interface{}{primaryKey: lastID}, nil
}

func (e *DbExplorer) handleTable(w http.ResponseWriter, r *http.Request) {
	var result map[string]interface{}
	tableName := strings.ReplaceAll(r.URL.Path, "/", "")

	switch r.Method {
	case http.MethodGet:
		limit := 5
		var err error
		if limitVals, isExist := r.URL.Query()["limit"]; isExist {
			limit, err = strconv.Atoi(limitVals[0])
			if err != nil {
				limit = 5
			}
		}
		offset := 0
		if offsetVals, isExist := r.URL.Query()["offset"]; isExist {
			offset, _ = strconv.Atoi(offsetVals[0])
		}

		result, err = e.getEntries(tableName, limit, offset)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	case http.MethodPut:
		valsRaw, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		valsJSON := make(map[string]interface{})
		err = json.Unmarshal(valsRaw, &valsJSON)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		result, err = e.createEntry(tableName, valsJSON)
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

func (e *DbExplorer) getEntry(name string, id int) (map[string]interface{}, error) {
	primaryKey := ""
	for _, fieldMetaInfo := range e.model.Fields[name] {
		if fieldMetaInfo.Key == "PRI" {
			primaryKey = fieldMetaInfo.Name
			break
		}
	}

	result := e.db.QueryRow(fmt.Sprintf("SELECT * FROM %s WHERE %s = ?;", name, primaryKey), id)
	valsLen := len(e.model.Fields[name])
	record := make(map[string]interface{})
	vals := make([]interface{}, valsLen)
	for i := 0; i < len(vals); i++ {
		var tmp interface{}
		vals[i] = &tmp
	}

	if err := result.Scan(vals...); err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, fmt.Errorf("record not found")
		}

		return nil, err
	}

	for idx, val := range vals {
		val, _ := val.(*interface{})
		if e.model.Fields[name][idx].Type == "int" {
			if valRaw, isOk := (*val).(int64); isOk {
				record[e.model.Fields[name][idx].Name] = valRaw
			}
		} else {
			if valRaw, isOk := (*val).([]byte); isOk {
				if len(valRaw) == 0 {
					record[e.model.Fields[name][idx].Name] = valRaw
				} else {
					record[e.model.Fields[name][idx].Name] = string(valRaw)
				}
			} else {
				record[e.model.Fields[name][idx].Name] = val
			}
		}
	}

	return map[string]interface{}{"record": record}, nil
}

func (e *DbExplorer) upadteEntry(name string, id int, vals map[string]interface{}) (map[string]interface{}, error) {
	fieldsMetaInfo := e.model.Fields[name]
	for _, field := range fieldsMetaInfo {
		if val, isExist := vals[field.Name]; isExist {
			// check primary key
			if field.Key == "PRI" {
				return nil, fmt.Errorf("field %s have invalid type", field.Name)
			}

			// check type
			if field.Type == "int" {
				if _, isOk := val.(float64); !isOk {
					return nil, fmt.Errorf("field %s have invalid type", field.Name)
				}
			} else {
				if _, isOk := val.(string); !isOk {
					if val != nil || field.Null == "NO" {
						return nil, fmt.Errorf("field %s have invalid type", field.Name)
					}
				}
			}
		}
	}

	primaryKey := ""
	for _, fieldMetaInfo := range e.model.Fields[name] {
		if fieldMetaInfo.Key == "PRI" {
			primaryKey = fieldMetaInfo.Name
			break
		}
	}

	rowsNames := ""
	placeholdersCount := 0
	var placeholdersVals []interface{}

	for cellName, cellVal := range vals {
		rowsNames += cellName + " = ?"
		if placeholdersCount != len(vals)-1 {
			rowsNames += ", "
		}

		placeholdersVals = append(placeholdersVals, cellVal)
		placeholdersCount++
	}
	placeholdersVals = append(placeholdersVals, id)

	queryStr := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?;", name, rowsNames, primaryKey)
	result, err := e.db.Exec(queryStr, placeholdersVals...)
	if err != nil {
		return nil, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{"updated": rowsAffected}, nil
}

func (e *DbExplorer) deleteEntry(name string, id int) (map[string]interface{}, error) {
	primaryKey := ""
	for _, fieldMetaInfo := range e.model.Fields[name] {
		if fieldMetaInfo.Key == "PRI" {
			primaryKey = fieldMetaInfo.Name
			break
		}
	}

	result, err := e.db.Exec(fmt.Sprintf("DELETE FROM %s WHERE %s = ?", name, primaryKey), id)
	if err != nil {
		return nil, err
	}

	rowsAffected, _ := result.RowsAffected()

	return map[string]interface{}{
		"deleted": rowsAffected,
	}, nil
}

func (e *DbExplorer) handleEntry(w http.ResponseWriter, r *http.Request) {
	splittedURL := strings.Split(r.URL.Path, "/")[1:]
	tableName := splittedURL[0]
	recordID, _ := strconv.Atoi(splittedURL[1])
	var result map[string]interface{}

	switch r.Method {
	case http.MethodGet:
		var err error
		result, err = e.getEntry(tableName, recordID)
		if err != nil {
			if err.Error() == "record not found" {
				w.WriteHeader(http.StatusNotFound)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}

			responseErr := map[string]interface{}{"error": err.Error()}
			responseErrJSON, _ := json.Marshal(responseErr)
			w.Write(responseErrJSON)

			return
		}
	case http.MethodPost:
		valsRaw, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		valsJSON := make(map[string]interface{})
		err = json.Unmarshal(valsRaw, &valsJSON)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		result, err = e.upadteEntry(tableName, recordID, valsJSON)
		if err != nil {
			if strings.Contains(err.Error(), "have invalid type") {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				w.WriteHeader(http.StatusInternalServerError)
			}

			responseErr := map[string]interface{}{"error": err.Error()}
			responseErrJSON, _ := json.Marshal(responseErr)
			w.Write(responseErrJSON)

			return
		}
	case http.MethodDelete:
		var err error
		result, err = e.deleteEntry(tableName, recordID)
		if err != err {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	default:
		responseErr := map[string]interface{}{
			"error": "unknown table",
		}
		responseErrJSON, _ := json.Marshal(responseErr)
		w.WriteHeader(http.StatusNotFound)
		w.Write(responseErrJSON)

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

// ServeHTTP ...
func (e *DbExplorer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	if urlPath == "/" {
		e.handleMain(w, r)
	} else if e.entryRegexp.MatchString(urlPath) {
		e.handleEntry(w, r)
	} else if e.tableRegexp.MatchString(urlPath) {
		e.handleTable(w, r)
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
