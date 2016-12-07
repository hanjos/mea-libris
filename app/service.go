package app

import (
	"net/http"

	"golang.org/x/oauth2"
	"log"
)

type Client interface {
	Config() *oauth2.Config
}

type Service interface {
	HandleBooks(w http.ResponseWriter, r *http.Request) *Error
	HandleConnect(w http.ResponseWriter, r *http.Request) *Error
	HandleDisconnect(w http.ResponseWriter, r *http.Request) *Error
	HandleOAuthCallback(w http.ResponseWriter, r *http.Request) *Error
}

type Router interface {
	Route(path string) string

	Books() string
	Connect() string
	Disconnect() string
	OAuthCallback() string
}

type defaultClient struct {
	config *oauth2.Config
}

func NewClient(config *oauth2.Config) Client {
	return &defaultClient{
		config: config,
	}
}

func (c *defaultClient) Config() *oauth2.Config {
	return c.config
}

type defaultService struct{}

func NewService() Service {
	return &defaultService{}
}

func (s *defaultService) HandleBooks(w http.ResponseWriter, r *http.Request) *Error {
	log.Println("defaultService: /books")
	return nil
}

func (s *defaultService) HandleConnect(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

func (s *defaultService) HandleDisconnect(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

func (s *defaultService) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

type defaultRouter struct {
	pathPrefix string
}

func NewRouter(pathPrefix string) Router {
	return &defaultRouter{
		pathPrefix: pathPrefix,
	}
}

func (r *defaultRouter) Route(path string) string {
	return r.pathPrefix + path
}

func (r *defaultRouter) Books() string {
	return r.Route("")
}

func (r *defaultRouter) Connect() string {
	return r.Route("/connect")
}

func (r *defaultRouter) Disconnect() string {
	return r.Route("/disconnect")
}

func (r *defaultRouter) OAuthCallback() string {
	return r.Route("/oauth2callback")
}

func BuildRedirectURL(r *http.Request, router Router) string {
	scheme := r.URL.Scheme // this may be empty, use 'http' by default
	if scheme == "" {
		scheme = "http"
	}

	return scheme + "://" + r.Host + router.OAuthCallback()
}
