FROM calavera/go-glide:v0.12.2

ADD . /go/src/github.com/netlify/netlify-subscriptions

RUN useradd -m netlify && cd /go/src/github.com/netlify/netlify-subscriptions && make deps build && mv netlify-subscriptions /usr/local/bin/

USER netlify
CMD ["netlify-subscriptions"]
