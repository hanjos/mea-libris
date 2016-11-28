package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang/gddo/httputil"
	"github.com/gorilla/sessions"
	"github.com/hanjos/mea-libris/libris"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/books/v1"
	"strings"
)

var (
	sessionName = "sessionName"

	clientID     = os.Getenv("CLIENT_ID")
	clientSecret = os.Getenv("CLIENT_SECRET")
	redirectURL  = os.Getenv("REDIRECT_URL")
	port         = os.Getenv("PORT")

	config = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURL,
		Scopes:       []string{books.BooksScope},
	}

	store = sessions.NewCookieStore([]byte(randomString()))
)

func _index(w http.ResponseWriter, r *http.Request) *appError {
	log.Println("Handling /")

	return nil
}

func _google(w http.ResponseWriter, r *http.Request) *appError {
	log.Println("Handling /google")

	session, err := store.Get(r, sessionName)
	if err != nil {
		// ignore session errors
		//return errSessionNotFound(session, err)
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
	log.Println("Handling /google/connect")

	session, err := store.Get(r, sessionName)
	if err != nil {
		//return errSessionError(session, err)
	}

	_, ok := session.Values["accessToken"].(string)
	if ok {
		log.Println("User authenticated and authorized.")
		fmt.Fprintln(w, "Connected!") // XXX w.WriteHeader(http.StatusOK) is implicit
		return nil
	}

	log.Println("User not authorized; beginning auth exchange")
	log.Println("Generating a new state")
	state := randomString()

	session.Values["state"] = state
	session.Save(r, w)

	url := config.AuthCodeURL(state)

	log.Println("Redirecting to Google's OAuth servers for a code")
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	return nil
}

func newBooksClient(ctx context.Context, token string) (*books.Service, error) {
	log.Println("Using the access token to build a Google Books client")

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
	log.Print("Getting the user's books")

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

	log.Printf("%d books processed (of a total of %d)\n", len(myBooks), totalItems)
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
	log.Printf("Requested response format: %s\n", r.Header.Get("Accept"))

	contentType := httputil.NegotiateContentType(r,
		[]string{"application/json", "text/csv", "application/csv"},
		"application/json")

	log.Printf("Negotiated content type: %s\n", contentType)
	switch contentType {
	case "application/json":
		return encodeBooksAsJSON(books, w)
	case "application/csv":
		fallthrough
	case "text/csv":
		return encodeBooksAsCSV(books, w)
	default:
		log.Printf("Unexpected content type %s; rendering as application/json", contentType)
		return encodeBooksAsJSON(books, w)
	}
}

func encodeBooksAsJSON(books []*libris.Book, w io.Writer) error {
	log.Println("Encoding books as JSON")

	// XXX setting headers has do be done BEFORE writing the body, or it'll be ignored!
	if rw, ok := w.(http.ResponseWriter); ok {
		rw.Header().Set("Content-Type", "application/json;charset=utf-8")
	}

	booksJSON, err := json.Marshal(books)
	if err != nil {
		return errCantMarshalBooksToJSON(err)
	}

	_, err2 := fmt.Fprintf(w, "%s", booksJSON)
	if err2 != nil {
		return errCantWriteResponse(err2)
	}

	return nil
}

func encodeBooksAsCSV(books []*libris.Book, w io.Writer) error {
	log.Println("Encoding books as CSV")

	// XXX setting headers has do be done BEFORE writing the body, or it'll be ignored!
	if rw, ok := w.(http.ResponseWriter); ok {
		rw.Header().Set("Content-Type", "text/csv;charset=utf-8")
	}

	err := libris.Books(books).EncodeCSV(w)
	if err != nil {
		return err
	}

	return nil
}

func _googleOAuthCallback(w http.ResponseWriter, r *http.Request) *appError {
	log.Println("Handling /google/oauth2callback")
	log.Println("Validating state")

	session, err := store.Get(r, sessionName)
	if err != nil {
		//return errSessionError(session, err)
	}

	sessionState, ok := session.Values["state"].(string)
	if !ok || r.FormValue("state") != sessionState {
		return errWrap(errInvalidState(sessionState, r.FormValue("state")), _status(http.StatusBadRequest))
	}

	log.Println("Extracting the code")
	code := r.FormValue("code")
	if code == "" {
		return errWrap(errCodeNotFound, _status(http.StatusBadRequest))
	}

	defer func() {
		session.Values["state"] = nil // XXX state is a one-time value; we don't need it anymore
	}()

	log.Println("Exchanging the code for an access token")
	token, err := config.Exchange(context.Background(), code)
	if err != nil {
		return errWrap(errTokenExchangeError(err), _status(http.StatusBadRequest))
	}

	session.Values["accessToken"] = token.AccessToken // XXX can't store a *oauth2.Token, so store a string
	session.Save(r, w)

	http.Redirect(w, r, "/google/connect", http.StatusTemporaryRedirect)
	return nil
}

// MAIN
func main() {
	http.Handle("/", appHandler(_index))
	http.Handle("/google", appHandler(_google))
	http.Handle("/google/connect", appHandler(_googleConnect))
	http.Handle("/google/oauth2callback", appHandler(_googleOAuthCallback))

	log.Printf("Starting server on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}

// session
type authSession struct {
	State string
	Code  string
}

// appHandler
type appHandler func(http.ResponseWriter, *http.Request) *appError

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		log.Println(err)
		http.Error(w, err.Message, err.Status)
	}
}

// general helpers
func randomString() string {
	return fmt.Sprintf("st%d", time.Now().UnixNano())
}

// appError
type appError struct {
	Message string
	Status  int
}

func (err appError) Error() string {
	return fmt.Sprintf("Error [%d]: %s", err.Status, err.Message)
}

type appErrorField func(appErr *appError)

func _prefix(str string) appErrorField {
	return func(appErr *appError) {
		if appErr == nil {
			return
		}

		if appErr.Message == "" {
			appErr.Message = str
			return
		}

		appErr.Message = str + ": " + appErr.Message
	}
}

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

func errSessionError(s *sessions.Session, err error) error {
	return fmt.Errorf("Error on session %v : %v", s, err)
}

var errAccessTokenNotFound = fmt.Errorf("User not authorized! Use the /google/connect endpoint.")

var errCodeNotFound = fmt.Errorf("Code not found!")

func errTokenExchangeError(err error) error {
	return fmt.Errorf("Problem with token exchange: %v", err)
}

func errCantLoadBooksClient(err error) error {
	return fmt.Errorf("Couldn't load Google Books client: %v", err)
}

func errCantLoadVolumes(err error) error {
	return fmt.Errorf("Couldn't load the user's volumes: %v", err)
}

func errCantMarshalBooksToJSON(err error) error {
	return fmt.Errorf("Couldn't marshal the books' info to JSON: %v", err)
}

func errCantWriteResponse(err error) error {
	return fmt.Errorf("Couldn't write response: %v", err)
}
