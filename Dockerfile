FROM calavera/go-glide:v0.12.2

ADD . /go/src/github.com/netlify/gojoin

RUN useradd -m netlify && cd /go/src/github.com/netlify/gojoin && make deps build && mv gojoin /usr/local/bin/

USER netlify
CMD ["gojoin"]
