package app

import (
	"net/http"

	"golang.org/x/oauth2"
)

type Service interface {
	Config() *oauth2.Config

	Books() Endpoint
	Connect() Endpoint
	Disconnect() Endpoint
	OAuthCallback() Endpoint
}

type Endpoint struct {
	Endpoint string
	Handler  Handler
}

func BuildRedirectURL(s Service, r *http.Request) string {
	scheme := r.URL.Scheme // this may be empty, use 'http' by default
	if scheme == "" {
		scheme = "http"
	}

	return scheme + "://" + r.Host + s.OAuthCallback().Endpoint
}

type DefaultService struct {
	Books_         Endpoint
	Connect_       Endpoint
	Disconnect_    Endpoint
	OAuthCallback_ Endpoint

	Config_ *oauth2.Config
}

func (g *DefaultService) Config() *oauth2.Config {
	return g.Config_
}

func (g *DefaultService) Books() Endpoint {
	return g.Books_
}

func (g *DefaultService) Connect() Endpoint {
	return g.Connect_
}

func (g *DefaultService) Disconnect() Endpoint {
	return g.Disconnect_
}

func (g *DefaultService) OAuthCallback() Endpoint {
	return g.OAuthCallback_
}
