package app

import (
	"net/http"

	"golang.org/x/oauth2"
)

// Client is an interface used to access OAuth-based services.
type Client interface {
	Config() *oauth2.Config
}

// Service is an interface used to handle book-listing requests. The methods are compatible with Handler, not
// http.Handler.
type Service interface {
	HandleBooks(w http.ResponseWriter, r *http.Request) *Error
	HandleConnect(w http.ResponseWriter, r *http.Request) *Error
	HandleDisconnect(w http.ResponseWriter, r *http.Request) *Error
	HandleOAuthCallback(w http.ResponseWriter, r *http.Request) *Error
}

// Router is an interface used to determine endpoints for Services.
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

// NewClient creates a Client which stores and returns a previously built *oauth2.Config.
func NewClient(config *oauth2.Config) Client {
	return &defaultClient{
		config: config,
	}
}

// Config implements the Client interface.
func (c *defaultClient) Config() *oauth2.Config {
	return c.config
}

type defaultService struct{}

// NewService creates a default Service, with empty implementations.
func NewService() Service {
	return &defaultService{}
}

// HandleBooks implements the Service interface, with an empty implementation.
func (s *defaultService) HandleBooks(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

// HandleConnect implements the Service interface, with an empty implementation.
func (s *defaultService) HandleConnect(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

// HandleDisconnect implements the Service interface, with an empty implementation.
func (s *defaultService) HandleDisconnect(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

// HandleOAuthCallback implements the Service interface, with an empty implementation.
func (s *defaultService) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

type defaultRouter struct {
	pathPrefix string
}

// NewRouter creates a new Router, which uses pre-determined routes for every method, prefixed by a given path.
func NewRouter(pathPrefix string) Router {
	return &defaultRouter{
		pathPrefix: pathPrefix,
	}
}

// Route implements the Router interface, prepending the default path prefix to the given path.
func (r *defaultRouter) Route(path string) string {
	return r.pathPrefix + path
}

// Books implements the Router interface, returning the default path prefix.
func (r *defaultRouter) Books() string {
	return r.Route("")
}

// Connect implements the Router interface, returning "<default path prefix>/connect".
func (r *defaultRouter) Connect() string {
	return r.Route("/connect")
}

// Disconnect implements the Router interface, returning "<default path prefix>/disconnect".
func (r *defaultRouter) Disconnect() string {
	return r.Route("/disconnect")
}

// OAuthCallback implements the Router interface, returning "<default path prefix>/oauth2connect".
func (r *defaultRouter) OAuthCallback() string {
	return r.Route("/oauth2callback")
}

// BuildRedirectURL builds a prospective redirect URL, given a request and a Router.
func BuildRedirectURL(r *http.Request, router Router) string {
	scheme := r.URL.Scheme // use 'http' if this is empty
	if scheme == "" {
		scheme = "http"
	}

	return scheme + "://" + r.Host + router.OAuthCallback()
}
