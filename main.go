package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/mux"

	"github.com/ayufan/debian-repository/internal/apache_log"
	"github.com/ayufan/debian-repository/internal/deb"
	"github.com/ayufan/debian-repository/internal/deb_cache"
	"github.com/ayufan/debian-repository/internal/deb_key"
	"github.com/ayufan/debian-repository/internal/github_client"
)

var signingKey *deb_key.Key

func createRoutes() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/settings/cache/clear", clearHandler).Methods("GET", "POST")

	r.HandleFunc("/", mainHandler).Methods("GET")

	r.HandleFunc("/orgs/{owner}", indexHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/", indexHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/archive.key", archiveKeyHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/dists/{suite}/{file:.*}", fileHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{component}", distributionIndexHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{component}/", distributionIndexHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{component}/pool/{repo}/{tag_name}/{file_name}", downloadHandler).Methods("GET", "HEAD")
	r.HandleFunc("/orgs/{owner}/{component}/{file:.*}", fileHandler).Methods("GET")

	// support dists/
	r.HandleFunc("/{owner}/{repo}", indexHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/", indexHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/archive.key", archiveKeyHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/dists/{suite}/{file:.*}", fileHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/pool/{tag_name}/{file_name}", downloadHandler).Methods("GET", "HEAD")
	r.HandleFunc("/{owner}/{repo}/{component}", distributionIndexHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/{component}/", distributionIndexHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/{component}/pool/{tag_name}/{file_name}", downloadHandler).Methods("GET", "HEAD")
	r.HandleFunc("/{owner}/{repo}/{component}/{file:.*}", fileHandler).Methods("GET")

	return r
}

func main() {
	var err error

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	flag.Parse()

	if *parseDeb != "" {
		deb, err := deb.ReadFromFile(*parseDeb)
		if err != nil {
			log.Fatalln(err)
		}

		log.Println(string(deb.Control))
		return
	}

	githubAPI = github_client.New(os.Getenv("GITHUB_TOKEN"), *requestCacheExpiration)
	packagesCache = deb_cache.New(*packageLruCache)

	signingKey, err = deb_key.New(os.Getenv("GPG_KEY"))
	if err != nil {
		log.Fatalln(err)
	}

	deb.Suites = strings.Split(*suites, ",")
	if len(deb.Suites) == 0 {
		log.Println("Allowed suites: none")
	} else {
		log.Println("Allowed suites:", strings.Join(deb.Suites, ", "))
	}

	deb.Architectures = strings.Split(*architectures, ",")
	if len(deb.Architectures) == 0 {
		log.Println("Default architectures: none")
	} else {
		log.Println("Default architectures:", strings.Join(deb.Architectures, ", "))
	}

	allowedOwners = strings.Split(os.Getenv("ALLOWED_ORGS"), ",")
	if len(allowedOwners) == 0 {
		log.Println("Allowed owners: none")
	} else {
		log.Println("Allowed owners:", strings.Join(allowedOwners, ", "))
	}

	routes := createRoutes()

	loggingHandler := apache_log.NewApacheLoggingHandler(routes, os.Stdout)
	http.Handle("/", loggingHandler)

	log.Println("Starting web-server on", *httpAddr, "...")
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}
