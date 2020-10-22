package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)
	
func validateProfileParams(q map[string][]string,) (ProfileParams, error) {
	p := ProfileParams{}

	// Login
	if params, isExist := q["login"]; isExist {
		p.Login = params[0];
	}

	if p.Login == "" {
		return p, errors.New("login must me not empty")
	}

	return p, nil
}

func validateCreateParams(q map[string][]string,) (CreateParams, error) {
	p := CreateParams{}

	// Login
	if params, isExist := q["login"]; isExist {
		p.Login = params[0];
	}

	if p.Login == "" {
		return p, errors.New("login must me not empty")
	}

	if len(p.Login) < 10 {
		return p, errors.New("login len must be >= 10")
	}

	// Name
	if params, isExist := q["full_name"]; isExist {
		p.Name = params[0];
	}

	// Status
	if params, isExist := q["status"]; isExist {
		p.Status = params[0];
	}

	if p.Status == "" {
		p.Status = "user"
	}

	isAllowed := false
	allowedVals := []string{"user", "moderator", "admin"}
	for _, val := range allowedVals {
		if p.Status == val {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return p, errors.New("status must be one of [user, moderator, admin]")
	}

	// Age
	if params, isExist := q["age"]; isExist {
		n, err := strconv.Atoi(params[0])
		if err != nil{
			return p, errors.New("age must be int")
		}

		p.Age = n
	}

	if p.Age < 0 {
		return p, errors.New("age must be >= 0")
	}

	if p.Age > 128 {
		return p, errors.New("age must be <= 128")
	}

	return p, nil
}

func (s *MyApi) handleProfile(w http.ResponseWriter, r *http.Request) {
	query := make(map[string][]string)
	if r.Method == "POST" {
		defer r.Body.Close()
		bodyRaw, _ := ioutil.ReadAll(r.Body)
		body := string(bodyRaw)
	
		for _, param := range strings.Split(body, "&") {
			splittedParam := strings.Split(param, "=")
			if len(splittedParam) > 1 {
				query[splittedParam[0]] = append(query[splittedParam[0]], splittedParam[1])
			}
		}
	} else if r.Method == "GET" {
		query = r.URL.Query()
	}

	params, err := validateProfileParams(query)
	if err != nil {
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: err.Error(),
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errRaw)

		return
	}

	ctx := context.Background()
	result, err := s.Profile(ctx, params)
	if err != nil {
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: err.Error(),
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		if apiErr, isOk := err.(ApiError); isOk {
			w.WriteHeader(apiErr.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(errRaw)

		return
	}

	response := struct{
		Err string `json:"error"`
		Result interface{} `json:"response"`
	}{
		Err: "",
		Result: result,
	}

	responseRaw, _ := json.Marshal(response)
	w.WriteHeader(http.StatusOK)
	w.Write(responseRaw)
}

func (s *MyApi) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: "bad method",
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write(errRaw)

		return
	}
	
	defer r.Body.Close()
	bodyRaw, _ := ioutil.ReadAll(r.Body)
	body := string(bodyRaw)

	query := make(map[string][]string)
	for _, param := range strings.Split(body, "&") {
		splittedParam := strings.Split(param, "=")
		if len(splittedParam) > 1 {
			query[splittedParam[0]] = append(query[splittedParam[0]], splittedParam[1])
		}
	}

	authToken := r.Header.Get("X-Auth")
	if authToken == "" {
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: "unauthorized",
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		w.WriteHeader(http.StatusForbidden)
		w.Write(errRaw)

		return
	}

	params, err := validateCreateParams(query)
	if err != nil {
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: err.Error(),
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errRaw)

		return
	}

	ctx := context.Background()
	result, err := s.Create(ctx, params)
	if err != nil {
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: err.Error(),
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		if apiErr, isOk := err.(ApiError); isOk {
			w.WriteHeader(apiErr.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(errRaw)

		return
	}

	response := struct{
		Err string `json:"error"`
		Result interface{} `json:"response"`
	}{
		Err: "",
		Result: result,
	}

	responseRaw, _ := json.Marshal(response)
	w.WriteHeader(http.StatusOK)
	w.Write(responseRaw)
}

func validateOtherCreateParams(q map[string][]string,) (OtherCreateParams, error) {
	p := OtherCreateParams{}

	// Username
	if params, isExist := q["username"]; isExist {
		p.Username = params[0];
	}

	if p.Username == "" {
		return p, errors.New("username must me not empty")
	}

	if len(p.Username) < 3 {
		return p, errors.New("username len must be >= 3")
	}

	// Name
	if params, isExist := q["account_name"]; isExist {
		p.Name = params[0];
	}

	// Class
	if params, isExist := q["class"]; isExist {
		p.Class = params[0];
	}

	if p.Class == "" {
		p.Class = "warrior"
	}

	isAllowed := false
	allowedVals := []string{"warrior", "sorcerer", "rouge"}
	for _, val := range allowedVals {
		if p.Class == val {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return p, errors.New("class must be one of [warrior, sorcerer, rouge]")
	}

	// Level
	if params, isExist := q["level"]; isExist {
		n, err := strconv.Atoi(params[0])
		if err != nil{
			return p, errors.New("level must be int")
		}

		p.Level = n
	}

	if p.Level < 1 {
		return p, errors.New("level must be >= 1")
	}

	if p.Level > 50 {
		return p, errors.New("level must be <= 50")
	}

	return p, nil
}

func (s *OtherApi) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: "bad method",
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		w.WriteHeader(http.StatusNotAcceptable)
		w.Write(errRaw)

		return
	}
	
	defer r.Body.Close()
	bodyRaw, _ := ioutil.ReadAll(r.Body)
	body := string(bodyRaw)

	query := make(map[string][]string)
	for _, param := range strings.Split(body, "&") {
		splittedParam := strings.Split(param, "=")
		if len(splittedParam) > 1 {
			query[splittedParam[0]] = append(query[splittedParam[0]], splittedParam[1])
		}
	}

	authToken := r.Header.Get("X-Auth")
	if authToken == "" {
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: "unauthorized",
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		w.WriteHeader(http.StatusForbidden)
		w.Write(errRaw)

		return
	}

	params, err := validateOtherCreateParams(query)
	if err != nil {
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: err.Error(),
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errRaw)

		return
	}

	ctx := context.Background()
	result, err := s.Create(ctx, params)
	if err != nil {
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: err.Error(),
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		if apiErr, isOk := err.(ApiError); isOk {
			w.WriteHeader(apiErr.HTTPStatus)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		w.Write(errRaw)

		return
	}

	response := struct{
		Err string `json:"error"`
		Result interface{} `json:"response"`
	}{
		Err: "",
		Result: result,
	}

	responseRaw, _ := json.Marshal(response)
	w.WriteHeader(http.StatusOK)
	w.Write(responseRaw)
}

func (s *MyApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/profile":
		s.handleProfile(w, r)
	case "/user/create":
		s.handleCreate(w, r)
	default:
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: "unknown method",
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		w.WriteHeader(http.StatusNotFound)
		w.Write(errRaw)
	}
}

func (s *OtherApi) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/user/create":
		s.handleCreate(w, r)
	default:
		handlerErr := struct {
			Err string `json:"error"`
		}{
			Err: "unknown method",
		}
		errRaw, _ := json.Marshal(handlerErr)
		
		w.WriteHeader(http.StatusNotFound)
		w.Write(errRaw)
	}
}

