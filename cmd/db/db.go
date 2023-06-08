package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/gonnagetbetter/architectureLab4/datastore"
	"github.com/gonnagetbetter/architectureLab4/httptools"
	"github.com/gonnagetbetter/architectureLab4/signal"
)

var port = flag.Int("port", 8083, "server port")

type Request struct {
	Value string `json:"value"`
}

type Response struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func main() {
	flag.Parse()

	dir, err := ioutil.TempDir("", "temp-dir")
	if err != nil {
		log.Fatal(err)
	}
	db, err := datastore.NewDb(dir, 250)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	h := http.NewServeMux()

	h.HandleFunc("/db/", func(rw http.ResponseWriter, req *http.Request) {
		key := req.URL.Path[4:]

		switch req.Method {
		case http.MethodGet:
			value, err := db.Get(key)
			if err != nil {
				rw.WriteHeader(http.StatusNotFound)
				return
			}

			resp := Response{
				Key:   key,
				Value: value,
			}
			rw.Header().Set("Content-Type", "application/json")
			rw.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(rw).Encode(resp)

		case http.MethodPost:
			var body Request
			err := json.NewDecoder(req.Body).Decode(&body)
			if err != nil {
				rw.WriteHeader(http.StatusBadRequest)
				return
			}

			err = db.Put(key, body.Value)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				return
			}
			rw.WriteHeader(http.StatusCreated)

		default:
			rw.WriteHeader(http.StatusBadRequest)
		}
	})

	server := httptools.CreateServer(*port, h)
	server.Start()
	signal.WaitForTerminationSignal()
}
