package api

import (
	"testing"

	"io/ioutil"

	"github.com/netlify/netlify-subscriptions/models"
	"github.com/stretchr/testify/assert"
)

func TestQueryForAllSubsAsUser(t *testing.T) {
	s1 := createSubscription(testUserID, testUserEmail, "nonsense")
	s2 := createSubscription(testUserID, testUserEmail, "more-nonsense")
	s3 := createSubscription("batman", "bruce@dc.com", "nonsense")
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
	s1 := createSubscription(testUserID, testUserEmail, "nonsense")
	s2 := createSubscription(testUserID, testUserEmail, "more-nonsense")
	defer cleanup(s1, s2)

	rsp := request(t, "GET", "/subscriptions/"+s1.ID, nil, false)
	sub := new(models.Subscription)
	extractPayload(t, rsp, sub)

	validateSub(t, s1, sub)
}

func TestQueryForSingleSubAsAdmin(t *testing.T) {
	s1 := createSubscription("twoface", testUserEmail, "nonsense")
	s2 := createSubscription("twoface", testUserEmail, "more-nonsense")
	defer cleanup(s1, s2)

	rsp := request(t, "GET", "/subscriptions/"+s1.ID, nil, true)

	sub := new(models.Subscription)
	extractPayload(t, rsp, sub)

	validateSub(t, s1, sub)
}

func TestRemoveSubscriptionAsUser(t *testing.T) {
	s1 := createSubscription(testUserID, testUserEmail, "nonsense")
	s2 := createSubscription(testUserID, testUserEmail, "more-nonsense")
	defer cleanup(s1, s2)

	rsp := request(t, "DELETE", "/subscriptions/"+s1.ID, nil, false)

	b, _ := ioutil.ReadAll(rsp.Body)
	assert.Equal(t, "{}\n", string(b))
	assert.Equal(t, 202, rsp.StatusCode)

	found := &models.Subscription{ID: s1.ID}
	dbRsp := db.Unscoped().Find(found)
	if assert.Nil(t, dbRsp.Error) {
		assert.NotNil(t, found.DeletedAt)
	}
}

func TestRemoveSubscriptionAsAdmin(t *testing.T) {
	s1 := createSubscription("twoface", testUserEmail, "nonsense")
	s2 := createSubscription("twoface", testUserEmail, "more-nonsense")
	defer cleanup(s1, s2)

	rsp := request(t, "DELETE", "/subscriptions/"+s1.ID, nil, true)

	b, _ := ioutil.ReadAll(rsp.Body)
	assert.Equal(t, "{}\n", string(b))
	assert.Equal(t, 202, rsp.StatusCode)

	found := &models.Subscription{ID: s1.ID}
	dbRsp := db.Unscoped().Find(found)
	if assert.Nil(t, dbRsp.Error) {
		assert.NotNil(t, found.DeletedAt)
	}
}

// ------------------------------------------------------------------------------------------------
// helpers
// ------------------------------------------------------------------------------------------------

func validateSub(t *testing.T, expected *models.Subscription, actual *models.Subscription) {
	assert.Equal(t, expected.ID, actual.ID)
	assert.Equal(t, expected.UserID, actual.UserID)
	assert.Equal(t, expected.UserEmail, actual.UserEmail)
	assert.Equal(t, expected.RemoteID, actual.RemoteID)
	assert.Equal(t, expected.Plan, actual.Plan)
}

func createSubscription(userID, userEmail, plan string) *models.Subscription {
	sub := models.NewSubscription(userID, userEmail, plan)
	db.Create(sub)
	return sub
}

func cleanup(todelete ...interface{}) {
	for _, td := range todelete {
		db.Unscoped().Delete(td)
	}
}
