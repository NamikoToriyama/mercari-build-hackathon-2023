package handler_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestPostPurchase(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		itemID              int64
		buyerUserID         int64
		injectorForUserRepo func(*db.MockUserRepository)
		injectorForItemRepo func(*db.MockItemRepository)
		wantStatusCode      int
	}{
		"200: correctly purchase": {
			itemID:      1,
			buyerUserID: 1,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(1)).Return(domain.User{
					ID:      1,
					Balance: 10,
				}, nil).Times(1)
				m.EXPECT().GetUser(gomock.Any(), int64(2)).Return(domain.User{
					ID:      2,
					Balance: 10,
				}, nil).Times(1)
				m.EXPECT().UpdateBalance(gomock.Any(), int64(1), int64(0)).Return(nil).Times(1)
				m.EXPECT().UpdateBalance(gomock.Any(), int64(2), int64(20)).Return(nil).Times(1)
			},
			injectorForItemRepo: func(m *db.MockItemRepository) {
				m.EXPECT().GetItem(gomock.Any(), int64(1)).Return(domain.Item{
					ID:     1,
					Price:  10,
					UserID: 2,
					Status: domain.ItemStatusOnSale,
				}, nil).Times(1)
				m.EXPECT().UpdateItemStatus(gomock.Any(), int64(1), domain.ItemStatusSoldOut).Return(nil).Times(1)
			},
			wantStatusCode: http.StatusOK,
		},
		"401: failed because of an invalid user id": {
			buyerUserID:         -1,
			injectorForUserRepo: func(_ *db.MockUserRepository) {},
			injectorForItemRepo: func(_ *db.MockItemRepository) {},
			wantStatusCode:      http.StatusUnauthorized,
		},
		"412: failed because item status is sold out": {
			itemID:      1,
			buyerUserID: 1,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(1)).Return(domain.User{
					ID:      1,
					Balance: 10,
				}, nil).Times(1)
			},
			injectorForItemRepo: func(m *db.MockItemRepository) {
				m.EXPECT().GetItem(gomock.Any(), int64(1)).Return(domain.Item{
					ID:     1,
					Status: domain.ItemStatusSoldOut,
				}, nil).Times(1)
			},
			wantStatusCode: http.StatusPreconditionFailed,
		},
		"412: failed because item is not found": {
			itemID:      2,
			buyerUserID: 1,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(1)).Return(domain.User{
					ID:      1,
					Balance: 10,
				}, nil).Times(1)
			},
			injectorForItemRepo: func(m *db.MockItemRepository) {
				m.EXPECT().GetItem(gomock.Any(), int64(2)).Return(domain.Item{}, sql.ErrNoRows).Times(1)
			},
			wantStatusCode: http.StatusPreconditionFailed,
		},
		"412: failed because a given user is not found": {
			buyerUserID: 2,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(2)).Return(domain.User{}, sql.ErrNoRows).Times(1)
			},
			injectorForItemRepo: func(_ *db.MockItemRepository) {},
			wantStatusCode:      http.StatusPreconditionFailed,
		},
		"412: failed because of buying given user owned item": {
			itemID:      1,
			buyerUserID: 1,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(1)).Return(domain.User{
					ID:      1,
					Balance: 10,
				}, nil).Times(1)
			},
			injectorForItemRepo: func(m *db.MockItemRepository) {
				m.EXPECT().GetItem(gomock.Any(), int64(1)).Return(domain.Item{
					ID:     1,
					UserID: 1,
					Status: domain.ItemStatusOnSale,
				}, nil).Times(1)
			},
			wantStatusCode: http.StatusPreconditionFailed,
		},
		"412: failed because of a lack of balance": {
			itemID:      1,
			buyerUserID: 1,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(1)).Return(domain.User{
					ID:      1,
					Balance: 10,
				}, nil).Times(1)
				m.EXPECT().GetUser(gomock.Any(), int64(2)).Return(domain.User{
					ID:      2,
					Balance: 10,
				}, nil).Times(1)
			},
			injectorForItemRepo: func(m *db.MockItemRepository) {
				m.EXPECT().GetItem(gomock.Any(), int64(1)).Return(domain.Item{
					ID:     1,
					UserID: 2,
					Price:  9999,
					Status: domain.ItemStatusOnSale,
				}, nil).Times(1)
			},
			wantStatusCode: http.StatusPreconditionFailed,
		},
		"412: failed because a seller user is not found": {
			itemID:      1,
			buyerUserID: 1,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(1)).Return(domain.User{
					ID:      1,
					Balance: 10,
				}, nil).Times(1)
				m.EXPECT().GetUser(gomock.Any(), int64(3)).Return(domain.User{}, sql.ErrNoRows)
			},
			injectorForItemRepo: func(m *db.MockItemRepository) {
				m.EXPECT().GetItem(gomock.Any(), int64(1)).Return(domain.Item{
					ID:     1,
					UserID: 3,
					Status: domain.ItemStatusOnSale,
				}, nil).Times(1)
			},
			wantStatusCode: http.StatusPreconditionFailed,
		},
		"500: internal server error": {
			buyerUserID: 9999,
			injectorForUserRepo: func(m *db.MockUserRepository) {
				m.EXPECT().GetUser(gomock.Any(), int64(9999)).Return(domain.User{}, errors.New("strange error")).Times(1)
			},
			injectorForItemRepo: func(_ *db.MockItemRepository) {},
			wantStatusCode:      http.StatusInternalServerError,
		},
	}

	for name, tt := range cases {
		tt := tt

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// ref: https://echo.labstack.com/guide/testing/
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/purchase/:itemID", nil)
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set("user", &jwt.Token{Claims: &handler.JwtCustomClaims{UserID: tt.buyerUserID}})
			c.SetParamNames("itemID")
			c.SetParamValues(strconv.Itoa(int(tt.itemID)))

			// ready gomock
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			userRepo := db.NewMockUserRepository(ctrl)
			tt.injectorForUserRepo(userRepo)
			itemRepo := db.NewMockItemRepository(ctrl)
			tt.injectorForItemRepo(itemRepo)

			// test handler
			h := handler.Handler{UserRepo: userRepo, ItemRepo: itemRepo}
			// TODO: might be better... :(
			if err := h.Purchase(c); err != nil {
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
