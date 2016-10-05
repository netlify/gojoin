package api

import (
	"context"
	"net/http"

	"github.com/guregu/kami"
	"github.com/netlify/netlify-subscriptions/models"
)

// TODO
// [ ] query params
// [ ] admin access
func listSubs(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	log := getLogger(ctx)
	claims := getClaims(ctx)
	db := getDB(ctx)

	subs := []models.Subscription{}
	if rsp := db.Where("user_id = ? ", claims.ID).Find(&subs); rsp.Error != nil {
		if rsp.RecordNotFound() {
			notFoundError(w, "Found no records associated with user id %s", claims.ID)
		} else {
			log.WithError(rsp.Error).Warnf("Failed to find records associated with %s", claims.ID)
			internalServerError(w, "DB error while searching for subscriptions")
		}
		return
	}

	log.Debugf("Found %d subscriptions associated with id %s", len(subs), claims.ID)
	sendJSON(w, http.StatusOK, subs)
}

func viewSub(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	subID := kami.Param(ctx, "id")
	sub, err := getSubscription(ctx, subID)
	if err != nil {
		sendJSON(w, err.Code, err)
	}

	claims := getClaims(ctx)
	if sub.UserID != claims.ID && !isAdmin(ctx) {
		unauthorizedError(w, "Can't access subscription %s", subID)
		return
	}

	sendJSON(w, http.StatusOK, sub)
}

func deleteSub(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	subID := kami.Param(ctx, "id")
	sub, err := getSubscription(ctx, subID)
	if err != nil {
		sendJSON(w, err.Code, err)
	}

	claims := getClaims(ctx)
	if sub.UserID != claims.ID && !isAdmin(ctx) {
		unauthorizedError(w, "Can't access subscription %s", subID)
		return
	}

	db := getDB(ctx)
	db.Delete(sub)

	sendJSON(w, http.StatusAccepted, struct{}{})
}

func createSub(ctx context.Context, w http.ResponseWriter, r *http.Request) {

}

func getSubscription(ctx context.Context, subID string) (*models.Subscription, *HTTPError) {
	log := getLogger(ctx).WithField("sub_id", subID)
	db := getDB(ctx)
	sub := &models.Subscription{
		ID: subID,
	}
	if rsp := db.Find(sub); rsp.Error != nil {
		if rsp.RecordNotFound() {
			return nil, httpError(http.StatusNotFound, "Found no record for subscription %s", subID)
		}
		log.WithError(rsp.Error).Warnf("Failed to find record %s", subID)
		return nil, httpError(http.StatusInternalServerError, "Found no record for subscription %s", subID)
	}
	log.Debug("Successfully retrieved subscription")
	return sub, nil
}
