//go:generate mockgen -source=$GOFILE -destination=mock_$GOFILE -package=$GOPACKAGE
package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"

	"github.com/NamikoToriyama/mecari-build-hackathon-2023/backend/domain"
)

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
	AddItem(ctx context.Context, item domain.Item) (domain.Item, error)
	UpdateItem(ctx context.Context, item domain.Item) (domain.Item, error)
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

func (r *ItemDBRepository) AddItem(ctx context.Context, item domain.Item) (domain.Item, error) {
	if _, err := r.ExecContext(ctx, "INSERT INTO items (name, price, description, category_id, seller_id, image, status) VALUES (?, ?, ?, ?, ?, ?, ?)", item.Name, item.Price, item.Description, item.CategoryID, item.UserID, item.Image, item.Status); err != nil {
		return domain.Item{}, err
	}

	row := r.QueryRowContext(ctx, "SELECT * FROM items WHERE name=? AND price=? ORDER BY rowid DESC LIMIT 1", item.Name, item.Price)

	var res domain.Item
	return res, row.Scan(&res.ID, &res.Name, &res.Price, &res.Description, &res.CategoryID, &res.UserID, &res.Image, &res.Status, &res.CreatedAt, &res.UpdatedAt)
}

func (r *ItemDBRepository) UpdateItem(ctx context.Context, item domain.Item) (domain.Item, error) {
	if _, err := r.ExecContext(ctx, "UPDATE items SET name = ?, category_id = ?, price = ?, description = ? WHERE id = ?", item.Name, item.CategoryID, item.Price, item.Description, item.ID); err != nil {
		return domain.Item{}, err
	}

	return r.GetItem(ctx, item.ID)
}

func (r *ItemDBRepository) GetItem(ctx context.Context, id int64) (domain.Item, error) {
	row := r.QueryRowContext(ctx, "SELECT * FROM items WHERE id = ?", id)

	var item domain.Item
	return item, row.Scan(&item.ID, &item.Name, &item.Price, &item.Description, &item.CategoryID, &item.UserID, &item.Image, &item.Status, &item.CreatedAt, &item.UpdatedAt)
}

func (r *ItemDBRepository) GetItemImage(ctx context.Context, id int64) ([]byte, error) {
	row := r.QueryRowContext(ctx, "SELECT image FROM items WHERE id = ?", id)
	var image []byte
	return image, row.Scan(&image)
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
	fmt.Print("hogehogehoge")
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
