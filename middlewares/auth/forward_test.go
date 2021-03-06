package auth

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/containous/traefik/middlewares/tracing"
	"github.com/containous/traefik/testhelpers"
	"github.com/containous/traefik/types"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/negroni"
)

func TestForwardAuthFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Forbidden", http.StatusForbidden)
	}))
	defer server.Close()

	middleware, err := NewAuthenticator(&types.Auth{
		Forward: &types.Forward{
			Address: server.URL,
		},
	}, &tracing.Tracing{})
	assert.NoError(t, err, "there should be no error")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "traefik")
	})
	n := negroni.New(middleware)
	n.UseHandler(handler)
	ts := httptest.NewServer(n)
	defer ts.Close()

	client := &http.Client{}
	req := testhelpers.MustNewRequest(http.MethodGet, ts.URL, nil)
	res, err := client.Do(req)
	assert.NoError(t, err, "there should be no error")
	assert.Equal(t, http.StatusForbidden, res.StatusCode, "they should be equal")

	body, err := ioutil.ReadAll(res.Body)
	assert.NoError(t, err, "there should be no error")
	assert.Equal(t, "Forbidden\n", string(body), "they should be equal")
}

func TestForwardAuthSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Success")
	}))
	defer server.Close()

	middleware, err := NewAuthenticator(&types.Auth{
		Forward: &types.Forward{
			Address: server.URL,
		},
	}, &tracing.Tracing{})
	assert.NoError(t, err, "there should be no error")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "traefik")
	})
	n := negroni.New(middleware)
	n.UseHandler(handler)
	ts := httptest.NewServer(n)
	defer ts.Close()

	client := &http.Client{}
	req := testhelpers.MustNewRequest(http.MethodGet, ts.URL, nil)
	res, err := client.Do(req)
	assert.NoError(t, err, "there should be no error")
	assert.Equal(t, http.StatusOK, res.StatusCode, "they should be equal")

	body, err := ioutil.ReadAll(res.Body)
	assert.NoError(t, err, "there should be no error")
	assert.Equal(t, "traefik\n", string(body), "they should be equal")
}

func TestForwardAuthRedirect(t *testing.T) {
	authTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/redirect-test", http.StatusFound)
	}))
	defer authTs.Close()

	authMiddleware, err := NewAuthenticator(&types.Auth{
		Forward: &types.Forward{
			Address: authTs.URL,
		},
	}, &tracing.Tracing{})
	assert.NoError(t, err, "there should be no error")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "traefik")
	})
	n := negroni.New(authMiddleware)
	n.UseHandler(handler)
	ts := httptest.NewServer(n)
	defer ts.Close()

	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req := testhelpers.MustNewRequest(http.MethodGet, ts.URL, nil)
	res, err := client.Do(req)
	assert.NoError(t, err, "there should be no error")
	assert.Equal(t, http.StatusFound, res.StatusCode, "they should be equal")

	location, err := res.Location()

	assert.NoError(t, err, "there should be no error")
	assert.Equal(t, "http://example.com/redirect-test", location.String(), "they should be equal")

	body, err := ioutil.ReadAll(res.Body)
	assert.NoError(t, err, "there should be no error")
	assert.NotEmpty(t, string(body), "there should be something in the body")
}

func TestForwardAuthCookie(t *testing.T) {
	authTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie := &http.Cookie{Name: "example", Value: "testing", Path: "/"}
		http.SetCookie(w, cookie)
		http.Error(w, "Forbidden", http.StatusForbidden)
	}))
	defer authTs.Close()

	authMiddleware, err := NewAuthenticator(&types.Auth{
		Forward: &types.Forward{
			Address: authTs.URL,
		},
	}, &tracing.Tracing{})
	assert.NoError(t, err, "there should be no error")

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "traefik")
	})
	n := negroni.New(authMiddleware)
	n.UseHandler(handler)
	ts := httptest.NewServer(n)
	defer ts.Close()

	client := &http.Client{}
	req := testhelpers.MustNewRequest(http.MethodGet, ts.URL, nil)
	res, err := client.Do(req)
	assert.NoError(t, err, "there should be no error")
	assert.Equal(t, http.StatusForbidden, res.StatusCode, "they should be equal")

	for _, cookie := range res.Cookies() {
		assert.Equal(t, "testing", cookie.Value, "they should be equal")
	}

	body, err := ioutil.ReadAll(res.Body)
	assert.NoError(t, err, "there should be no error")
	assert.Equal(t, "Forbidden\n", string(body), "they should be equal")
}

func Test_writeHeader(t *testing.T) {

	testCases := []struct {
		name               string
		headers            map[string]string
		trustForwardHeader bool
		emptyHost          bool
		expectedHeaders    map[string]string
	}{
		{
			name: "trust Forward Header",
			headers: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "fii.bir",
			},
			trustForwardHeader: true,
			expectedHeaders: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "fii.bir",
			},
		},
		{
			name: "not trust Forward Header",
			headers: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "fii.bir",
			},
			trustForwardHeader: false,
			expectedHeaders: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "foo.bar",
			},
		},
		{
			name: "trust Forward Header with empty Host",
			headers: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "fii.bir",
			},
			trustForwardHeader: true,
			emptyHost:          true,
			expectedHeaders: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "fii.bir",
			},
		},
		{
			name: "not trust Forward Header with empty Host",
			headers: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "fii.bir",
			},
			trustForwardHeader: false,
			emptyHost:          true,
			expectedHeaders: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "",
			},
		},
		{
			name: "trust Forward Header with forwarded URI",
			headers: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "fii.bir",
				"X-Forwarded-Uri":  "/forward?q=1",
			},
			trustForwardHeader: true,
			expectedHeaders: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "fii.bir",
				"X-Forwarded-Uri":  "/forward?q=1",
			},
		},
		{
			name: "not trust Forward Header with forward requested URI",
			headers: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "fii.bir",
				"X-Forwarded-Uri":  "/forward?q=1",
			},
			trustForwardHeader: false,
			expectedHeaders: map[string]string{
				"Accept":           "application/json",
				"X-Forwarded-Host": "foo.bar",
				"X-Forwarded-Uri":  "/path?q=1",
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {

			req := testhelpers.MustNewRequest(http.MethodGet, "http://foo.bar/path?q=1", nil)
			for key, value := range test.headers {
				req.Header.Set(key, value)
			}

			if test.emptyHost {
				req.Host = ""
			}

			forwardReq := testhelpers.MustNewRequest(http.MethodGet, "http://foo.bar/path?q=1", nil)

			writeHeader(req, forwardReq, test.trustForwardHeader)

			for key, value := range test.expectedHeaders {
				assert.Equal(t, value, forwardReq.Header.Get(key))
			}
		})
	}
}
