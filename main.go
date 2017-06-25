package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"strconv"

	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/armor"
	"golang.org/x/crypto/openpgp/clearsign"
	"golang.org/x/oauth2"
)

var httpAddr = flag.String("httpAddr", ":5000", "HTTP Address to listen to")

var allowedOwners []string
var client *github.Client
var signingKey *openpgp.Entity

func isOwnerAllowed(owner string) bool {
	for _, allowedOwner := range allowedOwners {
		if allowedOwner == owner {
			return true
		}
	}
	return false
}

func handleError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}

	http.Error(w, err.Error(), http.StatusInternalServerError)
	return true
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

	releases, resp, err := client.Repositories.ListReleases(vars["owner"], vars["repo"], nil)
	if err != nil {
		return err
	}

	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(resp.Rate.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(resp.Rate.Remaining))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resp.Rate.Reset.Unix(), 10))

	// do trigger loading of all packages
	err = iteratePackages(releases, vars["distribution"], func(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
		go func(release github.RepositoryRelease, asset github.ReleaseAsset) {
			packages.get(&release, &asset)
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

func getPackages(w http.ResponseWriter, r *http.Request) (*packageRepository, error) {
	repository := &packageRepository{}

	err := enumeratePackages(w, r, func(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
		if release.Prerelease != nil && *release.Prerelease {
			return nil
		}

		repository.add(release, asset)
		return nil
	})

	repository.sort()

	return repository, err
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
	githubUrl := "https://github.com/" + vars["owner"] + "/" + vars["repo"] + "/releases"

	fmt.Fprintln(w, "<h2>Welcome to automated Debian Repository made on top of GitHub Releases</h2>")
	fmt.Fprintln(w, "This repository is built for: ")
	fmt.Fprintf(w, `<a href=%q>%s</a><br>`, githubUrl, githubUrl)

	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>1. Add a repository key:</h4>")
	fmt.Fprintln(w, "<code>$ curl -fsSL "+url+"/archive.key | sudo apt-key add -</code>")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>2. Add stable repository:</h4>")
	fmt.Fprintln(w, `<code>$ sudo sudo: add-apt-repository: command not found "deb `+url+`/releases /"</code>`)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>3. (optionally) Add pre-release repository:</h4>")
	fmt.Fprintln(w, `<code>$ sudo sudo: add-apt-repository: command not found "deb `+url+`/pre-releases /"</code>`)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>4. Update apt:</h4>")
	fmt.Fprintln(w, `<code>$ sudo apt-get update</code>`)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "<h4>You can view the status of all packages at:</h4>")
	fmt.Fprintf(w, `<a href=%q>%s</a><br>`, url+"/releases", url+"/releases")
	fmt.Fprintf(w, `<a href=%q>%s</a><br>`, url+"/pre-releases", url+"/pre-releases")
	fmt.Fprintln(w)
}

func distributionIndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	fmt.Fprintln(w, "List of packages:")

	err := enumeratePackages(w, r, func(release *github.RepositoryRelease, asset *github.ReleaseAsset) error {
		_, err := packages.get(release, asset)
		fmt.Fprintln(w, "Package:", *release.TagName, "/", *asset.Name, "prerelase:", *release.Prerelease, "status:", err)
		return nil
	})
	if err != nil {
		fmt.Fprintln(w, "enumerate error:", err)
	}
}

func archiveKeyHandler(w http.ResponseWriter, r *http.Request) {
	wd, err := armor.Encode(w, openpgp.PublicKeyType, nil)
	if handleError(w, err) {
		return
	}
	defer wd.Close()

	signingKey.Serialize(wd)
}

func packagesHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getPackages(w, r)
	if handleError(w, err) {
		return
	}

	repository.write(w)
}

func packagesGzHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getPackages(w, r)
	if handleError(w, err) {
		return
	}

	w.Header().Set("Content-Encoding", "gzip")

	repository.writeGz(w)
}

func releaseHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getPackages(w, r)
	if handleError(w, err) {
		return
	}

	repository.writeRelease(w)
}

func releaseGpgHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getPackages(w, r)
	if handleError(w, err) {
		return
	}

	pr, pw := io.Pipe()
	defer pr.Close()

	go func() {
		defer pw.Close()
		repository.writeRelease(pw)
	}()

	openpgp.ArmoredDetachSign(w, signingKey, pr, nil)
}

func inReleaseHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getPackages(w, r)
	if handleError(w, err) {
		return
	}

	wd, err := clearsign.Encode(w, signingKey.PrivateKey, nil)
	if handleError(w, err) {
		return
	}
	defer wd.Close()

	repository.writeRelease(wd)
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s",
		vars["owner"], vars["repo"],
		vars["tag_name"], vars["file_name"])
	http.Redirect(w, r, url, http.StatusPermanentRedirect)
}

func main() {
	flag.Parse()

	if githubToken := os.Getenv("GITHUB_TOKEN"); githubToken != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: githubToken},
		)
		tc := oauth2.NewClient(ctx, ts)
		client = github.NewClient(tc)
		log.Println("Using GITHUB_TOKEN.")
	} else {
		client = github.NewClient(nil)
		log.Println("Using Public API. You may want to pass GITHUB_TOKEN.")
	}

	entityList, err := openpgp.ReadArmoredKeyRing(bytes.NewBufferString(os.Getenv("GPG_KEY")))
	if err != nil {
		log.Fatalln("Failed to parse environment GPG_KEY:", err)
	}
	if len(entityList) != 1 {
		log.Fatalln("Exactly one entity should be in GPG_KEY. Was:", len(entityList))
	}

	signingKey = entityList[0]
	allowedOwners = strings.Split(os.Getenv("ALLOWED_ORGS"), ",")

	if len(allowedOwners) == 0 {
		log.Println("Allowed owners: none")
	} else {
		log.Println("Allowed owners:", strings.Join(allowedOwners, ", "))
	}

	r := mux.NewRouter()
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
	r.HandleFunc("/{owner}/{repo}/{distribution}/download/{tag_name}/{file_name}", downloadHandler).Methods("GET")

	loggingHandler := NewApacheLoggingHandler(r, os.Stdout)
	http.Handle("/", loggingHandler)

	log.Println("Starting web-server on", *httpAddr, "...")
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}
