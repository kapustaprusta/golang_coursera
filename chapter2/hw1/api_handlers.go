package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
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
	params, err := validateProfileParams(r.URL.Query())
	if err != nil {
		errJSON := fmt.Sprintf("{error: \"%s\"}", err)
		errRaw, _ := json.Marshal(errJSON)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errRaw)

		return
	}

	ctx := context.Background()
	res, err := s.Profile(ctx, params)
	if err != nil {
		errJSON := fmt.Sprintf("{error: \"%s\"}", err)
		errRaw, _ := json.Marshal(errJSON)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errRaw)

		return
	}

	resRaw, _ := json.Marshal(res)
	w.WriteHeader(http.StatusOK)
	w.Write(resRaw)
}

func (s *MyApi) handleCreate(w http.ResponseWriter, r *http.Request) {
	params, err := validateCreateParams(r.URL.Query())
	if err != nil {
		errJSON := fmt.Sprintf("{error: \"%s\"}", err)
		errRaw, _ := json.Marshal(errJSON)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errRaw)

		return
	}

	ctx := context.Background()
	res, err := s.Create(ctx, params)
	if err != nil {
		errJSON := fmt.Sprintf("{error: \"%s\"}", err)
		errRaw, _ := json.Marshal(errJSON)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errRaw)

		return
	}

	resRaw, _ := json.Marshal(res)
	w.WriteHeader(http.StatusOK)
	w.Write(resRaw)
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
	params, err := validateOtherCreateParams(r.URL.Query())
	if err != nil {
		errJSON := fmt.Sprintf("{error: \"%s\"}", err)
		errRaw, _ := json.Marshal(errJSON)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errRaw)

		return
	}

	ctx := context.Background()
	res, err := s.Create(ctx, params)
	if err != nil {
		errJSON := fmt.Sprintf("{error: \"%s\"}", err)
		errRaw, _ := json.Marshal(errJSON)
		w.WriteHeader(http.StatusBadRequest)
		w.Write(errRaw)

		return
	}

	resRaw, _ := json.Marshal(res)
	w.WriteHeader(http.StatusOK)
	w.Write(resRaw)
}

