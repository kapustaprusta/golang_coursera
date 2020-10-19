package main

import (
	"context"
	"errors"
	"net/http"
)

func validateProfileParams(q map[string][]string, p *ProfileParams) error {
	if _, isExist := q["login"]; !isExist {
		return errors.New("login must me not empty")
	}
	p.Login = q["login"][0]



	return nil
}

func validateCreateParams(q map[string][]string, p *CreateParams) error {
	if _, isExist := q["login"]; !isExist {
		return errors.New("login must me not empty")
	}
	p.Login = q["login"][0]

	isAllowed := false
	allowedVals := []string{"user","moderator","admin"}
	for _, val := range allowedVals {
		if q["status"][0] == val {
			p.Status = val
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return errors.New("status must be one of [user, moderator, admin]")
	}

	p.Status = q["status"][0]
	if p.Status == "" {
		p.Status = "user"
	}

	return nil
}

func (s *MyApi) handleProfile(w http.Response, r *http.Request) {
	// Fill params
	// Validate

	ctx := context.Background()
	res, err := s.Profile(ctx, params)
}

func (s *MyApi) handleCreate(w http.Response, r *http.Request) {
	// Fill params
	// Validate

	ctx := context.Background()
	res, err := s.Create(ctx, params)
}

func validateOtherCreateParams(q map[string][]string, p *OtherCreateParams) error {
	if _, isExist := q["username"]; !isExist {
		return errors.New("username must me not empty")
	}
	p.Username = q["username"][0]

	isAllowed := false
	allowedVals := []string{"warrior","sorcerer","rouge"}
	for _, val := range allowedVals {
		if q["class"][0] == val {
			p.Class = val
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		return errors.New("class must be one of [warrior, sorcerer, rouge]")
	}

	p.Class = q["class"][0]
	if p.Class == "" {
		p.Class = "warrior"
	}

	return nil
}

func (s *OtherApi) handleCreate(w http.Response, r *http.Request) {
	// Fill params
	// Validate

	ctx := context.Background()
	res, err := s.Create(ctx, params)
}

