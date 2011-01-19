// Copyright 2011 Gary Burd
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
	"testing"
	"sort"
)

type routeTestHandler string

func (h routeTestHandler) ServeWeb(req *Request) {
	w := req.Respond(200)
	var keys []string
	for key, _ := range req.Param {
		keys = append(keys, key)
	}
	sort.SortStrings(keys)
	w.Write([]byte(string(h)))
	for _, key := range keys {
		w.Write([]byte(" "))
		w.Write([]byte(key))
		w.Write([]byte(":"))
		w.Write([]byte(req.Param.GetDef(key, "<nil>")))
	}
}

var routeTests = []struct {
	url    string
	method string
	status int
	body   string
}{
	{url: "/Bogus/Path", method: "GET", status: 404, body: ""},
	{url: "/Bogus/Path", method: "POST", status: 404, body: ""},
	{url: "/", method: "GET", status: 200, body: "home-get"},
	{url: "/", method: "HEAD", status: 200, body: "home-get"},
	{url: "/", method: "POST", status: 405, body: ""},
	{url: "/a", method: "GET", status: 200, body: "a-get"},
	{url: "/a", method: "HEAD", status: 200, body: "a-get"},
	{url: "/a", method: "POST", status: 200, body: "a-*"},
	{url: "/a/", method: "GET", status: 404, body: ""},
	{url: "/b", method: "GET", status: 200, body: "b-get"},
	{url: "/b", method: "HEAD", status: 200, body: "b-get"},
	{url: "/b", method: "POST", status: 200, body: "b-post"},
	{url: "/b", method: "PUT", status: 405, body: ""},
	{url: "/c", method: "GET", status: 200, body: "c-*"},
	{url: "/c", method: "HEAD", status: 200, body: "c-*"},
	{url: "/d", method: "GET", status: 301, body: ""},
	{url: "/d/", method: "GET", status: 200, body: "d"},
	{url: "/e/foo", method: "GET", status: 200, body: "e x:foo"},
	{url: "/e/foo/", method: "GET", status: 404, body: ""},
	{url: "/f/foo/bar", method: "GET", status: 301, body: ""},
	{url: "/f/foo/bar/", method: "GET", status: 200, body: "f x:foo y:bar"},
}

func TestRouter(t *testing.T) {
	r := NewRouter()
	r.Register("/", "GET", routeTestHandler("home-get"))
	r.Register("/a", "GET", routeTestHandler("a-get"), "*", routeTestHandler("a-*"))
	r.Register("/b", "GET", routeTestHandler("b-get"), "POST", routeTestHandler("b-post"))
	r.Register("/c", "*", routeTestHandler("c-*"))
	r.Register("/d/", "GET", routeTestHandler("d"))
	r.Register("/e/<x>", "GET", routeTestHandler("e"))
	r.Register("/f/<x>/<y>/", "GET", routeTestHandler("f"))

	for _, rt := range routeTests {
		status, _, body := RunHandler(rt.url, rt.method, nil, nil, r)
		if status != rt.status {
			t.Errorf("url=%s method=%s\n\texpected %d\n\tactual   %d", rt.url, rt.method, rt.status, status)
		}
		if status == 200 {
			if string(body) != rt.body {
				t.Errorf("url=%s method=%s\n\texpected %s\n\tactual   %s", rt.url, rt.method, rt.body, string(body))
			}
		}
	}
}

var hostRouteTests = []struct {
	url    string
	status int
	body   string
}{
	{url: "http://www.example.com/", status: 200, body: "www.example.com"},
	{url: "http://foo.example.com/", status: 200, body: "*.example.com x:foo"},
	{url: "http://example.com/", status: 200, body: "default"},
}

func TestHostRouter(t *testing.T) {
	r := NewHostRouter(routeTestHandler("default"))
	r.Register("www.example.com", routeTestHandler("www.example.com"))
	r.Register("<x>.example.com", routeTestHandler("*.example.com"))

	for _, rt := range hostRouteTests {
		status, _, body := RunHandler(rt.url, "GET", nil, nil, r)
		if status != rt.status {
			t.Errorf("url=%sn\texpected %d\n\tactual   %d", rt.url, rt.status, status)
		}
		if status == 200 {
			if string(body) != rt.body {
				t.Errorf("url=%s\n\texpected %s\n\tactual   %s", rt.url, rt.body, string(body))
			}
		}
	}
}
