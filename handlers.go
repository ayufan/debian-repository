package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/gorilla/mux"

	"github.com/ayufan/debian-repository/internal/github_client"
	"github.com/ayufan/debian-repository/internal/http_helpers"
)

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

	err := enumeratePackages(w, r, func(ghPackage github_client.Package) error {
		p, err := packagesCache.Get(ghPackage)
		fmt.Fprintln(w, "Package:", *ghPackage.Release.TagName, "/", *ghPackage.Asset.Name)
		fmt.Fprintln(w, "\tIsPrerelease:", *ghPackage.Release.Prerelease)
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
	repository, err := getRepository(w, r)
	if http_helpers.HandleError(w, err) {
		return
	}

	repository.Write(w)
}

func packagesGzHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getRepository(w, r)
	if http_helpers.HandleError(w, err) {
		return
	}

	w.Header().Set("Content-Type", "binary/octet-stream")

	repository.WriteGz(w)
}

func releaseHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getRepository(w, r)
	if http_helpers.HandleError(w, err) {
		return
	}

	repository.WriteRelease(w)
}

func releaseGpgHandler(w http.ResponseWriter, r *http.Request) {
	repository, err := getRepository(w, r)
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
	repository, err := getRepository(w, r)
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
