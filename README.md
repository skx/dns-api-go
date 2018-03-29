[![Travis CI](https://img.shields.io/travis/skx/dns-api-go/master.svg?style=flat-square)](https://travis-ci.org/skx/dns-api-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/skx/dns-api-go)](https://goreportcard.com/report/github.com/skx/dns-api-go)
[![license](https://img.shields.io/github/license/skx/dns-api-go.svg)](https://github.com/skx/dns-api-go/blob/master/LICENSE)
[![Release](https://img.shields.io/github/release/skx/dns-api-go.svg)](https://github.com/skx/dns-api-go/releases/latest)

# dns-api-go

This is a golang port of the old [perl-based DNS-API.org site](https://github.com/skx/dns-api.org/).

* Still allows the same lookups.
* Still has rate-limiting.

Missing features:

* TTL on records
   * Because I couldn't find a decent DNS library.


## Installation

You can install the project like so:

    $ go get github.com/skx/dns-api-go


## Notes

The main page dynamically includes the domain-name under which it was reached,
so we can deploy it automatically even on other sites.


## Hacking

If you update the files beneath `data/` you need to rebuild the `bindata.go` file before they will become visible.

First of all install the `go-bindata` tool if you don't already have it:

     $ go get -u github.com/jteeuwen/go-bindata/...

Now you can rebuild like so:

     $ go-bindata -nomemcopy data/
     $ go build .

Steve
--
