[![Travis CI](https://img.shields.io/travis/skx/dns-api-go/master.svg?style=flat-square)](https://travis-ci.org/skx/dns-api-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/skx/dns-api-go)](https://goreportcard.com/report/github.com/skx/dns-api-go)
[![license](https://img.shields.io/github/license/skx/dns-api-go.svg)](https://github.com/skx/dns-api-go/blob/master/LICENSE)
[![Release](https://img.shields.io/github/release/skx/dns-api-go.svg)](https://github.com/skx/dns-api-go/releases/latest)
[![gocover store](http://gocover.io/_badge/github.com/skx/dns-api-go)](http://gocover.io/github.com/skx/dns-api-go)

# dns-api-go

This is a golang port of the old [perl-based DNS-API.org site](https://github.com/skx/dns-api.org/).

* Still allows the same lookups.
* Still has rate-limiting.


## Installation

You can install the project like so:

    $ go get -u github.com/skx/dns-api-go

(The `-u` flag will update your installation if it was already present.)


### Rate Limiting

The server has support for rate-limiting, you can enable this by passing the address of a [redis](https://redis.io/) server to the binary:

    $ ./dns-api-go  -redis-server localhost:6379

If this flag is not present then rate-limiting will be disabled.  If a client
makes too many requests they will be returned a HTTP 425 status-cude, and each
request made will return a series of headers prefixed with `X-RateLimit` to
allow clients to see how many requests they have made, and have remaining.


### Docker deployment

If you've cloned this repository you'll notice there is an existing [Dockerfile](Dockerfile) which can be used to build a container.  Create your image like so:

    skx@frodo ~/go/src/github.com/skx/dns-api-go $ docker build -t dns-api:latest .
    Sending build context to Docker daemon  301.1kB
    Step 1/10 : FROM alpine
    latest: Pulling from library/alpine
    ..
    ..
    Step 10/10 : ENTRYPOINT ["/root/dns-api-go", "-host","0.0.0.0" , "-port 3001" ]
    ..
    Successfully built

Once you have your container you can launch it like so:

     $ docker run -d -p 9999:9999/tcp dns-api:latest

And test it via:

     $ curl http://localhost:9999/txt/steve.fi

Deploying with a redis image too, to allow rate-limiting, is [left as an exercise](https://blog.blindside.io/docker-compose-go-c2bb7bed0bab).


### Heroku deployment

Get into the cloned git repository ($GOPATH/src/github.com/skx/dns-api-go) and run:

    $ heroku create
    $ git push heroku master


## Notes

The main page dynamically includes the domain-name under which it was reached,
so we can deploy it automatically even on other sites.


## Hacking

If you update the files beneath `data/` you need to rebuild the `static.go` file before they will become visible.

First of all install the `implant` tool if you don't already have it:

     $ go get -u github.com/skx/implant

Now you can rebuild like so:

     $ implant -input data/ -output static.go
     $ go build .

Steve
--
