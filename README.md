[![Go Report Card](https://goreportcard.com/badge/github.com/skx/dns-api-go)](https://goreportcard.com/report/github.com/skx/dns-api-go)
[![license](https://img.shields.io/github/license/skx/dns-api-go.svg)](https://github.com/skx/dns-api-go/blob/master/LICENSE)
[![Release](https://img.shields.io/github/release/skx/dns-api-go.svg)](https://github.com/skx/dns-api-go/releases/latest)
[![gocover store](http://gocover.io/_badge/github.com/skx/dns-api-go)](http://gocover.io/github.com/skx/dns-api-go)



* [dns-api-go](#dns-api-go)
* [Installation](#installation)
  * [Source Installation go &lt;=  1.11](#source-installation-go---111)
  * [Source installation go  &gt;= 1.12](#source-installation-go---112)
* [Rate Limiting](#rate-limiting)
* [Metrics](#metrics)
* [Docker deployment](#docker-deployment)
* [Heroku deployment](#heroku-deployment)
* [Notes](#notes)
* [Hacking](#hacking)
* [Production Deployment](#production-deployment)
* [Github Setup](#github-setup)


# dns-api-go

This is a golang port of the old [perl-based DNS-API.org site](https://github.com/skx/dns-api.org/).

* Still allows the same lookups.
* Still has rate-limiting.


## Installation

There are two ways to install this project from source, which depend on the version of the [go](https://golang.org/) version you're using.

If you don't have a golang environment configured you should be able to download [a binary release](https://github.com/skx/dns-api-go/releases) from the github project.


### Source Installation go <=  1.11

If you're using `go` before 1.11 then the following command should fetch/update the project and install it upon your system:

     $ go get -u github.com/skx/dns-api-go


### Source installation go  >= 1.12

If you're using a more recent version of `go` (which is _highly_ recommended), you need to clone to a directory which is not present upon your `GOPATH`:

    git clone https://github.com/skx/dns-api-go
    cd dns-api-go
    go install




### Rate Limiting

The server has support for rate-limiting, you can enable this by passing the address of a [redis](https://redis.io/) server to the binary:

    $ dns-api-go  -redis-server localhost:6379

If this flag is not present then rate-limiting will be disabled.  If a client
makes too many requests they will be returned a [HTTP 429 status-code](https://httpstatuses.com/429).  Each request made will return a series of headers
prefixed with `X-RateLimit` to allow clients to see how many requests they
have made, and have remaining.



### Metrics

If you have a graphite-host you can point the environmental-variable `METRICS`
at it.  This will then receive metrics:

* Counts of DNS-queries by type.
* Count of success/failure responses.
* System-metrics.
   * via [golang-metrics](https://github.com/skx/golang-metrics)



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

* The main page dynamically includes the domain-name under which it was reached,
so we can deploy it automatically even on other sites.
* PTR (reverse-DNS) requests must be submitted in reverse-format, for example:
  * https://dns-api.org/ptr/100.183.9.176.in-addr.arpa.
  * https://dns-api.org/ptr/0.0.1.0.0.0.0.0.0.0.0.0.0.0.0.0.3.8.0.6.1.5.1.0.8.f.4.0.1.0.a.2.ip6.arpa.


## Hacking

If you alter the template-files beneath `data/` you will need to rebuild the `static.go` file before those changes will become visible.  (i.e. They are pre-processed and included inline in our generated binary, rather than being read at run-time.)

You'll need to install the [implant](https://github.com/skx/implant) tool.

Now you can regenerate the `static.go` file using that:

     $ implant -input data/ -output static.go

And rebuild the main binary:

     $ go build .


## Production Deployment

Surprisingly hosting this service publicly results in a **ton** of:

* Bandwidth usage.
* Support requests.

The former I tried to handle via cloudflare, but caching DNS data means the service doesn't run in a useful way, as you might expect.

To avoid further support-request and abuse complaints I'm taking the main service [dns-api.org](https://dns-api.org/) offline - unless people pay.  The code _here_ will always be open, and available for use though.

The current codebase will serve a "We're retired - unless you pay" message if the file `/tmp/retired` exists.  This will be checked every 30 seconds or so.

API-key support will be added shortly, in [#11](https://github.com/skx/dns-api-go/issues/11).



## Github Setup

This repository is configured to run tests upon every commit, and when pull-requests are created/updated.  The testing is carried out via [.github/run-tests.sh](.github/run-tests.sh) which is used by the [github-action-tester](https://github.com/skx/github-action-tester) action.

Releases are automated in a similar fashion via [.github/build](.github/build), and the [github-action-publish-binaries](https://github.com/skx/github-action-publish-binaries) action.


Steve
--
