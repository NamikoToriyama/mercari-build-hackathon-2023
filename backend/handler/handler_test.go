package handler_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NamikoToriyama/mecari-build-hackathon-2023/backend/db"
	"github.com/NamikoToriyama/mecari-build-hackathon-2023/backend/domain"
	"github.com/NamikoToriyama/mecari-build-hackathon-2023/backend/handler"
	"github.com/golang-jwt/jwt/v5"
	"github.com/golang/mock/gomock"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
)

func TestGetBalance(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		userID              int64
		injectorForUserRepo func(*db.MockUserRepository)
		wantStatusCode      int
		wantBalance         int64
	}{
		"200: correctly got balance": {
			userID: 1,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(1)).Return(domain.User{
					ID:      1,
					Balance: 57,
				}, nil).Times(1)
			},
			wantStatusCode: http.StatusOK,
			wantBalance:    57,
		},
		"401: failed because of an invalid user id": {
			userID:              -1,
			injectorForUserRepo: func(_ *db.MockUserRepository) {},
			wantStatusCode:      http.StatusUnauthorized,
		},
		"412: user not found": {
			userID: 2,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(2)).Return(domain.User{}, sql.ErrNoRows).Times(1)
			},
			wantStatusCode: http.StatusPreconditionFailed,
		},
		"500: internal server error": {
			userID: 9999,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(9999)).Return(domain.User{}, errors.New("strange error")).Times(1)
			},
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for name, tt := range cases {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// ref: https://echo.labstack.com/guide/testing/
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, "/balance", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("user", &jwt.Token{Claims: &handler.JwtCustomClaims{UserID: tt.userID}})

			// ready gomock
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			userRepo := db.NewMockUserRepository(ctrl)
			tt.injectorForUserRepo(userRepo)

			// test handler
			h := handler.Handler{UserRepo: userRepo}
			// TODO: might be better... :(
			if err := h.GetBalance(c); err != nil {
				echoErr, ok := err.(*echo.HTTPError)
				if !ok {
					t.Fatalf("unexpected error: %s", err.Error())
				}
				if tt.wantStatusCode != echoErr.Code {
					t.Fatalf("unexpected status code: want: %d, got: %d", tt.wantStatusCode, rec.Code)
				}
				if echoErr.Code != http.StatusOK {
					return
				}
			}
			resp := handler.GetBalanceResponse{}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("unexpected error for json.Unamrshal: %s", err.Error())
			}
			if tt.wantBalance != resp.Balance {
				t.Fatalf("unexpected balance: want: %d, got: %d", tt.wantBalance, resp.Balance)
			}
		})
	}
}

func TestPostBalance(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		reqBalance          int64
		userID              int64
		injectorForUserRepo func(*db.MockUserRepository)
		wantStatusCode      int
	}{
		"200: correctly add balance": {
			reqBalance: 10,
			userID:     1,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(1)).Return(domain.User{
					ID:      1,
					Balance: 57,
				}, nil).Times(1)
				// updating is DB logic, so the check after updating is unneeded
				m.EXPECT().UpdateBalance(gomock.Any(), int64(1), int64(67)).Return(nil)
			},
			wantStatusCode: http.StatusOK,
		},
		"400: failed because of negative balance": {
			reqBalance:          -1,
			userID:              2,
			injectorForUserRepo: func(_ *db.MockUserRepository) {},
			wantStatusCode:      http.StatusBadRequest,
		},
		"401: failed because of an invalid user id": {
			reqBalance:          1,
			userID:              -1,
			injectorForUserRepo: func(_ *db.MockUserRepository) {},
			wantStatusCode:      http.StatusUnauthorized,
		},
		"412: failed because of given user not found": {
			reqBalance: 1,
			userID:     3,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(3)).Return(domain.User{}, sql.ErrNoRows).Times(1)
			},
			wantStatusCode: http.StatusPreconditionFailed,
		},
		"500: internal server error": {
			reqBalance: 1,
			userID:     9999,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(9999)).Return(domain.User{}, errors.New("strange error")).Times(1)
			},
			wantStatusCode: http.StatusInternalServerError,
		},
	}

	for name, tt := range cases {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// ref: https://echo.labstack.com/guide/testing/
			e := echo.New()
			d, err := json.Marshal(handler.AddBalanceRequest{
				Balance: tt.reqBalance,
			})
			if err != nil {
				t.Fatalf("failed json.Marshal: %s", err.Error())
			}
			req := httptest.NewRequest(http.MethodPost, "/balance", bytes.NewBuffer(d))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("user", &jwt.Token{Claims: &handler.JwtCustomClaims{UserID: tt.userID}})

			// ready gomock
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			userRepo := db.NewMockUserRepository(ctrl)
			tt.injectorForUserRepo(userRepo)

			// test handler
			h := handler.Handler{UserRepo: userRepo}
			// TODO: might be better... :(
			if err := h.AddBalance(c); err != nil {
				t.Logf("err: %s", err.Error())
				echoErr, ok := err.(*echo.HTTPError)
				if !ok {
					t.Fatalf("unexpected error: %s", err.Error())
				}
				if tt.wantStatusCode != echoErr.Code {
					t.Fatalf("unexpected status code: want: %d, got: %d", tt.wantStatusCode, rec.Code)
				}
				if echoErr.Code != http.StatusOK {
					return
				}
			}
		})
	}
}
