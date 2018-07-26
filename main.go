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
	"sync"
	"text/template"
	"time"

	"github.com/go-redis/redis"
	"github.com/go-redis/redis_rate"
	"github.com/gorilla/mux"
	graphite "github.com/marpaia/graphite-golang"
	"github.com/robfig/cron"
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
// Stats (optionally) submitted to metric-host
//
var stats map[string]int64

//
// Mutex to protect stats-changes
//
var mutex = &sync.Mutex{}

// Handle to our metrics-host
var metrics *graphite.Graphite

// MetricsFromEnvironment sets up a carbon connection from the environment
// if suitable values are found.
func MetricsFromEnvironment() {

	//
	// Get the hostname to connect to.
	//
	host := os.Getenv("METRICS_HOST")
	if host == "" {
		host = os.Getenv("METRICS")
	}

	// No host then we'll return
	if host == "" {
		return
	}

	// Split the into Host + Port
	ho, pr, err := net.SplitHostPort(host)
	if err != nil {
		// If that failed we assume the port was missing
		ho = host
		pr = "2003"
	}

	// Setup the protocol to use
	protocol := os.Getenv("METRICS_PROTOCOL")
	if protocol == "" {
		protocol = "udp"
	}

	// Ensure that the port is an integer
	port, err := strconv.Atoi(pr)
	if err == nil {
		metrics, err = graphite.GraphiteFactory(protocol, ho, port, "")

		if err != nil {
			fmt.Printf("Error setting up metrics - skipping - %s\n", err.Error())
		}
	} else {
		fmt.Printf("Error setting up metrics - failed to convert port to number - %s\n", err.Error())

	}
}

//
// This function is called every 30 seconds if we were launched
// with a METRICS environmental-variable.
//
func submitMetrics() {
	if metrics != nil {
		mutex.Lock()
		for key, val := range stats {
			v := os.Getenv("METRICS_VERBOSE")
			if v != "" {
				fmt.Printf("%s %d\n", key, val)
			}
			metrics.SimpleSend(key, fmt.Sprintf("%d", val))
		}
		mutex.Unlock()
	}
}

//
// RemoteIP retrieves the remote IP address of the requesting HTTP-client.
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
// RobotHandler handles the request for /robots.txt
//
func RobotHandler(res http.ResponseWriter, req *http.Request) {
	serveResource(res, req, "data/robots.txt", "text/plain")
}

//
// HumanHandler handles the request for /humans.txt
//
func HumanHandler(res http.ResponseWriter, req *http.Request) {
	serveResource(res, req, "data/humans.txt", "text/plain")
}

//
// IconHandler handles the request for /favicon.ico
//
func IconHandler(res http.ResponseWriter, req *http.Request) {
	serveResource(res, req, "data/favicon.ico", "image/x-icon")
}

//
// IndexHandler returns our front-page.
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

		mutex.Lock()
		stats["dns.type."+t] += 1
		stats["dns.errors"] += 1
		mutex.Unlock()

	} else {
		out, _ := json.MarshalIndent(results, "", "     ")
		fmt.Fprintf(res, "%s", out)

		mutex.Lock()
		stats["dns.type."+t] += 1
		stats["dns.queries"] += 1
		mutex.Unlock()
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
	// If we have a metrics-host then we'll submit metrics there
	//
	MetricsFromEnvironment()

	//
	// If we did setup metrics-stuff then we'll want to submit metrics
	// regularly.  Fire up a job to do that every half-minute.
	//
	if metrics != nil {
		c := cron.New()
		c.AddFunc("@every 30s", func() { submitMetrics() })
		c.Start()
	}

	//
	// And finally start our HTTP-server
	//
	serve(*host, *port)
}

// init sets up our stats-map.
//
// Ordinarily I'd do this in `main()` however that would mean that the
// map wouldn't be created in time for our test-cases - as main() wouldn't
// be invoked.
//
func init() {
	//
	// Populate our stats-map
	//
	stats = make(map[string]int64)
}
