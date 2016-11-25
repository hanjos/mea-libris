[![GoDoc](https://godoc.org/github.com/hanjos/mea-libris?status.svg)](https://godoc.org/github.com/hanjos/mea-libris)
[![GoReportCard](https://goreportcard.com/badge/github.com/hanjos/mea-libris)](https://goreportcard.com/badge/github.com/hanjos/mea-libris)

Shows your Google Books list as JSON or CSV.

# FAQ
## How do I use this?
Run it: it uses [Go](https://golang.org/) (version 1.6+) and [Glide](http://glide.sh/).

## Google Books
### Why Google Books and not Amazon?
Seemed like the easiest to tackle. Amazon books should come, eventually :)

### The books I've previously rented aren't appearing!
Well, Google's Books API describes previously rented books as "User-rented books past their expiration time". So, if the books are expired, they're not 'your' books any more, are they :)
