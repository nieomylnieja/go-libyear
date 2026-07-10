package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	port := flag.Int("port", 8080, "port to run the server on")
	responsesPath := flag.String("path", "", "path to responses.json file")
	flag.Parse()
	if *responsesPath == "" {
		log.Fatal("flag -path is required")
	}

	data, err := os.ReadFile(*responsesPath)
	if err != nil {
		log.Fatal(err)
	}
	var responses map[string]interface{}
	if err = json.Unmarshal(data, &responses); err != nil {
		log.Fatal(err)
	}
	h := handler{R: responses}

	log.Printf("Listening on port: %d\n", *port)
	server := http.Server{
		Addr:              net.JoinHostPort("", strconv.Itoa(*port)),
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatalln(server.ListenAndServe())
}

type handler struct {
	R map[string]interface{}
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	v, ok := h.R[path]
	if !ok {
		http.Error(w, fmt.Sprintf("no matching versions found for: %s", path), http.StatusNotFound)
		return
	}
	switch {
	case strings.HasSuffix(path, ".mod"):
		data, err := os.ReadFile(v.(string))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(data); err != nil {
			log.Printf("failed to write module response: %v", err)
		}
	case strings.HasSuffix(path, "list"):
		if _, err := w.Write([]byte(v.(string))); err != nil {
			log.Printf("failed to write version list response: %v", err)
		}
	default:
		if err := json.NewEncoder(w).Encode(v); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
