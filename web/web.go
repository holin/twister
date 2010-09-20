// Copyright 2010 Gary Burd
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package web

import (
	"bufio"
	"container/vector"
	"fmt"
	"http"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

var (
	// Object not in valid state for call.
	ErrInvalidState = os.NewError("invalid state")
	ErrBadFormat    = os.NewError("bad format")
)

// StringsMap maps strings to slices of strings.
type StringsMap map[string][]string

// NewStringsMap returns a map initialized with the given key-value pairs.
func NewStringsMap(kvs ...string) StringsMap {
	if len(kvs)%2 == 1 {
		panic("twister: even number args required for NewStringsMap")
	}
	m := make(StringsMap)
	for i := 0; i < len(kvs); i += 2 {
		m.Append(kvs[i], kvs[i+1])
	}
	return m
}

// Get returns first value for given key.
func (m StringsMap) Get(key string) (value string, found bool) {
	values, found := m[key]
	if found && len(values) > 0 {
		return value, true
	}
	return "", false
}

// GetDef returns first value for given key, or def if key is not found.
func (m StringsMap) GetDef(key string, def string) (value string) {
	values, found := m[key]
	if found && len(values) > 0 {
		return value
	}
	return def
}

// Append value to slice for given key.
func (m StringsMap) Append(key string, value string) {
	v := vector.StringVector(m[key])
	v.Push(value)
	m[key] = v
}

// Set value for given key, discarding previous values if any.
func (m StringsMap) Set(key string, value string) {
	m[key] = []string{value}
}

type RequestBody interface {
	io.Reader
}

type ResponseBody interface {
	io.Writer
	// Flush writes any buffered data to the network.
	Flush() os.Error
}

// Connection represents the server side of an HTTP connection.
type Connection interface {
	// Respond commits the status and headers to the network and returns
	// a writer for the response body.
	Respond(status int, header StringsMap) ResponseBody

	// Hijack lets the caller take over the connection from the HTTP server.
	// The caller is responsible for closing the connection.
	Hijack() (rwc io.ReadWriteCloser, buf *bufio.ReadWriter, err os.Error)
}

// Request represents an HTTP request.
type Request struct {
	Connection      Connection        // The connection.
	Method          string            // Uppercase request method. GET, POST, etc.
	URL             *http.URL         // Parsed URL.
	ProtocolVersion int               // Major version * 1000 + minor version
	Param           StringsMap        // Form, QS and other.
	Cookie          map[string]string // Cookies.
	Host            string            // Requested host.
	ContentType     string            // Content type

	// ErrorHandler responds to the request with the given status code.
	// Applications set their error handler in middleware. 
	ErrorHandler func(req *Request, status int, message string)

	// Header maps canonical header names to slices of header values.
	Header StringsMap

	// ContentLength is the length of the request body or -1 if the content
	// length is not known.
	ContentLength int

	Body RequestBody
}

// Handler is the interface for web handlers.
type Handler interface {
	ServeWeb(req *Request)
}

// HandlerFunc is a type adapter to allow the use of ordinary functions as web
// handlers.
type HandlerFunc func(*Request)

// ServeWeb calls f(req).
func (f HandlerFunc) ServeWeb(req *Request) { f(req) }

// NewRequest allocates and initializes an empty request.
func NewRequest(method string, url string, protocolVersion int, header StringsMap) (req *Request, err os.Error) {
	req = &Request{
		Method:          strings.ToUpper(method),
		ProtocolVersion: protocolVersion,
		ErrorHandler:    defaultErrorHandler,
		Param:           make(StringsMap),
		Cookie:          make(map[string]string),
		Header:          header,
	}

	req.URL, err = http.ParseURL(url)
	if err != nil {
		return
	}

	// The url overrides header per section 5.2 of rfc 2616
	req.Host = req.URL.Host
	if req.Host == "" {
		req.Host = req.Header.GetDef(HeaderHost, "")
	}

	if s, found := req.Header.Get(HeaderContentLength); found {
		var err os.Error
		req.ContentLength, err = strconv.Atoi(s)
		if err != nil {
			err = os.ErrorString("bad content length")
			return
		}
	} else {
		req.ContentLength = -1
	}

	if s, found := req.Header.Get(HeaderContentType); found {
		i := 0
		for i < len(s) && (IsTokenByte(s[i]) || s[i] == '/') {
			i++
		}
		req.ContentType = s[0:i]
	}

	return
}

// Respond is a convenience function that adds (key, value) pairs in kvs to a
// StringsMap and calls through to the connection's Respond method.
func (req *Request) Respond(status int, kvs ...string) ResponseBody {
	if len(kvs)%2 == 1 {
		panic("twister: respond requires even number of kvs args")
	}
	header := StringsMap(make(map[string][]string))
	for i := 0; i < len(kvs); i += 2 {
		header.Append(kvs[i], kvs[i+1])
	}
	return req.Connection.Respond(status, header)
}

func defaultErrorHandler(req *Request, status int, message string) {
	w := req.Respond(status, HeaderContentType, "text/plain; charset=utf-8")
	if w != nil {
		fmt.Fprintln(w, message)
	}
}

// Error responds to the request with an error. 
func (req *Request) Error(status int, message string) {
	req.ErrorHandler(req, status, message)
}

// Redirect responds to the request with a redirect the specified URL.
func (req *Request) Redirect(url string, perm bool) {
	status := StatusFound
	if perm {
		status = StatusMovedPermanently
	}

	// Make relative path absolute
	u, err := http.ParseURL(url)
	if err != nil && u.Scheme == "" && url[0] != '/' {
		d, _ := path.Split(req.URL.Path)
		url = d + url
	}

	req.Respond(status, HeaderLocation, url)
}

// CheckRequestBodyLength returns true and responds to the request with an
// error if the content length is greater than the specified value.
func (req *Request) CheckRequestBodyLength(max int) (fail bool) {
	// TODO implement me
	return true
}

// CheckXSRF returns true and responds to the request with an error if the
// action token in params does not match the action token in the cookie.
func (req *Request) CheckXSRF(tokenName string) (fail bool) {
	// TODO implement me
	return true
}

type redirectHandler struct {
	url       string
	permanent bool
}

func (rh *redirectHandler) ServeWeb(req *Request) {
	req.Redirect(rh.url, rh.permanent)
}

// RedirectHandler returns a request handler that redirects to the given URL. 
func RedirectHandler(url string, permanent bool) Handler {
	return &redirectHandler{url, permanent}
}

var notFoundHandler = HandlerFunc(func(req *Request) { req.Error(StatusNotFound, "Not Found") })

// NotFoundHandler returns a request handler that responds with 404 not found.
func NotFoundHandler() Handler {
	return notFoundHandler
}