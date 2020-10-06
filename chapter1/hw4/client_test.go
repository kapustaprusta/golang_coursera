package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

var (
	users []User

	allowedOrderFields = []string{
		"",
		"Id",
		"Age",
		"Name",
	}

	accessToken = "abcde"
)

type TestCase struct {
	Name     string
	Request  SearchRequest
	Response *SearchResponse
	Error    string
}

func init() {
	users = parseUsers("dataset.xml")
}

func parseUsers(path string) []User {
	datasetFile, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer datasetFile.Close()

	users := make([]User, 0)
	datasetDecoder := xml.NewDecoder(datasetFile)
	for {
		currToken, err := datasetDecoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			panic(err)
		}
		if currToken == nil {
			break
		}

		switch currToken := currToken.(type) {
		case xml.StartElement:
			if currToken.Name.Local == "row" {
				users = append(users, User{})
			} else if currToken.Name.Local == "id" {
				id := 0
				if err := datasetDecoder.DecodeElement(&id, &currToken); err != nil {
					panic(err)
				}
				users[len(users)-1].Id = id
			} else if currToken.Name.Local == "age" {
				age := 0
				if err := datasetDecoder.DecodeElement(&age, &currToken); err != nil {
					panic(err)
				}
				users[len(users)-1].Age = age
			} else if currToken.Name.Local == "first_name" {
				firstName := ""
				if err := datasetDecoder.DecodeElement(&firstName, &currToken); err != nil {
					panic(err)
				}
				users[len(users)-1].Name = firstName
			} else if currToken.Name.Local == "last_name" {
				lastName := ""
				if err := datasetDecoder.DecodeElement(&lastName, &currToken); err != nil {
					panic(err)
				}
				users[len(users)-1].Name += " " + lastName
			} else if currToken.Name.Local == "gender" {
				gender := ""
				if err := datasetDecoder.DecodeElement(&gender, &currToken); err != nil {
					panic(err)
				}
				users[len(users)-1].Gender += gender
			} else if currToken.Name.Local == "about" {
				about := ""
				if err := datasetDecoder.DecodeElement(&about, &currToken); err != nil {
					panic(err)
				}
				users[len(users)-1].About += about
			}
		}
	}

	return users
}

// SearchServer ...
func SearchServer(w http.ResponseWriter, r *http.Request) {
	if accessToken != r.Header.Get("AccessToken") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	// Get request params
	reqParams := r.URL.Query()
	orderField := reqParams.Get("order_field")

	// Check order field
	isAllowed := false
	for _, allowedOrderField := range allowedOrderFields {
		if orderField == allowedOrderField {
			isAllowed = true
			break
		}
	}
	if !isAllowed {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"Error": "ErrorBadOrderField"}`))
		return
	}

	// Search users
	query := reqParams.Get("query")
	limit, err := strconv.Atoi(reqParams.Get("limit"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"Error": "%s"}`, err.Error())))
		return
	}

	foundUsers := make([]User, 0)
	for _, user := range users {
		if query == "" || strings.Contains(user.Name, query) || strings.Contains(user.About, query) {
			foundUsers = append(foundUsers, user)
		}
		if len(foundUsers) == limit {
			break
		}
	}

	// Sort found users
	orderBy, err := strconv.Atoi(reqParams.Get("order_by"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"Error": "%s"}`, err.Error())))
		return
	}

	if orderBy != OrderByAsIs {
		sort.Slice(foundUsers, func(i int, j int) bool {
			if orderField == "Id" {
				if orderBy == OrderByDesc {
					return foundUsers[i].Id > foundUsers[j].Id
				}
				return foundUsers[i].Id < foundUsers[j].Id
			} else if orderField == "Age" {
				if orderBy == OrderByDesc {
					return foundUsers[i].Age > foundUsers[j].Age
				}
				return foundUsers[i].Age < foundUsers[j].Age
			}

			if orderBy == OrderByDesc {
				return foundUsers[i].Name > foundUsers[j].Name
			}
			return foundUsers[i].Name < foundUsers[j].Name
		})
	}

	// Make offset
	offset, err := strconv.Atoi(reqParams.Get("offset"))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"Error": "%s"}`, err.Error())))
		return
	}
	foundUsers = foundUsers[offset:]

	// Serialize found user
	usersJSON, err := json.Marshal(foundUsers)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf(`{"Error": "%s"}`, err.Error())))
		return
	}

	// Write found users
	w.WriteHeader(http.StatusOK)
	w.Write(usersJSON)
}

// SearchServerWithTimeout ...
func SearchServerWithTimeout(w http.ResponseWriter, r *http.Request) {
	time.Sleep(client.Timeout * 2)
	SearchServer(w, r)
}

// SearchServerWithError ...
func SearchServerWithError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
}

// SearchServerBadError ...
func SearchServerBadError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(`{"Error": ErrorBadOrderField}`))
}

// SearchServerUnknownError ...
func SearchServerUnknownError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(`{"Error": "Error"}`))
}

// SearchServerBadResponse ...
func SearchServerBadResponse(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte{})
}

func TestWrongLimits(t *testing.T) {
	testCases := []TestCase{
		{
			Name: "limit < 0",
			Request: SearchRequest{
				Limit:      -1,
				Offset:     0,
				Query:      "",
				OrderField: "",
				OrderBy:    OrderByAsIs,
			},
			Response: nil,
			Error:    "limit must be > 0",
		},
		{
			Name: "limit > 25",
			Request: SearchRequest{
				Limit:      50,
				Offset:     0,
				Query:      "Boyd Wolf",
				OrderField: "Age",
				OrderBy:    OrderByAsIs,
			},
			Response: &SearchResponse{
				Users: []User{
					{
						Id:     0,
						Name:   "Boyd Wolf",
						Age:    22,
						About:  "Nulla cillum enim voluptate consequat laborum esse excepteur occaecat commodo nostrud excepteur ut cupidatat. Occaecat minim incididunt ut proident ad sint nostrud ad laborum sint pariatur. Ut nulla commodo dolore officia. Consequat anim eiusmod amet commodo eiusmod deserunt culpa. Ea sit dolore nostrud cillum proident nisi mollit est Lorem pariatur. Lorem aute officia deserunt dolor nisi aliqua consequat nulla nostrud ipsum irure id deserunt dolore. Minim reprehenderit nulla exercitation labore ipsum.\n",
						Gender: "male",
					},
				},
				NextPage: false,
			},
			Error: "",
		},
		{
			Name: "need next page",
			Request: SearchRequest{
				Limit:      1,
				Offset:     0,
				Query:      "Dillard",
				OrderField: "Age",
				OrderBy:    OrderByAsIs,
			},
			Response: &SearchResponse{
				Users: []User{
					{
						Id:     3,
						Name:   "Everett Dillard",
						Age:    27,
						About:  "Sint eu id sint irure officia amet cillum. Amet consectetur enim mollit culpa laborum ipsum adipisicing est laboris. Adipisicing fugiat esse dolore aliquip quis laborum aliquip dolore. Pariatur do elit eu nostrud occaecat.\n",
						Gender: "male",
					},
				},
				NextPage: true,
			},
			Error: "",
		},
	}

	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := SearchClient{
		AccessToken: accessToken,
		URL:         server.URL,
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			resp, err := client.FindUsers(testCase.Request)
			if testCase.Error == "" {
				if err != nil {
					t.Fatal(err)
				}
				if resp.NextPage != testCase.Response.NextPage {
					t.Fatal("wrong response")
				}
				for userIdx := 0; userIdx < len(resp.Users); userIdx++ {
					if resp.Users[userIdx] != testCase.Response.Users[userIdx] {
						t.Fatal("wrong response")
					}
				}
			} else {
				if err.Error() != testCase.Error {
					t.Fatal("unexpected error")
				}
				if resp != nil {
					t.Fatal("unexpected response")
				}
			}
		})
	}
}

func TestWrongOffset(t *testing.T) {
	testCase := TestCase{
		Name: "offset < 0",
		Request: SearchRequest{
			Limit:      10,
			Offset:     -1,
			Query:      "",
			OrderField: "",
			OrderBy:    OrderByAsIs,
		},
		Response: nil,
		Error:    "offset must be > 0",
	}

	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := SearchClient{
		AccessToken: accessToken,
		URL:         server.URL,
	}

	t.Run(testCase.Name, func(t *testing.T) {
		resp, err := client.FindUsers(testCase.Request)
		if err.Error() != testCase.Error {
			t.Fatal("unexpected error")
		}
		if resp != nil {
			t.Fatal("unexpected response")
		}
	})
}

func TestLongTimeout(t *testing.T) {
	testCase := TestCase{
		Name: "long timeout",
		Request: SearchRequest{
			Limit:      10,
			Offset:     0,
			Query:      "Boyd Wolf",
			OrderField: "Age",
			OrderBy:    OrderByAsIs,
		},
		Response: nil,
		Error:    "timeout for limit=11&offset=0&order_by=0&order_field=Age&query=Boyd+Wolf",
	}

	server := httptest.NewServer(http.HandlerFunc(SearchServerWithTimeout))
	defer server.Close()

	client := SearchClient{
		AccessToken: accessToken,
		URL:         server.URL,
	}

	t.Run(testCase.Name, func(t *testing.T) {
		resp, err := client.FindUsers(testCase.Request)
		if err.Error() != testCase.Error {
			t.Fatal("unexpected error")
		}
		if resp != nil {
			t.Fatal("unexpected response")
		}
	})
}

func TestUnauthorized(t *testing.T) {
	testCase := TestCase{
		Name: "bad access token",
		Request: SearchRequest{
			Limit:      10,
			Offset:     0,
			Query:      "Boyd Wolf",
			OrderField: "Age",
			OrderBy:    OrderByAsIs,
		},
		Response: nil,
		Error:    "Bad AccessToken",
	}

	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := SearchClient{
		AccessToken: "",
		URL:         server.URL,
	}

	t.Run(testCase.Name, func(t *testing.T) {
		resp, err := client.FindUsers(testCase.Request)
		if err.Error() != testCase.Error {
			t.Fatal("unexpected error")
		}
		if resp != nil {
			t.Fatal("unexpected response")
		}
	})
}

func TestInternalServerError(t *testing.T) {
	testCase := TestCase{
		Name: "search server fatal error",
		Request: SearchRequest{
			Limit:      10,
			Offset:     0,
			Query:      "Boyd Wolf",
			OrderField: "Age",
			OrderBy:    OrderByAsIs,
		},
		Response: nil,
		Error:    "SearchServer fatal error",
	}

	server := httptest.NewServer(http.HandlerFunc(SearchServerWithError))
	defer server.Close()

	client := SearchClient{
		AccessToken: accessToken,
		URL:         server.URL,
	}

	t.Run(testCase.Name, func(t *testing.T) {
		resp, err := client.FindUsers(testCase.Request)
		if err.Error() != testCase.Error {
			t.Fatal("unexpected error")
		}
		if resp != nil {
			t.Fatal("unexpected response")
		}
	})
}

func TestBadRequest(t *testing.T) {
	testCase := TestCase{
		Name: "bad order field",
		Request: SearchRequest{
			Limit:      10,
			Offset:     0,
			Query:      "Boyd Wolf",
			OrderField: "About",
			OrderBy:    OrderByAsIs,
		},
		Response: nil,
		Error:    "OrderField About invalid",
	}

	server := httptest.NewServer(http.HandlerFunc(SearchServer))
	defer server.Close()

	client := SearchClient{
		AccessToken: accessToken,
		URL:         server.URL,
	}

	t.Run(testCase.Name, func(t *testing.T) {
		resp, err := client.FindUsers(testCase.Request)
		if err.Error() != testCase.Error {
			t.Fatal("unexpected error")
		}
		if resp != nil {
			t.Fatal("unexpected response")
		}
	})
}

func TestBadJson(t *testing.T) {
	testCase := TestCase{
		Name: "cant unpack error json",
		Request: SearchRequest{
			Limit:      10,
			Offset:     0,
			Query:      "Boyd Wolf",
			OrderField: "About",
			OrderBy:    OrderByAsIs,
		},
		Response: nil,
		Error:    "cant unpack error json: invalid character 'E' looking for beginning of value",
	}

	server := httptest.NewServer(http.HandlerFunc(SearchServerBadError))
	defer server.Close()

	client := SearchClient{
		AccessToken: accessToken,
		URL:         server.URL,
	}

	t.Run(testCase.Name, func(t *testing.T) {
		resp, err := client.FindUsers(testCase.Request)
		if err.Error() != testCase.Error {
			t.Fatal("unexpected error")
		}
		if resp != nil {
			t.Fatal("unexpected response")
		}
	})
}

func TestUnknownError(t *testing.T) {
	testCase := TestCase{
		Name: "unknown error",
		Request: SearchRequest{
			Limit:      10,
			Offset:     0,
			Query:      "Boyd Wolf",
			OrderField: "About",
			OrderBy:    OrderByAsIs,
		},
		Response: nil,
		Error:    "unknown bad request error: Error",
	}

	server := httptest.NewServer(http.HandlerFunc(SearchServerUnknownError))
	defer server.Close()

	client := SearchClient{
		AccessToken: accessToken,
		URL:         server.URL,
	}

	t.Run(testCase.Name, func(t *testing.T) {
		resp, err := client.FindUsers(testCase.Request)
		if err.Error() != testCase.Error {
			t.Fatal("unexpected error")
		}
		if resp != nil {
			t.Fatal("unexpected response")
		}
	})
}

func TestBadResponse(t *testing.T) {
	testCase := TestCase{
		Name: "bad response",
		Request: SearchRequest{
			Limit:      10,
			Offset:     0,
			Query:      "Boyd Wolf",
			OrderField: "About",
			OrderBy:    OrderByAsIs,
		},
		Response: nil,
		Error:    "cant unpack result json: unexpected end of JSON input",
	}

	server := httptest.NewServer(http.HandlerFunc(SearchServerBadResponse))
	defer server.Close()

	client := SearchClient{
		AccessToken: accessToken,
		URL:         server.URL,
	}

	t.Run(testCase.Name, func(t *testing.T) {
		resp, err := client.FindUsers(testCase.Request)
		if err.Error() != testCase.Error {
			t.Fatal("unexpected error")
		}
		if resp != nil {
			t.Fatal("unexpected response")
		}
	})
}

func TestBadUrl(t *testing.T) {
	testCase := TestCase{
		Name: "bad url",
		Request: SearchRequest{
			Limit:      10,
			Offset:     0,
			Query:      "Boyd Wolf",
			OrderField: "About",
			OrderBy:    OrderByAsIs,
		},
		Response: nil,
		Error:    "unknown error Get \"?limit=11&offset=0&order_by=0&order_field=About&query=Boyd+Wolf\": unsupported protocol scheme \"\"",
	}

	client := SearchClient{
		AccessToken: accessToken,
		URL:         "",
	}

	t.Run(testCase.Name, func(t *testing.T) {
		resp, err := client.FindUsers(testCase.Request)
		if err.Error() != testCase.Error {
			t.Fatal("unexpected error")
		}
		if resp != nil {
			t.Fatal("unexpected response")
		}
	})
}
