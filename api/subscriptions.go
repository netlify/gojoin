package api

import (
	"context"
	"net/http"

	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/guregu/kami"
	"github.com/netlify/netlify-subscriptions/models"
	"gopkg.in/square/go-jose.v1/json"
)

type subscriptionRequest struct {
	StripeKey string `json:"stripe_key"`
	Plan      string `json:"plan"`
}

func (s subscriptionRequest) Valid() error {
	missing := []string{}
	if s.StripeKey == "" {
		missing = append(missing, "stripe_key")
	}
	if s.Plan == "" {
		missing = append(missing, "plan")
	}

	if len(missing) > 0 {
		return fmt.Errorf("Missing fields: " + strings.Join(missing, ","))
	}

	return nil
}

// TODO
// listSubs will query stripe for all the subscriptions for a given user.
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
			writeError(w, http.StatusInternalServerError, "DB error while searching for subscriptions")
		}
		return
	}

	log.Debugf("Found %d subscriptions associated with id %s", len(subs), claims.ID)
	sendJSON(w, http.StatusOK, subs)
}

func viewSub(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	subType := kami.Param(ctx, "type")
	claims := getClaims(ctx)
	sub, err := getSubscription(ctx, claims.ID, subType)
	if err != nil {
		sendJSON(w, err.Code, err)
	}
	if sub == nil {
		writeError(w, http.StatusNotFound, "No subscription found")
		return
	}

	sendJSON(w, http.StatusOK, sub)
}

func deleteSub(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	subType := kami.Param(ctx, "type")
	claims := getClaims(ctx)
	sub, err := getSubscription(ctx, claims.ID, subType)
	if err != nil {
		sendJSON(w, err.Code, err)
	}

	if sub != nil {
		log := getLogger(ctx).WithField("type", subType)

		pp := getPayerProxy(ctx)
		err := pp.delete(sub.RemoteID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Error communicating with stripe: %s", err)
			return
		}

		log.Info("Removed subscription from stripe")
		rsp := getDB(ctx).Delete(sub)
		if rsp.Error != nil {
			log.WithError(rsp.Error).Warnf("Error while deleting subscription %+v", sub)
			writeError(w, http.StatusInternalServerError, "Error while deleting subscription")
			return
		}

		log.Info("Removed subscription from db")
	}

	sendJSON(w, http.StatusAccepted, struct{}{})
}

func createOrModSub(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	payload, httpErr := extractValidPayload(r)
	if httpErr != nil {
		sendJSON(w, httpErr.Code, httpErr)
		return
	}

	subType := kami.Param(ctx, "type")
	log := getLogger(ctx).WithFields(logrus.Fields{
		"plan": payload.Plan,
		"type": subType,
	})
	ctx = setLogger(ctx, log)

	// do we have a subscription already?
	claims := getClaims(ctx)
	sub, httpErr := getSubscription(ctx, claims.ID, subType)
	if httpErr != nil {
		sendJSON(w, httpErr.Code, httpErr)
		return
	}

	if sub == nil {
		log.Debug("Starting to create new subscription")
		sub, httpErr = createSub(ctx, subType, payload)
	} else {
		log.WithField("old_plan", sub.Plan).Debug("Starting to update subscription")
		httpErr = updateSub(ctx, sub, payload)
	}

	if httpErr != nil {
		sendJSON(w, httpErr.Code, httpErr)
		return
	}

	sendJSON(w, http.StatusOK, sub)
}

func createSub(ctx context.Context, subType string, payload *subscriptionRequest) (*models.Subscription, *HTTPError) {
	log := getLogger(ctx)
	pp := getPayerProxy(ctx)
	claims := getClaims(ctx)

	remoteID, err := pp.create(claims.ID, payload.Plan, payload.StripeKey)
	if err != nil {
		log.WithError(err).Info("Failed to create sub in stripe")
		return nil, httpError(http.StatusBadRequest, "Failed create new subscription for plan %s", payload.Plan)
	}

	sub := &models.Subscription{
		RemoteID:  remoteID,
		UserEmail: claims.Email,
		UserID:    claims.ID,
		Plan:      payload.Plan,
		Type:      subType,
	}

	rsp := getDB(ctx).Create(sub)
	if rsp.Error != nil {
		log.WithError(rsp.Error).Warnf("Failed to create new subscription after successful stripe call: %+v", sub)
		return nil, httpError(http.StatusInternalServerError, "Error while creating db entry, but stripe call was successful")
	}

	return sub, nil
}

func updateSub(ctx context.Context, existing *models.Subscription, payload *subscriptionRequest) *HTTPError {
	log := getLogger(ctx)
	pp := getPayerProxy(ctx)

	remoteID, err := pp.update(existing.RemoteID, payload.Plan, payload.StripeKey)
	if err != nil {
		log.WithError(err).Info("Failed to create sub in stripe")
		return httpError(http.StatusBadRequest, "Failed updating subscription %s to plan %s", existing.RemoteID, payload.Plan)
	}

	existing.RemoteID = remoteID
	existing.Plan = payload.Plan

	rsp := getDB(ctx).Save(existing)
	if rsp.Error != nil {
		log.WithError(rsp.Error).Warnf("Failed to create new subscription after successful stripe call: %+v", existing)
		return httpError(http.StatusInternalServerError, "Error while creating db entry, but stripe call was successful")
	}

	return nil
}

func getSubscription(ctx context.Context, userID string, planType string) (*models.Subscription, *HTTPError) {
	log := getLogger(ctx).WithField("type", planType)
	db := getDB(ctx)
	sub := &models.Subscription{
		Type:   planType,
		UserID: userID,
	}

	if rsp := db.Where(sub).First(sub); rsp.Error != nil {

		if rsp.RecordNotFound() {
			log.Debug("Didn't find record")
			return nil, nil
		}
		forString := fmt.Sprintf("Error while searching for subscription user %s and type %s", userID, planType)
		log.WithError(rsp.Error).Warnf(forString)
		return nil, httpError(http.StatusInternalServerError, forString)
	}

	log.Debug("Successfully retrieved subscription")
	return sub, nil
}

func extractValidPayload(r *http.Request) (*subscriptionRequest, *HTTPError) {
	payload := new(subscriptionRequest)
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(payload); err != nil {
		return nil, httpError(http.StatusBadRequest, "failed to decode payload: "+err.Error())
	}
	if err := payload.Valid(); err != nil {
		return nil, httpError(http.StatusBadRequest, "Failed to provide a valid request: "+err.Error())
	}
	return payload, nil
}
