package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
	log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%d", *port), h))
}

type handler struct {
	R map[string]interface{}
}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	log.Println("Serving: ", path)
	v, ok := h.R[path]
	if !ok {
		http.Error(w, fmt.Sprintf("path not found: %s", path), http.StatusNotFound)
		return
	}
	switch {
	case strings.HasSuffix(path, ".mod"):
		data, err := os.ReadFile(v.(string))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(data)
	case strings.HasSuffix(path, "list"):
		w.Write([]byte(v.(string)))
	default:
		if err := json.NewEncoder(w).Encode(v); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
