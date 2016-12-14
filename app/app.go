/*
Package app defines the different pieces of the application, which are all put together in main.
*/
package app

import (
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// Handler is an http.Handler which runs the given function and sends the data in *Error, if any, to http.Error, with
// the proper status code.
type Handler func(w http.ResponseWriter, r *http.Request) *Error

// ServeHTTP implements the http.Handler interface.
func (fn Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		http.Error(w, err.Message, err.Status)
	}
}

// Error represents an error in processing, which will be returned from an app.Handler and converted into the
// appropriate HTTP status code and message.
type Error struct {
	Message string
	Status  int
}

// Error implements the error interface.
func (err *Error) Error() string {
	return fmt.Sprintf("[%d %s] %s", err.Status, http.StatusText(err.Status), err.Message)
}

// Wrap builds an app.Error from an error and status code. If err is nil or an *app.Error, it will be returned
// unmodified.
func Wrap(err error, status int) *Error {
	if err == nil {
		return nil
	}

	if appErr, ok := err.(*Error); ok {
		return appErr
	}

	appErr := &Error{err.Error(), status}

	return appErr
}

// Client is an interface which provides access to a specific OAuth-based provider.
type Client interface {
	// Config returns an *oauth2.Config configured to reach a OAuth provider.
	Config() *oauth2.Config
}

// Service is an interface which provides app.Handler-compatible methods, which will handle user requests.
type Service interface {
	// HandleBooks lists the user's books. Works only if the user was previously authenticated and authorized with
	// HandleConnect.
	HandleBooks(w http.ResponseWriter, r *http.Request) *Error

	// HandleConnect starts the OAuth flow, redirecting to the provider. The provider's response should be answered by
	// HandleOAuthCallback.
	HandleConnect(w http.ResponseWriter, r *http.Request) *Error

	// HandleDisconnect revokes this application's access to the user's data.
	HandleDisconnect(w http.ResponseWriter, r *http.Request) *Error

	// HandleOAuthCallback should be called by the OAuth provider with its answer to the auth attempt started in
	// HandleConnect.
	HandleOAuthCallback(w http.ResponseWriter, r *http.Request) *Error
}

// Router is an interface used to determine the endpoints which will be routed to an app.Service's methods.
type Router interface {
	// Route builds an endpoint to the given path.
	Route(path string) string

	// Books returns the endpoint which retrieves the user's books.
	Books() string

	// Connect returns an endpoint which starts the OAuth flow to authorize this app.
	Connect() string

	// Disconnect returns an endpoint which revoke's this application's access to the user's data.
	Disconnect() string

	// OAuthCallback returns an endpoint which will be called by the OAuth provider to end the OAuth flow.
	OAuthCallback() string
}

type defaultClient struct {
	config *oauth2.Config
}

// NewClient creates an app.Client which stores and returns a previously built *oauth2.Config.
func NewClient(config *oauth2.Config) Client {
	return &defaultClient{
		config: config,
	}
}

// Config implements the app.Client interface.
func (c *defaultClient) Config() *oauth2.Config {
	return c.config
}

type defaultService struct{}

// NewService creates a default app.Service with empty implementations.
func NewService() Service {
	return &defaultService{}
}

// HandleBooks implements the app.Service interface, with an empty implementation.
func (s *defaultService) HandleBooks(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

// HandleConnect implements the app.Service interface, with an empty implementation.
func (s *defaultService) HandleConnect(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

// HandleDisconnect implements the app.Service interface, with an empty implementation.
func (s *defaultService) HandleDisconnect(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

// HandleOAuthCallback implements the app.Service interface, with an empty implementation.
func (s *defaultService) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) *Error {
	return nil
}

type defaultRouter struct {
	pathPrefix string
}

// NewRouter creates a new app.Router which uses pre-determined routes for every method, prefixed by a given path.
func NewRouter(pathPrefix string) Router {
	return &defaultRouter{
		pathPrefix: pathPrefix,
	}
}

// Route implements the app.Router interface, prepending the default path prefix to the given path.
func (r *defaultRouter) Route(path string) string {
	return r.pathPrefix + path
}

// Books implements the app.Router interface, returning "<default path prefix>/".
func (r *defaultRouter) Books() string {
	return r.Route("/")
}

// Connect implements the app.Router interface, returning "<default path prefix>/connect".
func (r *defaultRouter) Connect() string {
	return r.Route("/connect")
}

// Disconnect implements the app.Router interface, returning "<default path prefix>/disconnect".
func (r *defaultRouter) Disconnect() string {
	return r.Route("/disconnect")
}

// OAuthCallback implements the app.Router interface, returning "<default path prefix>/oauth2connect".
func (r *defaultRouter) OAuthCallback() string {
	return r.Route("/oauth2callback")
}
