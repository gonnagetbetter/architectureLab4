package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gonnagetbetter/architectureLab4/httptools"
	"github.com/gonnagetbetter/architectureLab4/signal"
)

var (
	port         = flag.Int("port", 8090, "lb port")
	timeoutSec   = flag.Int("timeout-sec", 3, "request timeout time in seconds")
	https        = flag.Bool("https", false, "whether backends support HTTPs")
	traceEnabled = flag.Bool("trace", false, "whether to include tracing information into responses")
)

type Server struct {
	URL           string
	ConnCnt       int
	Healthy       bool
	DataProcessed int64
}

var (
	timeout     = time.Duration(*timeoutSec) * time.Second
	serversPool = []*Server{
		{URL: "server1:8080"},
		{URL: "server2:8080"},
		{URL: "server3:8080"},
	}
	mutex sync.Mutex
)

func scheme() string {
	if *https {
		return "https"
	}
	return "http"
}

func health(server *Server) bool {
	ctx, _ := context.WithTimeout(context.Background(), timeout)
	req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s://%s/health", scheme(), server.URL), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	if resp.StatusCode != http.StatusOK {
		return false
	}
	server.Healthy = true
	return true
}

func findBestServer(pool []*Server) int {
	bestServerIndex := -1
	leastDataProcessed := int64(^uint64(0) >> 1)

	for i, server := range pool {
		if server.Healthy {
			if bestServerIndex == -1 || server.DataProcessed < leastDataProcessed {
				bestServerIndex = i
				leastDataProcessed = server.DataProcessed
			}
		}
	}
	return bestServerIndex
}

func forward(rw http.ResponseWriter, r *http.Request) error {
	ctx, _ := context.WithTimeout(r.Context(), timeout)
	fwdRequest := r.Clone(ctx)

	mutex.Lock()
	minServerIndex := findBestServer(serversPool)

	if minServerIndex == -1 {
		mutex.Unlock()
		rw.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("all servers are busy")
	}

	dst := serversPool[minServerIndex]
	dst.ConnCnt++
	mutex.Unlock()

	fwdRequest.RequestURI = ""
	fwdRequest.URL.Host = dst.URL
	fwdRequest.URL.Scheme = scheme()
	fwdRequest.Host = dst.URL

	resp, err := http.DefaultClient.Do(fwdRequest)
	if err == nil {
		for k, values := range resp.Header {
			for _, value := range values {
				rw.Header().Add(k, value)
			}
		}
		if *traceEnabled {
			rw.Header().Set("lb-from", dst.URL)
		}
		log.Println("fwd", resp.StatusCode, resp.Request.URL)
		rw.WriteHeader(resp.StatusCode)
		defer resp.Body.Close()
		bodySize, err := io.Copy(rw, resp.Body)
		if err != nil {
			log.Printf("Failed to write response: %s", err)
		}

		mutex.Lock()
		dst.DataProcessed += bodySize
		mutex.Unlock()

		return nil
	} else {
		log.Printf("Failed to get response from %s: %s", dst.URL, err)
		rw.WriteHeader(http.StatusServiceUnavailable)
		return err
	}
}

func main() {
	flag.Parse()

	for _, server := range serversPool {
		server.Healthy = health(server)
		go func(s *Server) {
			for range time.Tick(10 * time.Second) {
				mutex.Lock()
				s.Healthy = health(s)
				log.Printf("%s: health=%t, connCnt=%d, dataProcessed=%d", s.URL, s.Healthy, s.ConnCnt, s.DataProcessed)
				mutex.Unlock()
			}
		}(server)
	}

	frontend := httptools.CreateServer(*port, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		forward(rw, r)
	}))

	log.Println("Starting load balancer...")
	log.Printf("Tracing support enabled: %t", *traceEnabled)
	frontend.Start()
	signal.WaitForTerminationSignal()
}
