//
// This is a port of the dns-api.org service to golang.
//
// Features:
//
//  * Rate-Limiting of 200/requests per hour.
//
//  * Lookup of most common DNS-types
//
//
// Steve
// --
//

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/go-redis/redis"
	"github.com/go-redis/redis_rate"
	"github.com/gorilla/mux"
	_ "github.com/skx/golang-metrics"
)

//
// Our version string.
//
var (
	version = "unreleased"
)

//
// Rate-limiter
//
var rateLimiter *redis_rate.Limiter

//
// Get the remote IP address of the client.
//
// This is used for our redis-based rate-limiting.
//
func RemoteIP(request *http.Request) string {

	//
	// Get the X-Forwarded-For header, if present.
	//
	xForwardedFor := request.Header.Get("X-Forwarded-For")

	//
	// No forwarded IP?  Then use the remote address directly.
	//
	if xForwardedFor == "" {
		ip, _, _ := net.SplitHostPort(request.RemoteAddr)
		return ip
	}

	entries := strings.Split(xForwardedFor, ",")
	address := strings.TrimSpace(entries[0])
	return (address)
}

//
// Serve a static thing from an embedded resource.
//
func serveResource(response http.ResponseWriter, request *http.Request, resource string, mime string) {
	tmpl, err := getResource(resource)
	if err != nil {
		fmt.Fprintf(response, err.Error())
		return
	}
	response.Header().Set("Content-Type", mime)
	fmt.Fprintf(response, string(tmpl))
}

//
// Handler for /robots.txt
//
func RobotHandler(res http.ResponseWriter, req *http.Request) {
	serveResource(res, req, "data/robots.txt", "text/plain")
}

//
// Handler for /humans.txt
//
func HumanHandler(res http.ResponseWriter, req *http.Request) {
	serveResource(res, req, "data/humans.txt", "text/plain")
}

//
// Handler for /favicon.ico
//
func IconHandler(res http.ResponseWriter, req *http.Request) {
	serveResource(res, req, "data/favicon.ico", "image/x-icon")
}

//
// Index-handler
//
// The index-page _should_ be static, but we rewrite the domain-name
// based upon the incoming request.
//
func IndexHandler(res http.ResponseWriter, req *http.Request) {

	//
	// This is the data that we add to our output-template.
	//
	type Pagedata struct {
		Hostname string
		Version  string
		Redis    bool
	}

	//
	// Create an instance and populate the hostname + version
	//
	var x Pagedata
	x.Hostname = req.Host
	x.Version = version
	x.Redis = (rateLimiter != nil)

	//
	// Load our template from the embedded resource.
	//
	tmpl, err := ExpandResource("data/index.html")
	if err != nil {
		fmt.Fprintf(res, err.Error())
		return
	}

	//
	// Parse our template.
	//
	src := string(tmpl)
	t := template.Must(template.New("tmpl").Parse(src))

	//
	// Execute the template into a temporary buffer.
	//
	buf := &bytes.Buffer{}
	err = t.Execute(buf, x)

	//
	// If there were errors, then show them.
	if err != nil {
		fmt.Fprintf(res, err.Error())
		return
	}

	//
	// Otherwise send the result to the caller.
	//
	buf.WriteTo(res)
}

//
// DNSHandler is the meat of our service, it is the handler for performing
// DNS lookups.
//
// It is called via requests like this:
//
//     GET /$TYPE/$NAME
//
//
func DNSHandler(res http.ResponseWriter, req *http.Request) {
	var (
		status int
		err    error
	)
	defer func() {
		if nil != err {
			http.Error(res, err.Error(), status)

			// Don't spam stdout when running test-cases.
			if flag.Lookup("test.v") == nil {
				fmt.Printf("Error: %s\n", err.Error())
			}
		}
	}()

	h := res.Header()
	h.Set("Access-Control-Allow-Origin", "*")

	//
	// Lookup the remote IP and limit to 200/Hour
	//
	ip := RemoteIP(req)
	limit := int64(200)

	//
	// If we've got a rate-limiter then we can use it.
	//
	// This is wrapped because it won't be configured when we
	// run our test-cases (minimal as they might be).
	//
	if rateLimiter != nil {

		//
		// Lookup the current stats.
		//
		rate, delay, allowed := rateLimiter.AllowHour(ip, limit)

		//
		// We'll return the rate-limit headers to the caller.
		//
		h.Set("X-RateLimit-Limit", strconv.FormatInt(limit, 10))
		h.Set("X-RateLimit-IP", ip)
		h.Set("X-RateLimit-Remaining", strconv.FormatInt(limit-rate, 10))
		delaySec := int64(delay / time.Second)
		h.Set("X-RateLimit-Delay", strconv.FormatInt(delaySec, 10))

		//
		// If the limit has been exceeded tell the client.
		//
		if !allowed {
			http.Error(res, "API rate limit exceeded.", 429)
			return
		}
	}

	//
	// Get the query-type and value.
	//
	vars := mux.Vars(req)
	t := vars["type"]
	v := vars["value"]

	//
	// Ensure we received parameters.
	//
	if len(t) < 1 {
		status = http.StatusNotFound
		err = errors.New("Missing 'type' parameter")
		return
	}
	if len(v) < 1 {
		status = http.StatusNotFound
		err = errors.New("Missing 'value' parameter")
		return
	}

	//
	// Lookup type should be upper-case
	//
	t = strings.ToUpper(t)

	//
	// Test that the type is valid
	//
	if t != "A" &&
		t != "AAAA" &&
		t != "CNAME" &&
		t != "MX" &&
		t != "NS" &&
		t != "PTR" &&
		t != "SOA" &&
		t != "TXT" {
		status = http.StatusNotFound
		err = errors.New("Invalid lookup-type - use A|AAAA|CNAME|MX|NS|PTR|SOA|TXT")
		return
	}

	//
	// The result of what we'll return
	//
	results, _ := lookup(v, t)

	//
	// Now output the results as JSON (prettily), if we got some
	// results.
	//
	if len(results) < 1 {
		//
		// Error.
		//
		tmp := make(map[string]string)
		tmp["error"] = "NXDOMAIN"
		out, _ := json.MarshalIndent(tmp, "", "     ")
		fmt.Fprintf(res, "%s", out)
	} else {
		out, _ := json.MarshalIndent(results, "", "     ")
		fmt.Fprintf(res, "%s", out)
	}
}

//
//  Entry-point.
//
func serve(host string, port int) {

	//
	// Create a new router and our route-mappings.
	//
	router := mux.NewRouter()

	//
	// API end-points
	//
	router.HandleFunc("/{type}/{value}", DNSHandler).Methods("GET")
	router.HandleFunc("/{type}/{value}/", DNSHandler).Methods("GET")
	router.HandleFunc("/humans.txt", HumanHandler).Methods("GET")
	router.HandleFunc("/robots.txt", RobotHandler).Methods("GET")
	router.HandleFunc("/favicon.ico", IconHandler).Methods("GET")
	router.HandleFunc("/", IndexHandler).Methods("GET")

	//
	// Bind the router.
	//
	http.Handle("/", router)

	//
	// Show where we'll bind
	//
	bind := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("Launching the server on http://%s\n", bind)

	//
	// Launch the server.
	//
	err := http.ListenAndServe(bind, router)
	if err != nil {
		fmt.Printf("\nError: %s\n", err.Error())
	}
}

func main() {

	//
	// The command-line flags we support
	//
	host := flag.String("host", "127.0.0.1", "The IP to bind upon.")
	red := flag.String("redis-server", "", "The address of a redis-server to store rate-limiting data.")
	port := flag.Int("port", 9999, "The port to bind upon.")
	vers := flag.Bool("version", false, "Show our version and exit.")

	//
	// Parse the flags
	//
	flag.Parse()

	//
	// Showing the version?
	//
	if *vers {
		fmt.Printf("dns-api-go %s\n", version)
		os.Exit(0)
	}

	//
	// If we have a redis-server defined then use it.
	//
	if *red != "" {

		//
		// Setup a redis-connection for rate-limiting.
		//
		ring := redis.NewRing(&redis.RingOptions{
			Addrs: map[string]string{
				"server1": *red,
			},
		})

		//
		// And point the rate-limiter to it
		//
		rateLimiter = redis_rate.NewLimiter(ring)
	} else {

		//
		// No rate-limiting in use
		//
		rateLimiter = nil
	}

	//
	// And finally start our server
	//
	serve(*host, *port)
}
