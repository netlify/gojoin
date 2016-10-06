package api

import (
	"testing"

	"io/ioutil"

	"github.com/netlify/netlify-subscriptions/models"
	"github.com/pborman/uuid"
	"github.com/stretchr/testify/assert"
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

//func TestQueryForSingleSubAsAdmin(t *testing.T) {
//	s1 := createSubscription("twoface", testUserEmail, "nonsense")
//	s2 := createSubscription("twoface", testUserEmail, "more-nonsense")
//	defer cleanup(s1, s2)
//
//	rsp := request(t, "GET", "/subscriptions/"+s1.ID, nil, true)
//
//	sub := new(models.Subscription)
//	extractPayload(t, rsp, sub)
//
//	validateSub(t, s1, sub)
//}
//
func TestRemoveSubscriptionAsUser(t *testing.T) {
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
}

func TestRemoveSubNotFound(t *testing.T) {
	// TODO
}

func TestGetSubNotFound(t *testing.T) {
	// TODO
}

//func TestRemoveSubscriptionAsAdmin(t *testing.T) {
//	s1 := createSubscription("twoface", testUserEmail, "nonsense")
//	s2 := createSubscription("twoface", testUserEmail, "more-nonsense")
//	defer cleanup(s1, s2)
//
//	rsp := request(t, "DELETE", "/subscriptions/"+s1.ID, nil, true)
//
//	b, _ := ioutil.ReadAll(rsp.Body)
//	assert.Equal(t, "{}\n", string(b))
//	assert.Equal(t, 202, rsp.StatusCode)
//
//	found := &models.Subscription{ID: s1.ID}
//	dbRsp := db.Unscoped().Find(found)
//	if assert.Nil(t, dbRsp.Error) {
//		assert.NotNil(t, found.DeletedAt)
//	}
//}

func TestCreateNewSubscription(t *testing.T) {
	tp := &testProxy{createSubID: "remote-id"}
	api.payerProxy = tp
	defer func() { api.payerProxy = &errorProxy{} }()

	payload := &subscriptionRequest{
		StripeKey: "something",
		Plan:      "super-important",
	}
	rsp := request(t, "PUT", "/subscriptions/membership", payload, false)
	rspSub := new(models.Subscription)
	extractPayload(t, rsp, rspSub)
	assert.NotEqual(t, "", rspSub.ID)

	expected := &models.Subscription{
		ID:        rspSub.ID,
		UserEmail: testUserEmail,
		Type:      "membership",
		UserID:    testUserID,
		Plan:      "super-important",
		RemoteID:  "remote-id",
	}
	validateSub(t, expected, rspSub)

	dbSub := &models.Subscription{
		ID: rspSub.ID,
	}
	dbRsp := db.Where(dbSub).First(dbSub)
	if assert.NoError(t, dbRsp.Error) {
		validateSub(t, expected, dbSub)
	}

	assert.Len(t, tp.createCalls, 1)
	call := tp.createCalls[0]
	assert.Equal(t, "super-important", call.plan)
	assert.Equal(t, "something", call.token)
	assert.Equal(t, testUserID, call.userID)
}

func TestModifySubscription(t *testing.T) {

}

//func TestCreateNewSubscriptionWithBadPayload(t *testing.T) {
//
//}
//
//func TestCreateNewSubscriptionWithExisting(t *testing.T) {
//
//}
//
//func TestCreateNewSubscriptionWithStripeError(t *testing.T) {
//
//}

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
	return "", nil
}
