/*
mea-libris starts a web server which shows your books. Right now, Google Books is the only service supported.

It needs 2 environment variables to function: CLIENT_ID and CLIENT_SECRET, which are this app's Google credentials.
They are necessary to reach your Google books via OAuth.

mea-libris will use other 2 environment variables if available:

	PORT: the port which this server will listen to. Defaults to 8080.

	REDIRECT_URL: the URL Google's OAuth server will respond to, as part of
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
	"github.com/gorilla/sessions"
	"github.com/hanjos/mea-libris/libris"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/books/v1"
)

var (
	sessionName = "sessionName"

	clientID     = os.Getenv("CLIENT_ID")
	clientSecret = os.Getenv("CLIENT_SECRET")
	port         = defaultValue(os.Getenv("PORT"), "8080")

	config = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{books.BooksScope},
	}

	store = sessions.NewCookieStore([]byte(randomString()))

	// No date or time; an external router can consume this log and provide that
	logOut = log.New(os.Stdout, "[mea-libris] ", 0)
	logErr = log.New(os.Stderr, "[mea-libris] ", 0)
)

// ROUTES
func _google(w http.ResponseWriter, r *http.Request) *appError {
	session, err := store.Get(r, sessionName)
	if err != nil {
		// TODO ignoring session errors
		//return errWrap(errSessionError(sessionName, err), _status(http.StatusInternalServerError))
	}

	token, ok := session.Values["accessToken"].(string)
	if !ok {
		return errWrap(errAccessTokenNotFound, _status(http.StatusUnauthorized))
	}

	svc, err := newBooksClient(context.Background(), token)
	if err != nil {
		return errWrap(err, _status(http.StatusInternalServerError))
	}

	bs, err := getBooks(svc)
	if err != nil {
		return errWrap(err, _status(http.StatusInternalServerError))
	}

	err = encodeBooks(bs, w, r)
	if err != nil {
		return errWrap(err, _status(http.StatusInternalServerError))
	}

	return nil
}

func _googleConnect(w http.ResponseWriter, r *http.Request) *appError {
	session, err := store.Get(r, sessionName)
	if err != nil {
		// TODO ignoring session errors
		//return errWrap(errSessionError(sessionName, err), _status(http.StatusInternalServerError))
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

	redirectURL, how := getRedirectURL(r)
	logOut.Printf("%v\n", how)
	logOut.Printf("The redirect URL is %v\n", redirectURL)
	config.RedirectURL = redirectURL
	url := config.AuthCodeURL(state)

	logOut.Println("Redirecting to Google's OAuth servers for a code")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	return nil
}

func _googleDisconnect(w http.ResponseWriter, r *http.Request) *appError {
	session, err := store.Get(r, sessionName)
	if err != nil {
		// TODO ignoring session errors
		//return errWrap(errSessionError(sessionName, err), _status(http.StatusInternalServerError))
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
		return errWrap(errCantRevokeToken(err), _status(http.StatusInternalServerError))
	}

	logOut.Println("Resetting the session")
	session.Values["state"] = nil
	session.Values["accessToken"] = nil
	session.Save(r, w)

	fmt.Fprintln(w, "User disconnected!")
	return nil
}

func _googleOAuthCallback(w http.ResponseWriter, r *http.Request) *appError {
	logOut.Println("Validating the state")

	session, err := store.Get(r, sessionName)
	if err != nil {
		// TODO ignoring session errors
		//return errWrap(errSessionError(sessionName, err), _status(http.StatusInternalServerError))
	}

	sessionState, ok := session.Values["state"].(string)
	if !ok || r.FormValue("state") != sessionState {
		return errWrap(errInvalidState(sessionState, r.FormValue("state")), _status(http.StatusBadRequest))
	}

	logOut.Println("Checking for errors")
	if errMsg := r.FormValue("error"); errMsg != "" {
		return errWrap(errCallbackError(errMsg), _status(http.StatusUnauthorized))
	}

	logOut.Println("Reading the code")
	code := r.FormValue("code")
	if code == "" {
		return errWrap(errCodeNotFound, _status(http.StatusBadGateway))
	}

	defer func() {
		session.Values["state"] = nil // XXX state is a one-time value; we don't need it anymore
	}()

	logOut.Println("Exchanging the code for an access token")
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return errWrap(errTokenExchangeError(err), _status(http.StatusInternalServerError))
	}

	session.Values["accessToken"] = token.AccessToken // XXX can't store a *oauth2.Token, so we store a string
	session.Save(r, w)

	logOut.Println("Redirecting to /google/connect to finish the auth process")
	http.Redirect(w, r, "/google/connect", http.StatusTemporaryRedirect)
	return nil
}

// STEP FUNCTIONS
// getRedirectURL reads the given request and returns both a redirect URL, and how it was determined.
func getRedirectURL(r *http.Request) (string, string) {
	if fromEnv := os.Getenv("REDIRECT_URL"); fromEnv != "" {
		return fromEnv, "Using the environment variable REDIRECT_URL"
	}

	scheme := r.URL.Scheme // this may be empty, use 'http' by default
	if scheme == "" {
		scheme = "http"
	}

	return scheme + "://" + r.Host + "/google/oauth2callback", "Building the redirect URL from the request"
}

func newBooksClient(ctx context.Context, token string) (*books.Service, error) {
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

func getBooks(svc *books.Service) ([]*libris.Book, error) {
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
	mux := http.NewServeMux()

	mux.Handle("/google", appHandler(_google))
	mux.Handle("/google/connect", appHandler(_googleConnect))
	mux.Handle("/google/disconnect", appHandler(_googleDisconnect))
	mux.Handle("/google/oauth2callback", appHandler(_googleOAuthCallback))

	logOut.Printf("Starting server on port %s\n", port)
	http.ListenAndServe(":"+port, mux)
}

// HANDLERS & MIDDLEWARES

// appHandler runs the given function and sends the data in *appError, if any, to http.Error.
// Does nothing if *appError is nil.
type appHandler func(http.ResponseWriter, *http.Request) *appError

// ServeHTTP implements the http.Handler interface.
func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		logErr.Println(err)
		http.Error(w, err.Message, err.Status)
	}
}

// APPLICATION ERRORS
type appError struct {
	Message string
	Status  int
}

// Error implements the error interface.
func (err appError) Error() string {
	return fmt.Sprintf("[%d %s] %s", err.Status, http.StatusText(err.Status), err.Message)
}

type appErrorField func(appErr *appError)

func _status(status int) appErrorField {
	return func(appErr *appError) {
		if appErr == nil {
			return
		}

		appErr.Status = status
	}
}

func errWrap(err error, fields ...appErrorField) *appError {
	if err == nil {
		return nil
	}

	if appErr, ok := err.(*appError); ok {
		return appErr
	}

	appErr := &appError{err.Error(), http.StatusInternalServerError}
	for _, field := range fields {
		field(appErr)
	}

	return appErr
}

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
