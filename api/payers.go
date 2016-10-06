package api

import (
	"errors"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/sub"
)

type payerProxy interface {
	create(userID, plan, token string) (string, error)
	update(subID, plan, token string) (string, error)
	delete(subID string) error
}

type StripeProxy struct {
}

func (StripeProxy) create(userID, plan, token string) (string, error) {
	s, err := sub.New(&stripe.SubParams{
		Customer: userID,
		Plan:     plan,
	})
	if err != nil {
		return "", err
	}
	return s.ID, nil
}

func (StripeProxy) update(subID, plan, token string) (string, error) {
	s, err := sub.Update(subID, &stripe.SubParams{
		Plan: plan,
	})
	if err != nil {
		return "", err
	}

	return s.ID, nil
}

func (StripeProxy) delete(subID string) error {
	_, err := sub.Cancel(subID, &stripe.SubParams{})
	return err
}

type errorProxy struct {
}

func (errorProxy) create(userID, plan, token string) (string, error) {
	return "", errors.New("No payer proxy provided")
}
func (errorProxy) update(subID, plan, token string) (string, error) {
	return "", errors.New("No payer proxy provided")
}
func (errorProxy) delete(subID string) error {
	return errors.New("No payer proxy provided")
}
