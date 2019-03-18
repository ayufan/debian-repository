package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strconv"
	"strings"

	"time"

	"github.com/google/go-github/github"
	"github.com/gorilla/mux"

	"github.com/ayufan/debian-repository/internal/apache_log"
	"github.com/ayufan/debian-repository/internal/deb"
	"github.com/ayufan/debian-repository/internal/deb_cache"
	"github.com/ayufan/debian-repository/internal/deb_key"
	"github.com/ayufan/debian-repository/internal/github_client"
	"github.com/ayufan/debian-repository/internal/http_helpers"
)

var httpAddr = flag.String("httpAddr", ":5000", "HTTP Address to listen to")
var requestCacheExpiration = flag.Duration("requestCache", 24*time.Hour, "Request cache expiration timeout")
var packageLruCache = flag.Int("packageLruCache", 10000, "Number of packages stored in memory")

var parseDeb = flag.String("parseDeb", "", "Try to parse a debian archive")

var allowedOwners []string
var githubAPI *github_client.API
var packagesCache *deb_cache.Cache
var signingKey *deb_key.Key

func isOwnerAllowed(owner string) bool {
	for _, allowedOwner := range allowedOwners {
		if allowedOwner == owner {
			return true
		}
	}
	return false
}

func iteratePackages(releases []github.RepositoryRelease, distribution string, fn func(release *github.RepositoryRelease, asset *github.ReleaseAsset) error) error {
	for _, release := range releases {
		if release.Draft != nil && *release.Draft {
			continue
		}

		switch distribution {
		case "releases":
			if release.Prerelease != nil && *release.Prerelease {
				continue
			}
		case "pre-releases":
		default:
			return fmt.Errorf("%q is unknown distribution", distribution)
		}

		for _, asset := range release.Assets {
			if !strings.HasSuffix(*asset.Name, ".deb") {
				continue
			}

			err := fn(&release, &asset)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func enumeratePackages(w http.ResponseWriter, r *http.Request, fn func(release *github.RepositoryRelease, asset *github.ReleaseAsset) error) error {
	vars := mux.Vars(r)

	if !isOwnerAllowed(vars["owner"]) {
		return fmt.Errorf("%q is not allowed. Please add it to ALLOWED_ORGS", vars["owner"])
	}

	releases, resp, err := githubAPI.ListReleases(vars["owner"], vars["repo"])
	if resp != nil {
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(resp.Rate.Limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(resp.Rate.Remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resp.Rate.Reset.Unix(), 10))
	}

	// do trigger loading of all packages
	err = iteratePackages(releases, vars["distribution"], func(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
		go func(release github.RepositoryRelease, asset github.ReleaseAsset) {
			packagesCache.Get(&release, &asset)
		}(*release, *asset)
		return nil
	})
	if err != nil {
		return err
	}

	// do actual loading
	err = iteratePackages(releases, vars["distribution"], fn)
	if err != nil {
		return err
	}
	return nil
}

func getPackages(w http.ResponseWriter, r *http.Request) (*deb.Repository, error) {
	vars := mux.Vars(r)

	repository := deb.NewRepository(vars["owner"], vars["repo"])

	err := enumeratePackages(w, r, func(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
		deb, err := packagesCache.Get(release, asset)
		if err == nil {
			repository.Add(deb)
		}
		return nil
	})

	repository.Sort()

	return repository, err
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")

	fmt.Fprintln(w, "<h2>Welcome to automated Debian Repository made on top of GitHub Releases</h2>")

	fmt.Fprintln(w, "<ul>")
	for _, allowedOwner := range allowedOwners {
		fmt.Fprintf(w, `<li><a href=%q>%s</a></li>`, "/orgs/"+allowedOwner, allowedOwner)
	}
	fmt.Fprintln(w, "</ul>")
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	w.Header().Set("Content-Type", "text/html")

	if !isOwnerAllowed(vars["owner"]) {
		fmt.Fprintln(w, "The", vars["owner"], "is not allowed. Please add it to ALLOWED_ORGS.")
		return
	}

	schema := r.Header.Get("X-Forwarded-Proto")
	if schema == "" {
		schema = "http"
	}
	url := schema + "://" + r.Host + strings.TrimSuffix(r.URL.String(), "/")

	fmt.Fprintln(w, "<h2>Welcome to automated Debian Repository made on top of GitHub Releases</h2>")

	if vars["repo"] != "" {
		githubURL := "https://github.com/" + vars["owner"] + "/" + vars["repo"] + "/releases"
		fmt.Fprintln(w, "This repository is built for: ")
		fmt.Fprintf(w, `<a href=%q>%s</a><br>`, githubURL, githubURL)
	} else {
		githubURL := "https://github.com/" + vars["owner"]
		fmt.Fprintln(w, "This repository for releases from all projects in: ")
		fmt.Fprintf(w, `<a href=%q>%s</a><br>`, githubURL, githubURL)
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>1. Add a repository key:</h4>")
	fmt.Fprintln(w, "<code>$ curl -fsSL "+url+"/archive.key | sudo apt-key add -</code>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>2. Add stable repository:</h4>")
	fmt.Fprintln(w, `<code>$ sudo add-apt-repository "deb `+url+`/releases /"</code>`)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>3. (optionally) Add pre-release repository:</h4>")
	fmt.Fprintln(w, `<code>$ sudo add-apt-repository "deb `+url+`/pre-releases /"</code>`)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>4. Update apt:</h4>")
	fmt.Fprintln(w, `<code>$ sudo apt-get update</code>`)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>You can view the status of all packages at:</h4>")
	fmt.Fprintf(w, `<a href=%q>%s</a><br>`, url+"/releases", url+"/releases")
	fmt.Fprintf(w, `<a href=%q>%s</a><br>`, url+"/pre-releases", url+"/pre-releases")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>You can view all packages at:</h4>")
	fmt.Fprintf(w, `<a href=%q>%s</a><br>`, url+"/releases/Packages", url+"/releases/Packages")
	fmt.Fprintf(w, `<a href=%q>%s</a><br>`, url+"/pre-releases/Packages", url+"/pre-releases/Packages")
	fmt.Fprintln(w)
}

func distributionIndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	fmt.Fprintln(w, "List of packages:")

	err := enumeratePackages(w, r, func(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
		p, err := packagesCache.Get(release, asset)
		fmt.Fprintln(w, "Package:", *release.TagName, "/", *asset.Name)
		fmt.Fprintln(w, "\tIsPrerelease:", *release.Prerelease)
		fmt.Fprintln(w, "\tStatus:", err)
		if p != nil {
			fmt.Fprintln(w, "\tRepo:", p.RepoName)
			fmt.Fprintln(w, "\tDownloadURL:", p.DownloadURL)
			fmt.Fprintln(w, "\tSize:", p.FileSize)
			fmt.Fprintln(w, "\tUpdatedAt:", p.UpdatedAt)
		}
		fmt.Fprintln(w)
		return nil
	})
	if err != nil {
		fmt.Fprintln(w, "enumerate error:", err)
	}
}

func archiveKeyHandler(w http.ResponseWriter, r *http.Request) {
	err := signingKey.WriteKey(w)
	if http_helpers.HandleError(w, err) {
		return
	}
}

func packagesHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getPackages(w, r)
	if http_helpers.HandleError(w, err) {
		return
	}

	repository.Write(w)
}

func packagesGzHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getPackages(w, r)
	if http_helpers.HandleError(w, err) {
		return
	}

	w.Header().Set("Content-Type", "binary/octet-stream")

	repository.WriteGz(w)
}

func releaseHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getPackages(w, r)
	if http_helpers.HandleError(w, err) {
		return
	}

	repository.WriteRelease(w)
}

func releaseGpgHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getPackages(w, r)
	if http_helpers.HandleError(w, err) {
		return
	}

	err = signingKey.EncodeWithArmor(w, func(wd io.Writer) error {
		repository.WriteRelease(wd)
		return nil
	})

	http_helpers.HandleError(w, err)
}

func inReleaseHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getPackages(w, r)
	if http_helpers.HandleError(w, err) {
		return
	}

	err = signingKey.Encode(w, func(wd io.Writer) error {
		repository.WriteRelease(wd)
		return nil
	})

	http_helpers.HandleError(w, err)
}

var httpProxy = httputil.ReverseProxy{
	Director: func(*http.Request) {},
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	realURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s",
		vars["owner"], vars["repo"],
		vars["tag_name"], vars["file_name"])

	req, err := http.NewRequest("GET", realURL, nil)
	if http_helpers.HandleError(w, err) {
		return
	}

	res, err := http.DefaultTransport.RoundTrip(req)
	if http_helpers.HandleError(w, err) {
		return
	}
	defer res.Body.Close()

	if res.StatusCode/100 != 3 {
		http_helpers.HandleError(w, fmt.Errorf("expected 3xx, but got: %d: %s", res.StatusCode, res.Status))
		return
	}

	location, err := res.Location()
	if http_helpers.HandleError(w, err) {
		return
	}

	newReq := *r
	newReq.URL = location
	newReq.Host = ""
	newReq.RequestURI = ""

	httpProxy.ServeHTTP(w, &newReq)
}

func clearHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintln(w, "OK")

	githubAPI.Flush()
	packagesCache.Clear()
}

func createRoutes() *mux.Router {
	r := mux.NewRouter()
	r.HandleFunc("/settings/cache/clear", clearHandler).Methods("GET", "POST")

	r.HandleFunc("/", mainHandler).Methods("GET")

	r.HandleFunc("/orgs/{owner}", indexHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/", indexHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/archive.key", archiveKeyHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{distribution}", distributionIndexHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{distribution}/", distributionIndexHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{distribution}/Packages", packagesHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{distribution}/Packages.gz", packagesGzHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{distribution}/Release", releaseHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{distribution}/Release.gpg", releaseGpgHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{distribution}/InRelease", inReleaseHandler).Methods("GET")
	r.HandleFunc("/orgs/{owner}/{distribution}/download/{repo}/{tag_name}/{file_name}", downloadHandler).Methods("GET", "HEAD")

	r.HandleFunc("/{owner}/{repo}", indexHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/", indexHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/archive.key", archiveKeyHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/{distribution}", distributionIndexHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/{distribution}/", distributionIndexHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/{distribution}/Packages", packagesHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/{distribution}/Packages.gz", packagesGzHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/{distribution}/Release", releaseHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/{distribution}/Release.gpg", releaseGpgHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/{distribution}/InRelease", inReleaseHandler).Methods("GET")
	r.HandleFunc("/{owner}/{repo}/{distribution}/download/{tag_name}/{file_name}", downloadHandler).Methods("GET", "HEAD")

	return r
}

func main() {
	var err error

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
