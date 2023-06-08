package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gonnagetbetter/architectureLab4/httptools"
	"github.com/gonnagetbetter/architectureLab4/signal"
)

var port = flag.Int("port", 8080, "server port")

const confResponseDelaySec = "CONF_RESPONSE_DELAY_SEC"
const confHealthFailure = "CONF_HEALTH_FAILURE"

const dbUrl = "http://db:8083/db"

type Request struct {
	Value string "json:\"value\""
}

type Response struct {
	Key   string "json:\"key\""
	Value string "json:\"value\""
}

func main() {
	flag.Parse()

	client := http.DefaultClient

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(rw http.ResponseWriter, r *http.Request) {
		rw.Header().Set("Content-Type", "text/plain")

		if failConfig := os.Getenv(confHealthFailure); failConfig == "true" {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte("FAILURE"))
		} else {
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write([]byte("OK"))
		}
	})

	report := NewReport()

	mux.HandleFunc("/api/v1/some-data", func(rw http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			rw.WriteHeader(http.StatusBadRequest)
			return
		}

		resp, err := client.Get(fmt.Sprintf("%s/%s", dbUrl, key))
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		statusOK := resp.StatusCode >= 200 && resp.StatusCode < 300
		if !statusOK {
			rw.WriteHeader(resp.StatusCode)
			return
		}

		respDelayString := os.Getenv(confResponseDelaySec)
		if delaySec, parseErr := strconv.Atoi(respDelayString); parseErr == nil && delaySec > 0 && delaySec < 300 {
			time.Sleep(time.Duration(delaySec) * time.Second)
		}

		report.Process(r)

		var body Response
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		rw.Header().Set("Content-Type", "application/json")

		rw.WriteHeader(http.StatusOK)

		_ = json.NewEncoder(rw).Encode(body)
	})

	server := httptools.CreateServer(*port, mux)

	server.Start()

	time.Sleep(5 * time.Second)

	buffer := new(bytes.Buffer)
	body := Request{Value: time.Now().Format(time.RFC3339)}
	if err := json.NewEncoder(buffer).Encode(body); err != nil {
		fmt.Println("Failed to encode request body:", err)
		return
	}

	res, err := client.Post(fmt.Sprintf("%s/architectureLab4", dbUrl), "application/json", buffer)
	if err != nil {
		fmt.Println("Failed to send POST request:", err)
		return
	}
	defer res.Body.Close()

	signal.WaitForTerminationSignal()
}

func NewReport() *Report {
	return &Report{}
}
