[![GoDoc](https://godoc.org/github.com/hanjos/mea-libris?status.svg)](https://godoc.org/github.com/hanjos/mea-libris)
[![GoReportCard](https://goreportcard.com/badge/github.com/hanjos/mea-libris)](https://goreportcard.com/report/github.com/hanjos/mea-libris)

Shows your Google Books list as JSON or CSV.

# FAQ
## How do I run this?
This command needs OAuth credentials to access your book data, which can be created at [Google API Console](https://console.developers.google.com/). The whole process for server-side web apps is explained [here](https://developers.google.com/identity/protocols/OAuth2WebServer). 

In particular, you'll need to register an authorized redirect URL, which will receive Google's auth responses. This program offers the `/google/oauth2callback` endpoint for that, so use the full URL: `http://<my-running-server>/google/oauth2callback`.
 
Your instance's Google OAuth credentials are read via two environment variables, `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET`.

Optionally, this command also reads two more environment variables:

* `PORT`: the port the server will be bound to. Defaults to 8080.
* `GOOGLE_REDIRECT_URL`: forces a specific redirect URL. More on this below.

### OK, it's running. Now what?

A running instance provides the following endpoints:

#### `GET /google` 

Returns your books in either JSON or CSV, depending on the request's `Accept` header. Will return 401 if the user hasn't previously allowed this instance to access her data.

#### `GET /google/connect`
Starts the auth exchange. As per OAuth, the user will be redirected to a Google consent screen to authorize this instance to get the data, and then redirected back. Will error out if this instance wasn't previously authorized in the user's Google API Console.

#### `GET /google/disconnect`
Revokes the user's authorization. Any further accesses to `/google` will be 401'ed until the user `/google/connect`s again.

#### `GET /google/oauth2callback`
This is called by Google's OAuth servers to answer `/google/connect` requests. As mentioned above, the `/google/oauth2callback` endpoint should be registered in the Google API Console as an authorized redirect URL.

### Google doesn't accept the redirect URL!

Yeah... `mea-libris` can build the redirect URL itself, but: 

* Google demands the URL to be an exact match (scheme, host, port and path) with what's registered in the API Console; and 
* I don't know a sure-fire way to get all the information needed (*e.g.* the scheme isn't always there, so `mea-libris` defaults to `http`). 

So, as an escape hatch, `mea-libris` will use the contents of the `GOOGLE_REDIRECT_URL` environment variable, if available.

## How do I build this?

mea-libris uses [Go](https://golang.org/) (version 1.6+) and [Glide](http://glide.sh/).

After the setup, compile and run:

```
$ glide update -v
$ go install 
$ $GOPATH/bin/mea-libris
```

Alternatively, you could just deploy it in the cloud, without building it at all (if your cloud provider supports Go and Glide):

```
$ cf push mea-libris -b go_buildpack
```

I've used Cloud Foundry here, but pick your favorite :)

### But I don't like Glide; I prefer <[take your pick](https://github.com/golang/go/wiki/PackageManagementTools)>!
I've found Glide to be nice, but I have no strong opinion or experience either way. The main requirement was something supported by Cloud Foundry (which I'm also checking out), so Glide worked well enough.

## Why Google Books and not Amazon?
Seemed like the easiest to tackle. ~~Amazon books should come, eventually :)~~ 

Upon further reading, Amazon apparently [doesn't have](http://stackoverflow.com/questions/7191429/get-kindle-library-book-list) a public API or [any interest](http://www.programmableweb.com/news/why-amazon-needs-kindle-api-and-will-never-have-one/2012/10/11) in making one. So... I'd need some (ahem) alternative means to get that information automatically :stuck_out_tongue_winking_eye: Any suggestions?

## The books I've previously rented aren't appearing!
Well, Google's Books API describes previously rented books as "User-rented books past their expiration time". So, if the books are expired, they're not "your" books any more ;)
