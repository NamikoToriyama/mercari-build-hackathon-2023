package handler_test

import (
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
		"400: failed to go user id": {
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
