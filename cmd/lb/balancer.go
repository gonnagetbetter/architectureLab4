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
	"container/heap"

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

type IndexedServer struct {
    Index  int
    Server *Server
}

type ServerHeap []*IndexedServer

func (h ServerHeap) Len() int {
    return len(h)
}

func (h ServerHeap) Less(i, j int) bool {
    return h[i].Server.DataProcessed < h[j].Server.DataProcessed
}

func (h ServerHeap) Swap(i, j int) {
    h[i], h[j] = h[j], h[i]
    h[i].Index = i
    h[j].Index = j
}

func (h *ServerHeap) Push(x interface{}) {
    n := len(*h)
    x.(*IndexedServer).Index = n
    *h = append(*h, x.(*IndexedServer))
}

func (h *ServerHeap) Pop() interface{} {
    old := *h
    n := len(old)
    x := old[n-1]
    *h = old[0 : n-1]
    return x
}

func health(server *Server, ch chan<- *Server) {
    ctx, _ := context.WithTimeout(context.Background(), timeout)
    req, _ := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s://%s/health", scheme(), server.URL), nil)
    resp, err := http.DefaultClient.Do(req)
    if err == nil && resp.StatusCode == http.StatusOK {
        server.Healthy = true
        ch <- server
    }
}

func findBestServer(pool []*Server) int {
    ch := make(chan *Server)
    for _, server := range pool {
        go health(server, ch)
    }

    h := &ServerHeap{}
    heap.Init(h)
    for i := 0; i < len(pool); i++ {
        server := <-ch
        if server.Healthy {
            heap.Push(h, &IndexedServer{Index: i, Server: server})
        }
    }

    if h.Len() > 0 {
        return heap.Pop(h).(*IndexedServer).Index
    }
    return -1
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

	// for _, server := range serversPool {
	// 	server.Healthy = health(server)
	// 	go func(s *Server) {
	// 		for range time.Tick(10 * time.Second) {
	// 			mutex.Lock()
	// 			s.Healthy = health(s)
	// 			log.Printf("%s: health=%t, connCnt=%d, dataProcessed=%d", s.URL, s.Healthy, s.ConnCnt, s.DataProcessed)
	// 			mutex.Unlock()
	// 		}
	// 	}(server)
	// }

	frontend := httptools.CreateServer(*port, http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		forward(rw, r)
	}))

	log.Println("Starting load balancer...")
	log.Printf("Tracing support enabled: %t", *traceEnabled)
	frontend.Start()
	signal.WaitForTerminationSignal()
}
