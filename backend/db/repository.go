//go:generate mockgen -source=$GOFILE -destination=mock_$GOFILE -package=$GOPACKAGE
package db

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"strconv"
	"sync"

	"github.com/NamikoToriyama/mecari-build-hackathon-2023/backend/domain"
)

const FILE_DIR = "./images/"

type UserRepository interface {
	AddUser(ctx context.Context, user domain.User) (int64, error)
	GetUser(ctx context.Context, id int64) (domain.User, error)
	UpdateBalance(ctx context.Context, id int64, balance int64) error
}

type UserDBRepository struct {
	*sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &UserDBRepository{DB: db}
}

func (r *UserDBRepository) AddUser(ctx context.Context, user domain.User) (int64, error) {
	if _, err := r.ExecContext(ctx, "INSERT INTO users (name, password) VALUES (?, ?)", user.Name, user.Password); err != nil {
		return 0, err
	}
	// TODO: if other insert query is executed at the same time, it might return wrong id
	// http.StatusConflict(409) 既に同じIDがあった場合
	row := r.QueryRowContext(ctx, "SELECT id FROM users WHERE rowid = LAST_INSERT_ROWID()")

	var id int64
	return id, row.Scan(&id)
}

func (r *UserDBRepository) GetUser(ctx context.Context, id int64) (domain.User, error) {
	row := r.QueryRowContext(ctx, "SELECT * FROM users WHERE id = ?", id)

	var user domain.User
	return user, row.Scan(&user.ID, &user.Name, &user.Password, &user.Balance)
}

func (r *UserDBRepository) UpdateBalance(ctx context.Context, id int64, balance int64) error {
	if _, err := r.ExecContext(ctx, "UPDATE users SET balance = ? WHERE id = ?", balance, id); err != nil {
		return err
	}
	return nil
}

type ItemRepository interface {
	AddItem(ctx context.Context, item domain.Item, file *multipart.FileHeader) (domain.Item, error)
	DeleteItems(ctx context.Context, item_id int64) error
	UpdateItem(ctx context.Context, item domain.Item, file *multipart.FileHeader) (domain.Item, error)
	GetItem(ctx context.Context, id int64) (domain.Item, error)
	GetItemImage(ctx context.Context, id int64) ([]byte, error)
	GetOnSaleItems(ctx context.Context) ([]domain.Item, error)
	GetItemsByUserID(ctx context.Context, userID int64) ([]domain.Item, error)
	GetCategory(ctx context.Context, id int64) (domain.Category, error)
	GetCategories(ctx context.Context) ([]domain.Category, error)
	UpdateItemStatus(ctx context.Context, id int64, status domain.ItemStatus) error
	SearchItemsByWord(ctx context.Context, word string) ([]domain.Item, error)
}

type ItemDBRepository struct {
	*sql.DB
}

func NewItemRepository(db *sql.DB) ItemRepository {
	return &ItemDBRepository{DB: db}
}

func (r *ItemDBRepository) AddItem(ctx context.Context, item domain.Item, file *multipart.FileHeader) (domain.Item, error) {
	if _, err := r.ExecContext(ctx, "INSERT INTO items (name, price, description, category_id, seller_id, image, status) VALUES (?, ?, ?, ?, ?, ?, ?)", item.Name, item.Price, item.Description, item.CategoryID, item.UserID, item.Image, item.Status); err != nil {
		return domain.Item{}, err
	}
	// TODO: if other insert query is executed at the same time, it might return wrong id
	// http.StatusConflict(409) 既に同じIDがあった場合
	row := r.QueryRowContext(ctx, "SELECT * FROM items WHERE rowid = LAST_INSERT_ROWID()")

	var res domain.Item
	err := row.Scan(&res.ID, &res.Name, &res.Price, &res.Description, &res.CategoryID, &res.UserID, &res.Image, &res.Status, &res.CreatedAt, &res.UpdatedAt)
	if err != nil {
		return domain.Item{}, err
	}

	err = saveImageLocal(res.ID, file)
	if err != nil {
		r.DeleteItems(ctx, res.ID)
		return domain.Item{}, err
	}

	return res, nil
}

func (r *ItemDBRepository) DeleteItems(ctx context.Context, item_id int64) error {
	if _, err := r.ExecContext(ctx, "DELETE FROM items WHERE id = ?", item_id); err != nil {
		return err
	}
	return nil
}

func saveImageLocal(id int64, file *multipart.FileHeader) error {
	src, err := file.Open()
	if err != nil {
		return err
	}

	defer func() {
		if err := src.Close(); err != nil {
			log.Printf("failed src.Close: %s", err.Error())
		}
	}()

	var dest []byte
	blob := bytes.NewBuffer(dest)
	if _, err := io.Copy(blob, src); err != nil {
		return err
	}

	out, err := os.Create(FILE_DIR + strconv.FormatInt(id, 10) + ".jpg")
	if err != nil {
		return err
	}
	defer out.Close()

	io.Copy(out, blob)

	return nil
}

func (r *ItemDBRepository) UpdateItem(ctx context.Context, item domain.Item, file *multipart.FileHeader) (domain.Item, error) {
	if _, err := r.ExecContext(ctx, "UPDATE items SET name = ?, category_id = ?, price = ?, description = ? WHERE id = ?", item.Name, item.CategoryID, item.Price, item.Description, item.ID); err != nil {
		return domain.Item{}, err
	}

	// Update images
	saveImageLocal(item.ID, file)

	return r.GetItem(ctx, item.ID)
}

func (r *ItemDBRepository) GetItem(ctx context.Context, id int64) (domain.Item, error) {
	row := r.QueryRowContext(ctx, "SELECT * FROM items WHERE id = ?", id)

	var item domain.Item
	err := row.Scan(&item.ID, &item.Name, &item.Price, &item.Description, &item.CategoryID, &item.UserID, &item.Image, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return domain.Item{}, err
	}

	img, err := r.GetItemImage(ctx, item.ID)
	if err != nil {
		return domain.Item{}, err
	}
	item.Image = img

	return item, nil
}

func (r *ItemDBRepository) GetItemImage(ctx context.Context, id int64) ([]byte, error) {
	f, err := os.Open(FILE_DIR + strconv.FormatInt(id, 10) + ".jpg")
	if err != nil {
		return nil, err
	}

	var dest []byte
	blob := bytes.NewBuffer(dest)
	if _, err := io.Copy(blob, f); err != nil {
		return nil, err
	}

	return blob.Bytes(), err
}

func (r *ItemDBRepository) GetOnSaleItems(ctx context.Context) ([]domain.Item, error) {
	rows, err := r.QueryContext(ctx, "SELECT * FROM items WHERE status = ? ORDER BY updated_at desc", domain.ItemStatusOnSale)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed rows.Close: %s", err.Error())
		}
	}()

	var items []domain.Item
	for rows.Next() {
		var item domain.Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Price, &item.Description, &item.CategoryID, &item.UserID, &item.Image, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ItemDBRepository) GetItemsByUserID(ctx context.Context, userID int64) ([]domain.Item, error) {
	rows, err := r.QueryContext(ctx, "SELECT * FROM items WHERE seller_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed rows.Close: %s", err.Error())
		}
	}()

	var items []domain.Item
	for rows.Next() {
		var item domain.Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Price, &item.Description, &item.CategoryID, &item.UserID, &item.Image, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *ItemDBRepository) UpdateItemStatus(ctx context.Context, id int64, status domain.ItemStatus) error {
	if _, err := r.ExecContext(ctx, "UPDATE items SET status = ? WHERE id = ?", status, id); err != nil {
		return err
	}
	return nil
}

/*
$ head -6 10_data.sql
BEGIN TRANSACTION;

INSERT INTO "category" VALUES(1,'food');
INSERT INTO "category" VALUES(2,'fashion');
INSERT INTO "category" VALUES(3,'furniture');
*/
var (
	categories = []domain.Category{
		{ID: 1, Name: "food"},
		{ID: 2, Name: "fashion"},
		{ID: 3, Name: "furniture"},
	}
	categoryMu sync.RWMutex
)

func (r *ItemDBRepository) GetCategory(ctx context.Context, id int64) (domain.Category, error) {
	if !(1 <= id && id <= int64(len(categories))) {
		return domain.Category{}, fmt.Errorf("invalid category ID: %d", id)
	}

	categoryMu.RLock()
	defer categoryMu.RUnlock()
	return categories[id-1], nil
}

func (r *ItemDBRepository) GetCategories(ctx context.Context) ([]domain.Category, error) {
	categoryMu.RLock()
	defer categoryMu.RUnlock()
	return categories, nil
}

func (r *ItemDBRepository) SearchItemsByWord(ctx context.Context, word string) ([]domain.Item, error) {
	rows, err := r.QueryContext(ctx, "SELECT * FROM items WHERE name like ?", "%"+word+"%")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("failed rows.Close: %s", err.Error())
		}
	}()

	var items []domain.Item
	for rows.Next() {
		var item domain.Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Price, &item.Description, &item.CategoryID, &item.UserID, &item.Image, &item.Status, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
