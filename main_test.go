package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

//
// Test that our static-resources work.
//
func TestStaticResources(t *testing.T) {

	//
	// Wire up our end-points
	//
	router := mux.NewRouter()
	router.HandleFunc("/humans.txt", HumanHandler).Methods("GET")
	router.HandleFunc("/robots.txt", RobotHandler).Methods("GET")
	router.HandleFunc("/favicon.ico", IconHandler).Methods("GET")
	router.HandleFunc("/", IndexHandler).Methods("GET")

	//
	// The path we're requesting and the expected content-type
	// of the response.
	//
	type TestCase struct {
		Path string
		Type string
	}

	//
	// The tests
	//
	tests := []TestCase{
		{"/robots.txt", "text/plain"},
		{"/humans.txt", "text/plain"},
		{"/favicon.ico", "image/x-icon"},
		{"/", "text/html; charset=utf-8"}}

	//
	// Run each one.
	//
	for _, test := range tests {

		//
		// Make the request, with the appropriate Accept: header
		//
		req, err := http.NewRequest("GET", test.Path, nil)
		if err != nil {
			t.Fatal(err)
		}

		//
		// Fake it out
		//
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		//
		// Test the status-code is OK
		//
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("Unexpected status-code: %v", status)
		}

		//
		// Test that we got the content-type back
		//
		if ctype := rr.Header().Get("Content-Type"); ctype != test.Type {
			t.Errorf("content-type header does not match: got %v want %v",
				ctype, test.Type)
		}
	}

}

//
// Simple test of our HTTP-handler
//
func TestInvalidDNSTypes(t *testing.T) {
	// Wire up the route
	r := mux.NewRouter()
	r.HandleFunc("/{type}/{value}", DNSHandler).Methods("GET")
	r.HandleFunc("/{type}/{value}/", DNSHandler).Methods("GET")

	// These are all bogus
	states := []string{"cnime", "text", "nameserver", "pointer"}

	// Get the test-server
	ts := httptest.NewServer(r)
	defer ts.Close()

	for _, ty := range states {
		url := ts.URL + "/" + ty + "/localhost"

		resp, err := http.Get(url)
		if err != nil {
			t.Fatal(err)
		}

		//
		// Get the body
		//
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			t.Errorf("Failed to read response-body %v\n", err)
		}

		content := fmt.Sprintf("%s", body)
		if status := resp.StatusCode; status != http.StatusNotFound {
			t.Errorf("Unexpected status-code: %v", status)
		}
		if content != "Invalid lookup-type - use A|AAAA|ANY|CNAME|MX|NS|PTR|SOA|TXT\n" {
			t.Fatalf("Unexpected body: '%s'", body)
		}
	}
}

//
// Test a single lookup of steve.fi's TXT-record.
//
func TestSteve(t *testing.T) {
	// Wire up the route
	r := mux.NewRouter()
	r.HandleFunc("/{type}/{value}", DNSHandler).Methods("GET")
	r.HandleFunc("/{type}/{value}/", DNSHandler).Methods("GET")

	// Get the test-server
	ts := httptest.NewServer(r)
	defer ts.Close()

	url := ts.URL + "/txt/steve.fi"

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}

	//
	// Get the body
	//
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		t.Errorf("Failed to read response-body %v\n", err)
	}

	content := fmt.Sprintf("%s", body)
	if status := resp.StatusCode; status != http.StatusOK {
		t.Errorf("Unexpected status-code: %v", status)
	}

	if !strings.Contains(content, "v=spf1") {
		t.Fatalf("Unexpected body: '%s'", content)
	}
}

//
// Test a single lookup of a bogus-domain.
//
func TestBogusDNS(t *testing.T) {

	// Wire up the route
	r := mux.NewRouter()
	r.HandleFunc("/{type}/{value}", DNSHandler).Methods("GET")
	r.HandleFunc("/{type}/{value}/", DNSHandler).Methods("GET")

	// Get the test-server
	ts := httptest.NewServer(r)
	defer ts.Close()

	url := ts.URL + "/a/invalid@example.com"

	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}

	//
	// Get the body
	//
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		t.Errorf("Failed to read response-body %v\n", err)
	}

	content := fmt.Sprintf("%s", body)
	if status := resp.StatusCode; status != http.StatusOK {
		t.Errorf("Unexpected status-code: %v", status)
	}

	if !strings.Contains(content, "NXDOMAIN") {
		t.Fatalf("Unexpected body: '%s'", content)
	}
}
