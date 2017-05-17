package api

import (
	"testing"

	"io/ioutil"

	"net/http"

	"github.com/netlify/gojoin/models"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func TestQueryForAllSubsAsUser(t *testing.T) {
	tu1 := createUser(testUserID, testUserEmail, "some-stripe-value")
	tu2 := createUser("batman", "bruce@dc.com", "eulav-epits-emos")
	s1 := createSubscription(testUserID, "membership", "nonsense")
	s2 := createSubscription(testUserID, "revenue", "more-nonsense")
	s3 := createSubscription("batman", "membership", "nonsense")
	defer cleanup(s1, s2, s3, tu1, tu2)

	rsp := request(t, "GET", "/subscriptions", nil, false)
	body := new(getAllResponse)
	extractPayload(t, rsp, &body)

	assert.Equal(t, 2, len(body.Subscriptions))
	for _, s := range body.Subscriptions {
		switch s.ID {
		case s1.ID:
			validateSub(t, s1, &s)
		case s2.ID:
			validateSub(t, s2, &s)
		default:
			assert.Fail(t, "unexpected sub: "+s.ID)
		}
	}

	assert.NotEmpty(t, body.Token)
	claims := decodeToken(t, body.Token, config.JWTSecret)
	if assert.NotNil(t, claims) {
		meta, ok := claims["app_metadata"].(map[string]interface{})
		if !ok {
			assert.Fail(t, "No app_metadata in token")
		}
		subs, ok := meta["subscriptions"]
		if !ok {
			assert.Fail(t, "No subscriptions in metadata")
		}

		subsMap, ok := subs.(map[string]interface{})
		if !ok {
			assert.Fail(t, "Subscriptions is not a map")
		}
		assert.Equal(t, "nonsense", subsMap["membership"])
		assert.Equal(t, "more-nonsense", subsMap["revenue"])
	}
}

func TestQueryForSingleSubAsUser(t *testing.T) {
	tu := createUser(testUserID, testUserEmail, "stripe-given-value")
	s1 := createSubscription(testUserID, "membership", "nonsense")
	s2 := createSubscription(testUserID, "revenue", "more-nonsense")
	defer cleanup(s1, s2, tu)

	rsp := request(t, "GET", "/subscriptions/membership", nil, false)
	sub := new(models.Subscription)
	extractPayload(t, rsp, sub)

	validateSub(t, s1, sub)
}

func TestRemoveSubscriptionAsUser(t *testing.T) {
	tp := &testProxy{}
	api.payerProxy = tp
	tu := createUser(testUserID, testUserEmail, "stripe-given-value")
	s1 := createSubscription(testUserID, "membership", "nonsense")
	s2 := createSubscription(testUserID, "revenue", "more-nonsense")
	defer cleanup(s1, s2, tu)

	rsp := request(t, "DELETE", "/subscriptions/membership", nil, false)

	b, _ := ioutil.ReadAll(rsp.Body)
	assert.Equal(t, "{}\n", string(b))
	assert.Equal(t, 202, rsp.StatusCode)

	found := &models.Subscription{ID: s1.ID}
	dbRsp := db.Unscoped().Find(found)
	if assert.Nil(t, dbRsp.Error) {
		assert.NotNil(t, found.DeletedAt)
	}

	// validate it was removed in stripe
	assert.Len(t, tp.deleteCalls, 1)
	assert.Equal(t, s1.RemoteID, tp.deleteCalls[0])
}

func TestRemoveSubNotFound(t *testing.T) {
	rsp := request(t, "DELETE", "/subscriptions/membership", nil, false)

	b, _ := ioutil.ReadAll(rsp.Body)
	assert.Equal(t, "{}\n", string(b))
	assert.Equal(t, 202, rsp.StatusCode)
}

func TestGetSubNotFound(t *testing.T) {
	rsp := request(t, "GET", "/subscriptions/membership", nil, false)
	extractError(t, 404, rsp)
}

func TestCreateNewSubscription(t *testing.T) {
	tp := &testProxy{createSubID: "remote-id", createCustomerID: "remote-user-id"}
	api.payerProxy = tp
	defer func() { api.payerProxy = &errorProxy{} }()

	payload := &subscriptionRequest{
		StripeKey: "something",
		Plan:      "super-important",
	}
	rsp := request(t, "PUT", "/subscriptions/membership", payload, false)

	expectedSub := models.Subscription{
		Type:     "membership",
		UserID:   testUserID,
		Plan:     "super-important",
		RemoteID: "remote-id",
	}
	expectedUser := models.User{
		ID:       testUserID,
		Email:    testUserEmail,
		RemoteID: "remote-user-id",
	}

	dbRsp, dbUser := validateResponseAndDBVal(t, rsp, &expectedSub, &expectedUser)
	cleanup(dbRsp, dbUser)

	assert.Len(t, tp.createCalls, 1)
	call := tp.createCalls[0]
	assert.Equal(t, "super-important", call.plan)
	assert.Equal(t, "something", call.token)
	assert.Equal(t, "remote-user-id", call.userID)
	assert.Empty(t, tp.updateCalls)
}

func TestModifySubscription(t *testing.T) {
	tp := &testProxy{updateSubID: "remote-id"}
	api.payerProxy = tp
	defer func() { api.payerProxy = &errorProxy{} }()

	tu := createUser(testUserID, testUserEmail, "stripe-given-value")
	s1 := createSubscription(testUserID, "pokemon", "magicarp")
	defer cleanup(s1, tu)

	payload := &subscriptionRequest{
		StripeKey: "something",
		Plan:      "charizard",
	}
	rsp := request(t, "PUT", "/subscriptions/pokemon", payload, false)
	expectedSub := &models.Subscription{
		Type:     "pokemon",
		UserID:   testUserID,
		Plan:     "charizard",
		RemoteID: "remote-id",
	}
	expectedUser := &models.User{
		ID:       testUserID,
		Email:    testUserEmail,
		RemoteID: "stripe-given-value",
	}
	dbRsp, dbUser := validateResponseAndDBVal(t, rsp, expectedSub, expectedUser)
	cleanup(dbRsp, dbUser)

	assert.Len(t, tp.updateCalls, 1)
	call := tp.updateCalls[0]
	assert.Equal(t, "charizard", call.plan)
	assert.Equal(t, "something", call.token)
	assert.Equal(t, s1.RemoteID, call.subID)
	assert.Empty(t, tp.createCalls)

	assert.Len(t, tp.createCustomerCalls, 0)
}

func TestCreateNewSubscriptionWithBadPayload(t *testing.T) {
	payload := &subscriptionRequest{
		StripeKey: "something",
		Plan:      "",
	}
	rsp := request(t, "PUT", "/subscriptions/membership", payload, false)
	extractError(t, fasthttp.StatusBadRequest, rsp)

	payload.StripeKey = ""
	payload.Plan = "something"
	rsp = request(t, "PUT", "/subscriptions/membership", payload, false)
	extractError(t, fasthttp.StatusBadRequest, rsp)
}

func TestCreateNewSubscriptionWithStripeError(t *testing.T) {
	defer cleanup(createUser(testUserID, testUserEmail, "remote-id"))
	api.payerProxy = errorProxy{}
	payload := &subscriptionRequest{
		StripeKey: "something",
		Plan:      "unicorn",
	}
	rsp := request(t, "PUT", "/subscriptions/membership", payload, false)
	extractError(t, fasthttp.StatusBadRequest, rsp)
}

// ------------------------------------------------------------------------------------------------
// helpers
// ------------------------------------------------------------------------------------------------

func validateSub(t *testing.T, expected *models.Subscription, actual *models.Subscription) {
	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.UserID, actual.UserID)
	assert.Equal(t, expected.Type, actual.Type)
	assert.Equal(t, expected.RemoteID, actual.RemoteID)
	assert.Equal(t, expected.Plan, actual.Plan)

	assert.NotEmpty(t, actual.ID)
	assert.NotEmpty(t, actual.UserID)
	assert.NotEmpty(t, actual.Type)
	assert.NotEmpty(t, actual.RemoteID)
	assert.NotEmpty(t, actual.Plan)
}

func createUser(userID, email, remoteID string) *models.User {
	user := &models.User{
		Email:    email,
		ID:       userID,
		RemoteID: remoteID,
	}
	db.Create(user)
	return user
}

func createSubscription(userID, planType string, plan string) *models.Subscription {
	sub := &models.Subscription{
		UserID:   userID,
		Plan:     plan,
		Type:     planType,
		RemoteID: uuid.NewRandom().String(),
	}

	db.Create(sub)
	return sub
}

func cleanup(todelete ...interface{}) {
	for _, td := range todelete {
		db.Unscoped().Delete(td)
	}
}

type testProxy struct {
	createSubID string
	createCalls []struct {
		userID string
		plan   string
		token  string
	}
	updateSubID string
	updateCalls []struct {
		subID string
		plan  string
		token string
	}
	deleteCalls []string

	createCustomerID    string
	createCustomerCalls []struct {
		userID string
		email  string
		token  string
	}
}

func (tp *testProxy) createCustomer(userID, email, payToken string) (string, error) {
	tp.createCustomerCalls = append(tp.createCustomerCalls, struct {
		userID string
		email  string
		token  string
	}{userID, email, payToken})
	return tp.createCustomerID, nil
}

func (tp *testProxy) delete(subID string) error {
	tp.deleteCalls = append(tp.deleteCalls, subID)
	return nil
}

func (tp *testProxy) create(userID, plan, token string) (string, error) {
	tp.createCalls = append(tp.createCalls, struct {
		userID string
		plan   string
		token  string
	}{userID, plan, token})
	return tp.createSubID, nil
}

func (tp *testProxy) update(subID, plan, token string) (string, error) {
	tp.updateCalls = append(tp.updateCalls, struct {
		subID string
		plan  string
		token string
	}{subID, plan, token})
	return tp.updateSubID, nil
}

func validateResponseAndDBVal(t *testing.T, rsp *http.Response, expected *models.Subscription, expectedUser *models.User) (*models.Subscription, *models.User) {
	var dbSub *models.Subscription
	var dbUser *models.User

	if assert.Equal(t, http.StatusOK, rsp.StatusCode) {
		rspSub := new(models.Subscription)
		extractPayload(t, rsp, rspSub)
		if rspSub.ID == "" {
			assert.FailNow(t, "Failed to get a valid subscription")
		}

		expected.ID = rspSub.ID
		validateSub(t, expected, rspSub)

		dbSub = &models.Subscription{
			ID: rspSub.ID,
		}
		dbRsp := db.Where(dbSub).First(dbSub)
		if assert.NoError(t, dbRsp.Error) {
			validateSub(t, expected, dbSub)
		}

		dbUser = new(models.User)
		userRsp := db.Where("id = ?", expectedUser.ID).Find(dbUser)
		if assert.NoError(t, userRsp.Error) {
			assert.Equal(t, expectedUser.Email, dbUser.Email)
			assert.Equal(t, expectedUser.RemoteID, dbUser.RemoteID)
		}
	}

	return dbSub, dbUser
}
