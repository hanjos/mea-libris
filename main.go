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

	code, ok := getFlash(session, "code")
	session.Save(r, w)
	if !ok {
		err := connect(w, r)

		return errWrap(err, _status(http.StatusUnauthorized))
	}

	svc, err := newGoogleBooksClient(code)
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

func connect(w http.ResponseWriter, r *http.Request) error {
	log.Println("Beginning auth exchange")

	session, err := store.Get(r, sessionName)
	if err != nil {
		//return errSessionError(session, err)
	}

	state := randomString()

	session.AddFlash(state, "state")
	session.Save(r, w)

	url := config.AuthCodeURL(state)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
	return nil
}

func newGoogleBooksClient(code string) (*books.Service, error) {
	log.Println("Exchanging the code for an access token")

	ctx := context.Background()
	token, err := config.Exchange(ctx, code)
	if err != nil {
		return nil, errTokenExchangeError(err)
	}

	client := config.Client(ctx, token)
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

func _oauthCallback(w http.ResponseWriter, r *http.Request) *appError {
	log.Println("Handling /oauth2callback; validating state")

	session, err := store.Get(r, sessionName)
	if err != nil {
		//return errSessionError(session, err)
	}

	sessionState, ok := getFlash(session, "state")
	session.Save(r, w)
	if !ok || r.FormValue("state") != sessionState {
		return errWrap(errInvalidState(sessionState, r.FormValue("state")), _status(http.StatusBadRequest))
	}

	log.Println("Saving the code to the session")
	code := r.FormValue("code")
	if code == "" {
		return errWrap(errCodeNotFound, _status(http.StatusBadRequest))
	}

	session.AddFlash(code, "code")
	session.Save(r, w)

	http.Redirect(w, r, "/google", http.StatusTemporaryRedirect)
	return nil
}

// MAIN
func main() {
	http.Handle("/", appHandler(_index))
	http.Handle("/google", appHandler(_google))
	http.Handle("/oauth2callback", appHandler(_oauthCallback))

	log.Printf("Starting server on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}

// session
type authSession struct {
	State string
	Code  string
}

func getFlash(s *sessions.Session, field string) (value string, found bool) {
	flashes := s.Flashes(field)

	if len(flashes) < 1 {
		return "", false
	}

	if len(flashes) > 1 {
		log.Printf("Lots of codes available (%d); using the first\n", len(flashes))
	}

	return flashes[0].(string), true
}

// appHandler
type appHandler func(http.ResponseWriter, *http.Request) *appError

func (fn appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := fn(w, r); err != nil {
		// e is *appError, not os.Error.
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
	return fmt.Sprintf("Error(%d): %s", err.Status, err.Message)
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

var errCodeNotFound = fmt.Errorf("Code not found!")

func errTokenExchangeError(err error) error {
	return fmt.Errorf("Error on token exchange: %v", err)
}

func errCantLoadBooksClient(err error) error {
	return fmt.Errorf("Error while loading Google Books client: %v", err)
}

func errCantLoadVolumes(err error) error {
	return fmt.Errorf("Error while loading the user's volumes: %v", err)
}

func errCantMarshalBooksToJSON(err error) error {
	return fmt.Errorf("Error while marshalling book info to JSON: %v", err)
}

func errCantWriteResponse(err error) error {
	return fmt.Errorf("Error while writing response: %v", err)
}
