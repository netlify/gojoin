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

	"github.com/fsouza/go-dockerclient/external/golang.org/x/net/context"
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

	api = NewAPI(config, db)
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
	// TODO
}

func TestMiddleware(t *testing.T) {
	tokenString := testToken(t, testUserID, testUserEmail, config.JWTSecret, true)
	r, _ := http.NewRequest("GET", serverURL+"/", nil)
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

func request(t *testing.T, method, path string, body []byte, isAdmin bool) *http.Response {
	var r *http.Request
	if body != nil {
		r, _ = http.NewRequest(method, serverURL+path, bytes.NewBuffer(body))
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
	if rsp.StatusCode != http.StatusOK {
		assert.FailNow(t, fmt.Sprintf("Expected a 200 - %d: with payload: %s", rsp.StatusCode, string(b)))
	}

	err := json.Unmarshal(b, payload)
	if !assert.NoError(t, err) {
		assert.FailNow(t, "Failed to parse payload: "+string(b))
	}
}

func testToken(t *testing.T, name, email, secret string, isAdmin bool) string {
	claims := &JWTClaims{
		ID:    name,
		Email: email,
	}
	claims.ExpiresAt = time.Now().Add(time.Hour).Unix()

	if isAdmin {
		claims.Groups = []string{"admin"}
	}

	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if !assert.NoError(t, err) {
		assert.FailNow(t, "failed to create token")
	}
	return tokenString
}