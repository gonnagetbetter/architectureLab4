package main

import (
    "net/http"
    "net/http/httptest"
    "testing"

    . "gopkg.in/check.v1"

    "github.com/jarcoal/httpmock"
)

func Test(t *testing.T) { TestingT(t) }

type MySuite struct{}

var _ = Suite(&MySuite{})

func (s *MySuite) TestScheme(c *C) {
    *https = false
    c.Assert(scheme(), Equals, "http")

    *https = true
    c.Assert(scheme(), Equals, "https")

    *https = false
}

func (s *MySuite) TestFindBestServer(c *C) {
    serversPool = []*Server{
        {URL: "Server1", DataProcessed: 10, Healthy: false},
        {URL: "Server2", DataProcessed: 20, Healthy: false},
        {URL: "Server3", DataProcessed: 30, Healthy: false},
    }
    c.Assert(findBestServer(serversPool), Equals, -1)

    serversPool = []*Server{
        {URL: "Server1", DataProcessed: 10, Healthy: true},
        {URL: "Server2", DataProcessed: 20, Healthy: true},
        {URL: "Server3", DataProcessed: 30, Healthy: true},
    }
    c.Assert(findBestServer(serversPool), Equals, 0)

    serversPool = []*Server{
        {URL: "Server1", DataProcessed: 10, Healthy: false},
        {URL: "Server2", DataProcessed: 20, Healthy: true},
        {URL: "Server3", DataProcessed: 30, Healthy: true},
    }
    c.Assert(findBestServer(serversPool), Equals, 1)

    serversPool = []*Server{
        {URL: "Server1", DataProcessed: 10, Healthy: true},
        {URL: "Server2", DataProcessed: 5, Healthy: true},
        {URL: "Server3", DataProcessed: 30, Healthy: true},
    }
    c.Assert(findBestServer(serversPool), Equals, 1)
}


func (s *MySuite) TestForward(c *C) {
    httpmock.Activate()
    defer httpmock.DeactivateAndReset()

    httpmock.RegisterResponder("GET", "http://server1:8080/",
        httpmock.NewStringResponder(200, "OK"))

    serversPool = []*Server{
        {URL: "server1:8080", Healthy: true, DataProcessed: 0},
    }

    req, err := http.NewRequest("GET", "/", nil)
    c.Assert(err, IsNil)
    rr := httptest.NewRecorder()
    err = forward(rr, req)
    c.Assert(err, IsNil)
    // Check if data processed has been updated
    c.Assert(serversPool[0].DataProcessed, Equals, int64(2))
}

func (s *MySuite) TestForwardWithUnhealthyServer(c *C) {
    httpmock.Activate()
    defer httpmock.DeactivateAndReset()

    httpmock.RegisterResponder("GET", "http://server1:8080/",
        httpmock.NewStringResponder(500, "Error"))

    serversPool = []*Server{
        {URL: "server1:8080", Healthy: false, DataProcessed: 0},
    }

    req, err := http.NewRequest("GET", "/", nil)
    c.Assert(err, IsNil)
    rr := httptest.NewRecorder()
    err = forward(rr, req)
    c.Assert(err, NotNil)
}
