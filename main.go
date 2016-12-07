/*
mea-libris starts a web server which shows your books. Right now, Google Books is the only service supported.

It needs 2 environment variables to function: GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET, which are this app's Google
credentials. They are necessary to reach your Google books via OAuth.

mea-libris will use other 2 environment variables if available:

	PORT: the port which this server will listen to. Defaults to 8080.

	GOOGLE_REDIRECT_URL: the URL Google's OAuth server will respond to, as part of
	  the OAuth authorization flow. Defaults to
	  (request.URL.Scheme || http)://(request.Host)/google/oauth2callback.

More details at https://github.com/hanjos/mea-libris .
*/
package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang/gddo/httputil"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/hanjos/mea-libris/app"
	"github.com/hanjos/mea-libris/libris"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/books/v1"
)

var (
	sessionName = "sessionName"

	clientID     = os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret = os.Getenv("GOOGLE_CLIENT_SECRET")
	port         = defaultValue(os.Getenv("PORT"), "8080")

	goog = newGoogleService(clientID, clientSecret)

	store = sessions.NewCookieStore([]byte(randomString()))

	// No date or time; an external router can consume this log and provide that
	logOut = log.New(os.Stdout, "[mea-libris] ", 0)
	logErr = log.New(os.Stderr, "[mea-libris] ", 0)
)

// SERVICES
type googleService struct {
	app.Service
	app.Router
	app.Client
}

func newGoogleService(clientID, clientSecret string) app.Service {
	s := &googleService{
		app.NewService(),
		app.NewRouter("/google"),
		app.NewClient(
			&oauth2.Config{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				Endpoint:     google.Endpoint,
				Scopes:       []string{books.BooksScope},
			}),
	}

	return s
}

func (goog *googleService) HandleBooks(w http.ResponseWriter, r *http.Request) *app.Error {
	session, err := store.Get(r, sessionName)
	if err != nil {
		// TODO ignoring session errors
		//return app.Wrap(errSessionError(sessionName, err), http.StatusInternalServerError)
	}

	token, ok := session.Values["accessToken"].(string)
	if !ok {
		return app.Wrap(errAccessTokenNotFound, http.StatusUnauthorized)
	}

	svc, err := newGoogleBooksClient(goog.Config(), context.Background(), token)
	if err != nil {
		return app.Wrap(err, http.StatusInternalServerError)
	}

	bs, err := getGoogleBooks(svc)
	if err != nil {
		return app.Wrap(err, http.StatusInternalServerError)
	}

	err = encodeBooks(bs, w, r)
	if err != nil {
		return app.Wrap(err, http.StatusInternalServerError)
	}

	return nil
}

func (goog *googleService) HandleConnect(w http.ResponseWriter, r *http.Request) *app.Error {
	session, err := store.Get(r, sessionName)
	if err != nil {
		// TODO ignoring session errors
		//return app.Wrap(errSessionError(sessionName, err), http.StatusInternalServerError)
	}

	_, ok := session.Values["accessToken"].(string)
	if ok {
		logOut.Println("User authenticated and authorized.")
		fmt.Fprintln(w, "Connected!") // XXX w.WriteHeader(http.StatusOK) is implicit
		return nil
	}

	logOut.Println("User not authorized; beginning auth exchange")
	logOut.Println("Generating a new state")
	state := randomString()
	session.Values["state"] = state
	session.Save(r, w)

	config := goog.Config()
	redirectURL := defaultValue(os.Getenv("GOOGLE_REDIRECT_URL"), app.BuildRedirectURL(r, goog))
	logOut.Printf("The redirect URL is %v\n", redirectURL)
	config.RedirectURL = redirectURL
	url := config.AuthCodeURL(state)

	logOut.Println("Redirecting to Google's OAuth servers for a code")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	return nil
}

func (goog *googleService) HandleDisconnect(w http.ResponseWriter, r *http.Request) *app.Error {
	session, err := store.Get(r, sessionName)
	if err != nil {
		// TODO ignoring session errors
		//return app.Wrap(errSessionError(sessionName, err), http.StatusInternalServerError)
	}

	token, ok := session.Values["accessToken"].(string)
	if !ok {
		logOut.Println("User wasn't connected. Nothing was done.")
		fmt.Fprintln(w, "User wasn't connected. Nothing was done.")
		return nil
	}

	logOut.Println("Disconnecting the current user")
	url := "https://accounts.google.com/o/oauth2/revoke?token=" + token
	resp, err := http.Get(url)
	defer resp.Body.Close()
	if err != nil {
		return app.Wrap(errCantRevokeToken(err), http.StatusInternalServerError)
	}

	logOut.Println("Resetting the session")
	session.Values["state"] = nil
	session.Values["accessToken"] = nil
	session.Save(r, w)

	fmt.Fprintln(w, "User disconnected!")
	return nil
}

func (goog *googleService) HandleOAuthCallback(w http.ResponseWriter, r *http.Request) *app.Error {
	logOut.Println("Validating the state")

	session, err := store.Get(r, sessionName)
	if err != nil {
		// TODO ignoring session errors
		//return app.Wrap(errSessionError(sessionName, err), http.StatusInternalServerError)
	}

	sessionState, ok := session.Values["state"].(string)
	if !ok || r.FormValue("state") != sessionState {
		return app.Wrap(errInvalidState(sessionState, r.FormValue("state")), http.StatusBadRequest)
	}

	logOut.Println("Checking for errors")
	if errMsg := r.FormValue("error"); errMsg != "" {
		return app.Wrap(errCallbackError(errMsg), http.StatusUnauthorized)
	}

	logOut.Println("Reading the code")
	code := r.FormValue("code")
	if code == "" {
		return app.Wrap(errCodeNotFound, http.StatusBadGateway)
	}

	defer func() {
		session.Values["state"] = nil // XXX state is a one-time value; we don't need it anymore
	}()

	logOut.Println("Exchanging the code for an access token")
	config := goog.Config()
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return app.Wrap(errTokenExchangeError(err), http.StatusInternalServerError)
	}

	session.Values["accessToken"] = token.AccessToken // XXX can't store a *oauth2.Token, so we store a string
	session.Save(r, w)

	connectEndpoint := goog.Route("/connect")
	logOut.Printf("Redirecting to %v to finish the auth process\n", connectEndpoint)
	http.Redirect(w, r, connectEndpoint, http.StatusTemporaryRedirect)
	return nil
}

// STEP FUNCTIONS
func newGoogleBooksClient(config *oauth2.Config, ctx context.Context, token string) (*books.Service, error) {
	logOut.Println("Using the access token to build a Google Books client")

	tok := new(oauth2.Token)
	tok.AccessToken = token

	client := config.Client(ctx, tok)
	svc, err := books.New(client)
	if err != nil {
		return nil, errCantLoadBooksClient(err)
	}

	return svc, nil
}

func getGoogleBooks(svc *books.Service) ([]*libris.Book, error) {
	logOut.Print("Getting the user's books")

	myBooks := []*libris.Book{}
	nextIndex, totalItems := int64(0), int64(0)
	for {
		volumes, err := svc.Volumes.Mybooks.List().
			StartIndex(nextIndex).
			AcquireMethod("FAMILY_SHARED", "PREORDERED", "PUBLIC_DOMAIN", "PURCHASED", "RENTED", "SAMPLE", "UPLOADED").
			ProcessingState("COMPLETED_SUCCESS").
			Do()
		if err != nil {
			return nil, errCantLoadVolumes(err)
		}

		for _, v := range volumes.Items {
			myBooks = append(myBooks, newBook(v))
		}

		nextIndex, totalItems = nextIndex+int64(len(volumes.Items)), volumes.TotalItems
		if nextIndex >= totalItems {
			// XXX since there's no do-while, here we 'go'
			break
		}
	}

	logOut.Printf("%d books processed (of a total of %d)\n", len(myBooks), totalItems)
	return myBooks, nil
}

func newBook(v *books.Volume) *libris.Book {
	info := v.VolumeInfo

	// resolving the identification
	var id, idType string

	for _, identifier := range info.IndustryIdentifiers {
		if identifier.Identifier == "" {
			continue
		}

		id = identifier.Identifier
		idType = identifier.Type
		break
	}

	// getting the file type
	fileType := "UNKNOWN"
	if v.AccessInfo.Pdf != nil {
		fileType = "PDF"
	} else if v.AccessInfo.Epub != nil {
		fileType = "EPUB"
	}

	// removing the extension from the title if it's there
	title := info.Title
	if strings.HasSuffix(strings.ToLower(title), ".pdf") && fileType == "PDF" {
		title = title[:len(title)-4]
	} else if strings.HasSuffix(strings.ToLower(title), ".epub") && fileType == "EPUB" {
		title = title[:len(title)-5]
	}

	return &libris.Book{
		Title:          title,
		Authors:        info.Authors,
		Identifier:     id,
		IdentifierType: idType,
		AverageRating:  info.AverageRating,
		Publisher:      info.Publisher,
		FileType:       fileType,
	}
}

func encodeBooks(books []*libris.Book, w io.Writer, r *http.Request) error {
	logOut.Printf("Requested response format: %s\n", r.Header.Get("Accept"))

	contentType := httputil.NegotiateContentType(r,
		[]string{"application/json", "text/csv", "application/csv"},
		"application/json")

	logOut.Printf("Negotiated content type: %s\n", contentType)
	switch contentType {
	case "application/json":
		return encodeBooksAsJSON(books, w)
	case "application/csv":
		fallthrough
	case "text/csv":
		return encodeBooksAsCSV(books, w)
	default:
		logOut.Printf("Unexpected content type %s; rendering as application/json", contentType)
		return encodeBooksAsJSON(books, w)
	}
}

func encodeBooksAsJSON(books []*libris.Book, w io.Writer) error {
	logOut.Println("Encoding books as JSON")

	// XXX setting headers has do be done BEFORE writing the body, or it'll be ignored!
	if rw, ok := w.(http.ResponseWriter); ok {
		rw.Header().Set("Content-Type", "application/json;charset=utf-8")
	}

	err := libris.Books(books).EncodeJSON(w)
	if err != nil {
		return errCantEncodeBooks(err)
	}

	return nil
}

func encodeBooksAsCSV(books []*libris.Book, w io.Writer) error {
	logOut.Println("Encoding books as CSV")

	// XXX setting headers has do be done BEFORE writing the body, or it'll be ignored!
	if rw, ok := w.(http.ResponseWriter); ok {
		rw.Header().Set("Content-Type", "text/csv;charset=utf-8")
	}

	err := libris.Books(books).EncodeCSV(w)
	if err != nil {
		return errCantEncodeBooks(err)
	}

	return nil
}

// MAIN
func main() {
	r := mux.NewRouter().StrictSlash(true) // XXX so that /google and /google/ match

	if gRout, ok := goog.(app.Router); ok {
		stack := logging

		r.Handle(gRout.Books(), stack(app.Handler(goog.HandleBooks)))
		r.Handle(gRout.Connect(), stack(app.Handler(goog.HandleConnect)))
		r.Handle(gRout.Disconnect(), stack(app.Handler(goog.HandleDisconnect)))
		r.Handle(gRout.OAuthCallback(), stack(app.Handler(goog.HandleOAuthCallback)))
	} else {
		logErr.Fatalln("No Google endpoints handled!")
	}

	logOut.Printf("Starting server on port %s\n", port)
	http.ListenAndServe(":"+port, r)
}

// HANDLERS & MIDDLEWARES

// logging middleware
type statusResponseWriter interface {
	http.ResponseWriter
	http.Flusher

	Status() int
}

type statusResponseLogger struct {
	w      http.ResponseWriter
	status int
}

func (s *statusResponseLogger) Header() http.Header {
	return s.w.Header()
}

func (s *statusResponseLogger) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}

	return s.w.Write(b)
}

func (s *statusResponseLogger) WriteHeader(status int) {
	s.w.WriteHeader(status)
	s.status = status
}

func (s *statusResponseLogger) Status() int {
	return s.status
}

func (s *statusResponseLogger) Flush() {
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
}

func logging(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lw := &statusResponseLogger{w: w, status: http.StatusOK}

		h.ServeHTTP(lw, r)

		if lw.Status() >= 400 {
			logErr.Printf("Error %d: %s\n", lw.Status(), http.StatusText(lw.Status()))
		}
	})
}

// APPLICATION ERRORS
func errInvalidState(expected, actual string) error {
	return fmt.Errorf("Invalid state parameter: expected %s; got %s", expected, actual)
}

func errCallbackError(message string) error {
	return fmt.Errorf("Callback received error: %v", message)
}

var errCodeNotFound = errors.New("Code not found.")

var errAccessTokenNotFound = errors.New("User not authorized. Use the /google/connect endpoint.")

func errTokenExchangeError(err error) error {
	return fmt.Errorf("Problem with token exchange: %v", err)
}

func errCantLoadBooksClient(err error) error {
	return fmt.Errorf("Couldn't load Google Books client: %v", err)
}

func errCantLoadVolumes(err error) error {
	return fmt.Errorf("Couldn't load the user's volumes: %v", err)
}

func errCantEncodeBooks(err error) error {
	return fmt.Errorf("Couldn't encode the books: %v", err)
}

func errCantRevokeToken(err error) error {
	return fmt.Errorf("Failed to revoke token for the current user: %v", err)
}

// UTILITIES
func randomString() string {
	return fmt.Sprintf("st%d", time.Now().UnixNano())
}

func defaultValue(v string, def string) string {
	if v == "" {
		return def
	}

	return v
}
