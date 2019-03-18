package github_client

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/google/go-github/github"
	"github.com/patrickmn/go-cache"
	"golang.org/x/oauth2"
)

var listOptions = github.ListOptions{PerPage: 10000}

type API struct {
	client       *github.Client
	requestCache *cache.Cache
}

func (a *API) Flush() {
	a.requestCache.Flush()
}

func New(token string, cacheExpiration time.Duration) *API {
	var httpClient *http.Client

	if token != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		httpClient = oauth2.NewClient(ctx, ts)
		log.Println("Using GITHUB_TOKEN.")
	} else {
		log.Println("Using Public API. You may want to pass GITHUB_TOKEN.")
	}

	return &API{
		client:       github.NewClient(httpClient),
		requestCache: cache.New(cacheExpiration, time.Minute),
	}
}
