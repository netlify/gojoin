package api

import (
	"net/http"
	"testing"

	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"

	"io/ioutil"

	"os"

	"net/http/httptest"

	"fmt"

	"encoding/json"

	"bytes"

	"context"

	"github.com/netlify/netlify-subscriptions/conf"
	"github.com/netlify/netlify-subscriptions/models"
)

var db *gorm.DB
var config *conf.Config
var api *API

var serverURL string
var client *http.Client

var testUserID = "joker"
var testUserEmail = "joker@dc.com"

func TestMain(m *testing.M) {
	f, err := ioutil.TempFile("", "test-db")
	if err != nil {
		panic(err)
	}
	defer os.Remove(f.Name())

	config = &conf.Config{
		AdminGroupName: "admin",
		JWTSecret:      "secret",
		DBConfig: conf.DBConfig{
			Automigrate: true,
			Namespace:   "test",
			Driver:      "sqlite3",
			ConnURL:     f.Name(),
		},
	}
	db, err = models.Connect(&config.DBConfig)

	if err != nil {
		fmt.Println("Failed to connect to db")
		os.Exit(1)
	}

	api = NewAPI(config, db, errorProxy{}, "test")
	server := httptest.NewServer(api.handler)
	defer server.Close()

	serverURL = server.URL
	client = new(http.Client)

	os.Exit(m.Run())
}

func TestTokenExtraction(t *testing.T) {
	tokenString := testToken(t, testUserID, testUserEmail, config.JWTSecret, true)
	r, _ := http.NewRequest("GET", "http://doesnotmatter", nil)
	r.Header.Add("Authorization", "Bearer "+tokenString)

	token, err := extractToken("secret", r)
	assert.Nil(t, err)
	if assert.NotNil(t, token) {
		assert.Nil(t, token.Claims.Valid())
		outClaims := token.Claims.(*JWTClaims)
		assert.Equal(t, "joker", outClaims.ID)

		foundAdmin := false
		for _, g := range outClaims.Groups {
			switch g {
			case "admin":
				foundAdmin = true
			default:
				assert.Fail(t, "unexpected group: "+g)
			}
		}
		assert.True(t, foundAdmin)
	}
}

func TestBadAuthHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", serverURL+"/subscriptions", nil)
	r.Header.Add("Authorization", "Bearer NONSENSE")

	rsp, _ := client.Do(r)
	extractError(t, http.StatusUnauthorized, rsp)
}

func TestMissingAuthHeader(t *testing.T) {
	r, _ := http.NewRequest("GET", serverURL+"/subscriptions", nil)

	rsp, _ := client.Do(r)
	extractError(t, http.StatusBadRequest, rsp)
}

func TestMiddleware(t *testing.T) {
	tokenString := testToken(t, testUserID, testUserEmail, config.JWTSecret, true)
	r, _ := http.NewRequest("GET", serverURL+"/subscriptions", nil)
	r.Header.Add("Authorization", "Bearer "+tokenString)

	ctx := context.Background()
	ctx = api.populateConfig(ctx, nil, r)

	assert.Equal(t, db, getDB(ctx))
	assert.NotEqual(t, "", getRequestID(ctx))
	assert.True(t, isAdmin(ctx))
	assert.Equal(t, config, getConfig(ctx))
	log := getLogger(ctx)

	expectedFields := map[string]bool{
		"request_id": false,
		"method":     false,
		"path":       false,
		"is_admin":   false,
		"user_id":    false,
	}

	for k, v := range log.Data {
		assert.NotEqual(t, "", v, k+" was empty")
		expectedFields[k] = true
	}
	for k, v := range expectedFields {
		assert.True(t, v, k+" is missing")
	}
}

func TestGetHello(t *testing.T) {
	rsp := request(t, "GET", "", nil, false)
	payload := make(map[string]interface{})
	extractPayload(t, rsp, &payload)

	_, exists := payload["version"]
	assert.True(t, exists)
	_, exists = payload["application"]
	assert.True(t, exists)
}

// ------------------------------------------------------------------------------------------------
// utilities
// ------------------------------------------------------------------------------------------------

func request(t *testing.T, method, path string, body interface{}, isAdmin bool) *http.Response {
	var r *http.Request
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			assert.FailNow(t, "failed to make request: "+err.Error())
		}

		r, _ = http.NewRequest(method, serverURL+path, bytes.NewBuffer(b))
	} else {
		r, _ = http.NewRequest(method, serverURL+path, nil)
	}
	tokenString := testToken(t, testUserID, testUserEmail, config.JWTSecret, isAdmin)
	r.Header.Add("Authorization", "Bearer "+tokenString)

	rsp, err := client.Do(r)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "failed to make request: "+r.URL.String())
	}

	return rsp
}

func extractPayload(t *testing.T, rsp *http.Response, payload interface{}) {
	b, _ := ioutil.ReadAll(rsp.Body)
	defer rsp.Body.Close()
	if rsp.StatusCode != http.StatusOK {
		assert.FailNow(t, fmt.Sprintf("Expected a 200 - %d: with payload: %s", rsp.StatusCode, string(b)))
	}

	err := json.Unmarshal(b, payload)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Failed to parse payload: "+string(b))
	}
}

func extractError(t *testing.T, errCode int, rsp *http.Response) *HTTPError {
	var err *HTTPError
	if assert.Equal(t, errCode, rsp.StatusCode) {
		b, _ := ioutil.ReadAll(rsp.Body)
		err = new(HTTPError)
		e := json.Unmarshal(b, err)
		if !assert.NoError(t, e) {
			assert.FailNow(t, "Failed to parse payload: "+string(b))
		}

		assert.Equal(t, errCode, err.Code)
		assert.NotEmpty(t, err.Message)
	}

	return err
}

func testToken(t *testing.T, name, email, secret string, isAdmin bool) string {
	return testTokenWithGroups(t, name, email, secret, isAdmin, []string{})
}

func testTokenWithGroups(t *testing.T, name, email, secret string, isAdmin bool, groups []string) string {
	claims := &JWTClaims{
		ID:    name,
		Email: email,
	}
	claims.ExpiresAt = time.Now().Add(time.Hour).Unix()

	if isAdmin {
		claims.Groups = []string{"admin"}
	}

	for _, g := range groups {
		claims.Groups = append(claims.Groups, g)
	}

	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if !assert.NoError(t, err) {
		assert.FailNow(t, "failed to create token")
	}
	return tokenString
}

func decodeToken(t *testing.T, jwtString, secret string) *JWTClaims {
	token, err := jwt.ParseWithClaims(jwtString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if assert.Equal(t, token.Header["alg"], signingMethod.Name) {
			return []byte(secret), nil
		}
		return nil, nil
	})
	if assert.NoError(t, err) {
		return token.Claims.(*JWTClaims)
	}
	return nil
}
