package authboss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthBossInit(t *testing.T) {
	t.Parallel()

	ab := New()
	err := ab.Init()
	if err != nil {
		t.Error("Unexpected error:", err)
	}
}

func TestAuthbossUpdatePassword(t *testing.T) {
	t.Parallel()

	user := &mockUser{}
	storer := newMockServerStorer()

	ab := New()
	ab.Config.Storage.Server = storer

	if err := ab.UpdatePassword(context.Background(), user, "hello world"); err != nil {
		t.Error(err)
	}

	if len(user.Password) == 0 {
		t.Error("password was not updated")
	}
}

type testRedirector struct {
	Opts RedirectOptions
}

func (r *testRedirector) Redirect(w http.ResponseWriter, req *http.Request, ro RedirectOptions) error {
	r.Opts = ro
	if len(ro.RedirectPath) == 0 {
		panic("no redirect path on redirect call")
	}
	http.Redirect(w, req, ro.RedirectPath, ro.Code)
	return nil
}

func TestAuthbossMiddleware(t *testing.T) {
	t.Parallel()

	ab := New()
	ab.Core.Logger = mockLogger{}
	ab.Storage.Server = &mockServerStorer{
		Users: map[string]*mockUser{
			"test@test.com": &mockUser{},
		},
	}

	setupMore := func(redirect bool, allowHalfAuth bool) (*httptest.ResponseRecorder, bool, bool) {
		r := httptest.NewRequest("GET", "/super/secret", nil)
		rec := httptest.NewRecorder()
		w := ab.NewResponse(rec)

		var err error
		r, err = ab.LoadClientState(w, r)
		if err != nil {
			t.Fatal(err)
		}

		mid := Middleware(ab, redirect, allowHalfAuth)
		var called, hadUser bool
		server := mid(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			hadUser = r.Context().Value(CTXKeyUser) != nil
			w.WriteHeader(http.StatusOK)
		}))

		server.ServeHTTP(w, r)

		return rec, called, hadUser
	}

	t.Run("Accept", func(t *testing.T) {
		ab.Storage.SessionState = mockClientStateReadWriter{
			state: mockClientState{SessionKey: "test@test.com"},
		}

		_, called, hadUser := setupMore(false, false)

		if !called {
			t.Error("should have been called")
		}
		if !hadUser {
			t.Error("should have had user")
		}
	})
	t.Run("AcceptHalfAuth", func(t *testing.T) {
		ab.Storage.SessionState = mockClientStateReadWriter{
			state: mockClientState{SessionKey: "test@test.com", SessionHalfAuthKey: "true"},
		}

		_, called, hadUser := setupMore(false, true)

		if !called {
			t.Error("should have been called")
		}
		if !hadUser {
			t.Error("should have had user")
		}
	})
	t.Run("Reject404", func(t *testing.T) {
		ab.Storage.SessionState = mockClientStateReadWriter{}

		rec, called, hadUser := setupMore(false, false)

		if rec.Code != http.StatusNotFound {
			t.Error("wrong code:", rec.Code)
		}
		if called {
			t.Error("should not have been called")
		}
		if hadUser {
			t.Error("should not have had user")
		}
	})
	t.Run("RejectRedirect", func(t *testing.T) {
		redir := &testRedirector{}
		ab.Config.Core.Redirector = redir

		ab.Storage.SessionState = mockClientStateReadWriter{}

		_, called, hadUser := setupMore(true, false)

		if redir.Opts.Code != http.StatusTemporaryRedirect {
			t.Error("code was wrong:", redir.Opts.Code)
		}
		if redir.Opts.RedirectPath != "/auth/login?redir=%2Fsuper%2Fsecret" {
			t.Error("redirect path was wrong:", redir.Opts.RedirectPath)
		}
		if called {
			t.Error("should not have been called")
		}
		if hadUser {
			t.Error("should not have had user")
		}
	})
	t.Run("RejectHalfAuth", func(t *testing.T) {
		ab.Storage.SessionState = mockClientStateReadWriter{
			state: mockClientState{SessionKey: "test@test.com", SessionHalfAuthKey: "true"},
		}

		rec, called, hadUser := setupMore(false, false)

		if rec.Code != http.StatusNotFound {
			t.Error("wrong code:", rec.Code)
		}
		if called {
			t.Error("should not have been called")
		}
		if hadUser {
			t.Error("should not have had user")
		}
	})

	/*
		var err error
		r, err = ab.LoadClientState(w, r)
		if err != nil {
			t.Fatal(err)
		}
		server.ServeHTTP(w, r)
		if called || hadUser {
			t.Error("should not be called or have a user when no session variables have been provided")
		}
		if rec.Code != http.StatusNotFound {
			t.Error("want a not found code")
		}

		ab.Storage.SessionState = mockClientStateReadWriter{
			state: mockClientState{SessionKey: "test@test.com"},
		}
		ab.Storage.Server = &mockServerStorer{
			Users: map[string]*mockUser{
				"test@test.com": &mockUser{},
			},
		}

		r = httptest.NewRequest("GET", "/", nil)
		rec = httptest.NewRecorder()
		w = ab.NewResponse(rec)

		r, err = ab.LoadClientState(w, r)
		if err != nil {
			t.Fatal(err)
		}
		server.ServeHTTP(w, r)
		if !called {
			t.Error("it should have been called")
		}
		if !hadUser {
			t.Error("it should have had a user loaded")
		}
		if rec.Code != http.StatusOK {
			t.Error("want a not found code")
		}
	*/
}
