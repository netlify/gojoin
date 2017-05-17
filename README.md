# GoJoin

This acts as a proxy to Stripe. It exposes a very simple way to call Stripe's subscription endpoints.

GoJoin is released under the [MIT License](LICENSE).
Please make sure you understand its [implications and guarantees](https://writing.kemitchell.com/2016/09/21/MIT-License-Line-by-Line.html).

## authentication
All of the endpoints rely on a JWT token. We will use the user ID set in that token for the user information to Stripe.

The API as is:

    GET /subscriptions -- list all the subscriptions for the user

This endpoint will return a list of subscriptions, but also a JWT token that has been decorated with an `app_metadata.subscriptions` property which is a map of the users subscriptions.

These endpoints are all grouped by a `type` of subscription. For instance if you have a `membership` type with
plan levels gold, silver, and bronze.

    GET /subscriptions/:type
    POST /subscriptions/:type
    DELETE /subscriptions/:type

The POST endpoint takes a payload like so

``` json
    {
        "stripe_key": "xxxxx",
        "plan": "silver"
    }
```

Using this endpoint will create the plan if it doesn't exist, otherwise it will change the subscription to that plan.
The other responses are defined in `api/subscriptions.go`.
