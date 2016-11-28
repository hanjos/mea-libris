[![GoDoc](https://godoc.org/github.com/hanjos/mea-libris?status.svg)](https://godoc.org/github.com/hanjos/mea-libris)
[![GoReportCard](https://goreportcard.com/badge/github.com/hanjos/mea-libris)](https://goreportcard.com/badge/github.com/hanjos/mea-libris)

Shows your Google Books list as JSON or CSV.

# FAQ
## How do I run this?
This program needs OAuth credentials to access your book data, which can be created at [Google API Console](https://console.developers.google.com/). The whole process for server-side web apps is explained [here](https://developers.google.com/identity/protocols/OAuth2WebServer). 

In particular, you'll need an authorized redirect URL which will receive Google's auth responses. This program offers the `/oauth2callback` endpoint for that, so add the full URL: `https://<my-running-server>/oauth2callback`.
 
The application reads four environment variables: `CLIENT_ID`, `CLIENT_SECRET`, `REDIRECT_URL` and `PORT`. `PORT` is merely the port this application will listen to; `CLIENT_ID` and `CLIENT_SECRET` are your application's Google OAuth credentials; and `REDIRECT_URL` is the redirect URL you registered at the API Console.

After all that setup, just compile and run: It uses [Go](https://golang.org/) (version 1.6+) and [Glide](http://glide.sh/) for dependency management:

```
$ glide install -v
$ go install 
$ $GOPATH/bin/mea-libris
```

### But I don't like Glide; I prefer Godep/GB/gpm/<[take your pick](https://github.com/golang/go/wiki/PackageManagementTools)>!
I've found Glide to be nice, but I have no strong opinion or experience either way. The main requirement was something supported by Cloud Foundry (which I'm also checking out), so Glide worked well enough.

## Google Books
### Why Google Books and not Amazon?
Seemed like the easiest to tackle. Amazon books should come, eventually :)

### The books I've previously rented aren't appearing!
Well, Google's Books API describes previously rented books as "User-rented books past their expiration time". So, if the books are expired, they're not "your" books any more ;)
