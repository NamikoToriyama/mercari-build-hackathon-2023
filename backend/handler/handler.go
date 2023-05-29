package handler

import (
	"bytes"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/NamikoToriyama/mecari-build-hackathon-2023/backend/db"
	"github.com/NamikoToriyama/mecari-build-hackathon-2023/backend/domain"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

var (
	logFile = getEnv("LOGFILE", "access.log")
)

type JwtCustomClaims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

type InitializeResponse struct {
	Message string `json:"message"`
}

type registerRequest struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type registerResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type getUserItemsResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Price        int64  `json:"price"`
	CategoryName string `json:"category_name"`
}

type getOnSaleItemsResponse struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Price        int64  `json:"price"`
	CategoryName string `json:"category_name"`
}

type getCategoriesResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type sellRequest struct {
	ItemID int64 `json:"item_id"`
}

type itemRequest struct {
	Name        string `form:"name"`
	CategoryID  int64  `form:"category_id"`
	Price       int64  `form:"price"`
	Description string `form:"description"`
}

type addItemResponse struct {
	ID int64 `json:"id"`
}

type AddBalanceRequest struct {
	Balance int64 `json:"balance"`
}

type GetBalanceResponse struct {
	Balance int64 `json:"balance"`
}

type loginRequest struct {
	UserID   int64  `json:"user_id"`
	Password string `json:"password"`
}

type loginResponse struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

type Handler struct {
	DB       *sql.DB
	UserRepo db.UserRepository
	ItemRepo db.ItemRepository
}

func GetSecret() string {
	if secret := os.Getenv("SECRET"); secret != "" {
		return secret
	}
	return "secret-key"
}

func (h *Handler) Initialize(c echo.Context) error {
	err := os.Truncate(logFile, 0)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, errors.Wrap(err, "Failed to truncate access log"))
	}

	err = db.Initialize(c.Request().Context(), h.DB)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, errors.Wrap(err, "Failed to initialize"))
	}

	return c.JSON(http.StatusOK, InitializeResponse{Message: "Success"})
}

func (h *Handler) AccessLog(c echo.Context) error {
	return c.File(logFile)
}

func (h *Handler) Register(c echo.Context) error {
	// TODO: validation
	// http.StatusBadRequest(400)
	req := new(registerRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	userID, err := h.UserRepo.AddUser(c.Request().Context(), domain.User{Name: req.Name, Password: string(hash)})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, registerResponse{ID: userID, Name: req.Name})
}

func (h *Handler) Login(c echo.Context) error {
	ctx := c.Request().Context()
	// TODO: validation
	// http.StatusBadRequest(400)
	req := new(loginRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	user, err := h.UserRepo.GetUser(ctx, req.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return echo.NewHTTPError(http.StatusUnauthorized, err)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	// Set custom claims
	claims := &JwtCustomClaims{
		req.UserID,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 72)),
		},
	}
	// Create token with claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// Generate encoded token and send it as response.
	encodedToken, err := token.SignedString([]byte(GetSecret()))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, loginResponse{
		ID:    user.ID,
		Name:  user.Name,
		Token: encodedToken,
	})
}

func (h *Handler) AddItem(c echo.Context) error {
	ctx := c.Request().Context()

	req := new(itemRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err)
	}

	_, err = h.ItemRepo.GetCategory(ctx, req.CategoryID)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid categoryID")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	imageByte, err := getImageByte(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	item, err := h.ItemRepo.AddItem(c.Request().Context(), domain.Item{
		Name:        req.Name,
		CategoryID:  req.CategoryID,
		UserID:      userID,
		Price:       req.Price,
		Description: req.Description,
		Image:       imageByte,
		Status:      domain.ItemStatusInitial,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, addItemResponse{ID: int64(item.ID)})
}

func (h *Handler) UpdateItem(c echo.Context) error {
	ctx := c.Request().Context()

	itemID, err := strconv.ParseInt(c.Param("itemID"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	req := new(itemRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("itemRequest c.Bind() error: %v", err))
	}

	_, err = getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err)
	}

	_, err = h.ItemRepo.GetCategory(ctx, req.CategoryID)
	if err != nil {
		if err == sql.ErrNoRows {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid categoryID")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("GetCategory() error: %v", err))
	}

	imageByte, err := getImageByte(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("getImageByte error: %v", err))
	}

	item, err := h.ItemRepo.UpdateItem(c.Request().Context(), domain.Item{
		ID:          itemID,
		Name:        req.Name,
		CategoryID:  req.CategoryID,
		Price:       req.Price,
		Description: req.Description,
		Image:       imageByte,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, item.ConvertToGetItemResponse())
}

func getImageByte(c echo.Context) ([]byte, error) {
	file, err := c.FormFile("image")
	if err != nil {
		log.Printf("file not found: %s", err.Error())
		return nil, err
	}

	src, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := src.Close(); err != nil {
			log.Printf("failed src.Close: %s", err.Error())
		}
	}()

	var dest []byte
	blob := bytes.NewBuffer(dest)

	if _, err := io.Copy(blob, src); err != nil {
		return nil, err
	}
	return blob.Bytes(), nil
}

func (h *Handler) Sell(c echo.Context) error {
	ctx := c.Request().Context()
	req := new(sellRequest)

	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}

	item, err := h.ItemRepo.GetItem(ctx, req.ItemID)
	// TODO: not found handling
	// http.StatusPreconditionFailed(412)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	// TODO: check req.UserID and item.UserID
	// http.StatusPreconditionFailed(412)
	// TODO: only update when status is initial
	// http.StatusPreconditionFailed(412)
	if err := h.ItemRepo.UpdateItemStatus(ctx, item.ID, domain.ItemStatusOnSale); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, "successful")
}

func (h *Handler) GetOnSaleItems(c echo.Context) error {
	ctx := c.Request().Context()

	items, err := h.ItemRepo.GetOnSaleItems(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err)
	}

	var res []getOnSaleItemsResponse
	for _, item := range items {
		cats, err := h.ItemRepo.GetCategories(ctx)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, err)
		}
		for _, cat := range cats {
			if cat.ID == item.CategoryID {
				res = append(res, getOnSaleItemsResponse{ID: item.ID, Name: item.Name, Price: item.Price, CategoryName: cat.Name})
			}
		}
	}

	return c.JSON(http.StatusOK, res)
}

func (h *Handler) GetItem(c echo.Context) error {
	ctx := c.Request().Context()

	itemID, err := strconv.ParseInt(c.Param("itemID"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	item, err := h.ItemRepo.GetItem(ctx, itemID)
	// TODO: not found handling
	// http.StatusNotFound(404)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	category, err := h.ItemRepo.GetCategory(ctx, item.CategoryID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	res := item.ConvertToGetItemResponse()
	res.CategoryName = category.Name
	return c.JSON(http.StatusOK, res)
}

func (h *Handler) GetUserItems(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := strconv.ParseInt(c.Param("userID"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid userID type")
	}

	items, err := h.ItemRepo.GetItemsByUserID(ctx, userID)
	// TODO: not found handling
	// http.StatusNotFound(404)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	var res []getUserItemsResponse
	for _, item := range items {
		cat, err := h.ItemRepo.GetCategory(ctx, item.CategoryID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, err)
		}
		res = append(res, getUserItemsResponse{
			ID:           item.ID,
			Name:         item.Name,
			Price:        item.Price,
			CategoryName: cat.Name,
		})
	}

	return c.JSON(http.StatusOK, res)
}

func (h *Handler) GetCategories(c echo.Context) error {
	ctx := c.Request().Context()

	cats, err := h.ItemRepo.GetCategories(ctx)
	// TODO: not found handling
	// http.StatusNotFound(404)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	res := make([]getCategoriesResponse, len(cats))
	for i, cat := range cats {
		res[i] = getCategoriesResponse{ID: cat.ID, Name: cat.Name}
	}

	return c.JSON(http.StatusOK, res)
}

func (h *Handler) GetImage(c echo.Context) error {
	ctx := c.Request().Context()

	itemID, err := strconv.ParseInt(c.Param("itemID"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "invalid itemID type")
	}

	data, err := h.ItemRepo.GetItemImage(ctx, itemID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err)
	}

	return c.Blob(http.StatusOK, "image/jpeg", data)
}

func (h *Handler) SearchItems(c echo.Context) error {
	ctx := c.Request().Context()

	// get search word
	searchWord := c.Request().URL.Query().Get("name")
	if searchWord == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "please specified search word")
	}

	items, err := h.ItemRepo.SearchItemsByWord(ctx, searchWord)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	categories, err := h.ItemRepo.GetCategories(ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	res := make([]domain.GetItemResponse, len(items))
	for i, item := range items {
		res[i] = item.ConvertToGetItemResponse()
		// TODO: refactor here...
		if !(0 <= res[i].CategoryID && res[i].CategoryID <= int64(len(categories))) {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Errorf("invalid category ID: %d", res[i].CategoryID))
		}
		res[i].CategoryName = categories[res[i].CategoryID-1].Name
	}

	return c.JSON(http.StatusOK, res)
}

func (h *Handler) AddBalance(c echo.Context) error {
	ctx := c.Request().Context()

	// validation
	req := new(AddBalanceRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	if req.Balance <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Errorf("negative added balance is invalid: %d", req.Balance))
	}

	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err)
	}

	user, err := h.UserRepo.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusPreconditionFailed, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	if err := h.UserRepo.UpdateBalance(ctx, userID, user.Balance+req.Balance); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, "successful")
}

func (h *Handler) GetBalance(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err)
	}

	user, err := h.UserRepo.GetUser(ctx, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusPreconditionFailed, err.Error())
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, GetBalanceResponse{Balance: user.Balance})
}

func (h *Handler) Purchase(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, err)
	}

	itemID, err := strconv.ParseInt(c.Param("itemID"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	// TODO: use transaction
	buyer, err := h.UserRepo.GetUser(ctx, userID)
	if err != nil {
		// not found handling
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusPreconditionFailed, err)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	// TODO: this request can be reduced by updating UpdateItemStatus
	// e.g.) UpdateItemStatusIfOnSale, UpdateItemStatus(context, itemID, beforeCondition, afterCondition)
	item, err := h.ItemRepo.GetItem(ctx, itemID)
	if err != nil {
		// not found handling
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusPreconditionFailed, err)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	// update only when item status is on sale
	if item.Status != domain.ItemStatusOnSale {
		return echo.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("item is not on sale"))
	}
	// not to buy own items. 自身の商品を買おうとしていたら、http.StatusPreconditionFailed(412)
	if buyer.ID == item.UserID {
		return echo.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("failed to buy because of user owned item"))
	}

	sellerID := item.UserID
	seller, err := h.UserRepo.GetUser(ctx, sellerID)
	if err != nil {
		// not found handling
		if errors.Is(err, sql.ErrNoRows) {
			return echo.NewHTTPError(http.StatusPreconditionFailed, err)
		}
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	// if it is fail here, item status is still sold
	// balance consistency
	if buyer.Balance-item.Price < 0 {
		return echo.NewHTTPError(http.StatusPreconditionFailed, fmt.Errorf("failed to buy because of lack of balances: balance: %d, price: %d", buyer.Balance, item.Price))
	}
	if err := h.ItemRepo.UpdateItemStatus(ctx, itemID, domain.ItemStatusSoldOut); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	if err := h.UserRepo.UpdateBalance(ctx, userID, buyer.Balance-item.Price); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	if err := h.UserRepo.UpdateBalance(ctx, sellerID, seller.Balance+item.Price); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	return c.JSON(http.StatusOK, "successful")
}

func getUserID(c echo.Context) (int64, error) {
	user := c.Get("user").(*jwt.Token)
	// use same error for security reason
	if user == nil {
		return -1, fmt.Errorf("invalid token")
	}
	claims := user.Claims.(*JwtCustomClaims)
	if claims == nil {
		return -1, fmt.Errorf("invalid token")
	}
	if claims.UserID < 0 {
		return -1, fmt.Errorf("invalid token")
	}

	return claims.UserID, nil
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}
