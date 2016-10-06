package api

import (
	"testing"

	"io/ioutil"

	"net/http"

	"github.com/netlify/netlify-subscriptions/models"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/valyala/fasthttp"
)

func TestQueryForAllSubsAsUser(t *testing.T) {
	s1 := createSubscription(testUserID, testUserEmail, "membership", "nonsense")
	s2 := createSubscription(testUserID, testUserEmail, "revenue", "more-nonsense")
	s3 := createSubscription("batman", "bruce@dc.com", "membership", "nonsense")
	defer cleanup(s1, s2, s3)

	rsp := request(t, "GET", "/subscriptions", nil, false)
	subs := []models.Subscription{}
	extractPayload(t, rsp, &subs)

	assert.Equal(t, 2, len(subs))
	foundS1, foundS2 := false, false
	for _, s := range subs {
		switch s.ID {
		case s1.ID:
			validateSub(t, s1, &s)
			foundS1 = true
		case s2.ID:
			validateSub(t, s2, &s)
			foundS2 = true
		default:
			assert.Fail(t, "unexpected sub: "+s.ID)
		}
	}

	assert.True(t, foundS1)
	assert.True(t, foundS2)
}

func TestQueryForSingleSubAsUser(t *testing.T) {
	s1 := createSubscription(testUserID, testUserEmail, "membership", "nonsense")
	s2 := createSubscription(testUserID, testUserEmail, "revenue", "more-nonsense")
	defer cleanup(s1, s2)

	rsp := request(t, "GET", "/subscriptions/membership", nil, false)
	sub := new(models.Subscription)
	extractPayload(t, rsp, sub)

	validateSub(t, s1, sub)
}

func TestRemoveSubscriptionAsUser(t *testing.T) {
	tp := &testProxy{}
	api.payerProxy = tp
	s1 := createSubscription(testUserID, testUserEmail, "membership", "nonsense")
	s2 := createSubscription(testUserID, testUserEmail, "revenue", "more-nonsense")
	defer cleanup(s1, s2)

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
	tp := &testProxy{createSubID: "remote-id"}
	api.payerProxy = tp
	defer func() { api.payerProxy = &errorProxy{} }()

	payload := &subscriptionRequest{
		StripeKey: "something",
		Plan:      "super-important",
	}
	rsp := request(t, "PUT", "/subscriptions/membership", payload, false)

	dbRsp := validateResponseAndDBVal(t, rsp, &models.Subscription{
		UserEmail: testUserEmail,
		Type:      "membership",
		UserID:    testUserID,
		Plan:      "super-important",
		RemoteID:  "remote-id",
	})
	cleanup(dbRsp)

	assert.Len(t, tp.createCalls, 1)
	call := tp.createCalls[0]
	assert.Equal(t, "super-important", call.plan)
	assert.Equal(t, "something", call.token)
	assert.Equal(t, testUserID, call.userID)
	assert.Empty(t, tp.updateCalls)
}

func TestModifySubscription(t *testing.T) {
	tp := &testProxy{updateSubID: "remote-id"}
	api.payerProxy = tp
	defer func() { api.payerProxy = &errorProxy{} }()

	s1 := createSubscription(testUserID, testUserEmail, "pokemon", "magicarp")
	defer cleanup(s1)

	payload := &subscriptionRequest{
		StripeKey: "something",
		Plan:      "charizard",
	}
	rsp := request(t, "PUT", "/subscriptions/pokemon", payload, false)
	dbRsp := validateResponseAndDBVal(t, rsp, &models.Subscription{
		UserEmail: testUserEmail,
		Type:      "pokemon",
		UserID:    testUserID,
		Plan:      "charizard",
		RemoteID:  "remote-id",
	})
	cleanup(dbRsp)

	assert.Len(t, tp.updateCalls, 1)
	call := tp.updateCalls[0]
	assert.Equal(t, "charizard", call.plan)
	assert.Equal(t, "something", call.token)
	assert.Equal(t, s1.RemoteID, call.subID)
	assert.Empty(t, tp.createCalls)
}

func validateResponseAndDBVal(t *testing.T, rsp *http.Response, expected *models.Subscription) *models.Subscription {
	var dbSub *models.Subscription
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
	}

	return dbSub
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

//
//func TestCreateNewSubscriptionWithExisting(t *testing.T) {
//
//}
//
func TestCreateNewSubscriptionWithStripeError(t *testing.T) {
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
	assert.Equal(t, expected.UserEmail, actual.UserEmail)
	assert.Equal(t, expected.Type, actual.Type)
	assert.Equal(t, expected.RemoteID, actual.RemoteID)
	assert.Equal(t, expected.Plan, actual.Plan)

	assert.NotEmpty(t, actual.ID)
	assert.NotEmpty(t, actual.UserID)
	assert.NotEmpty(t, actual.Type)
	assert.NotEmpty(t, actual.RemoteID)
	assert.NotEmpty(t, actual.Plan)
}

func createSubscription(userID, userEmail, planType string, plan string) *models.Subscription {
	sub := &models.Subscription{
		UserID:    userID,
		UserEmail: userEmail,
		Plan:      plan,
		Type:      planType,
		RemoteID:  uuid.NewRandom().String(),
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
