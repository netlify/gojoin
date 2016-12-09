package api

import (
	"errors"

	"github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/customer"
	"github.com/stripe/stripe-go/sub"
)

type payerProxy interface {
	createCustomer(userID, email, payToken string) (string, error)
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

func (StripeProxy) createCustomer(userID, email, payToken string) (string, error) {
	params := &stripe.CustomerParams{
		Email: email,
		Source: &stripe.SourceParams{
			Token: payToken,
		},
	}
	params.Meta = map[string]string{"nf_id": userID}
	c, err := customer.New(params)
	if err != nil {
		return "", err
	}
	return c.ID, nil
}

/*

POST /subscriptions/members/smashing
{first_name: Matt, last_name: Biilmann, strie_token: "sdfsdfsfsd"}
Signed by {user_id: 1234}

---

existingUser = db.findUser({user_id: 1234})

if existingUser
  stripApi.subscribeToPlan(existingUser.customer_id, params.plan, source: params.stripe_token)
else
  customer = stripeApi.createCustomer({user_id: ...})
  user.create({customer_id: customer.id})
  stripeApi.subscripbeToPlan(user.customer_id, params.plan, source: params.stripe_token)
 end


*/

type errorProxy struct {
}

func (errorProxy) createCustomer(_, _, _ string) (string, error) {
	return "", errors.New("No payer proxy provided")
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
