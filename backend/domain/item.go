package domain

type ItemStatus int

const (
	ItemStatusInitial ItemStatus = iota
	ItemStatusOnSale
	ItemStatusSoldOut
)

type Item struct {
	ID          int64
	Name        string
	Price       int64
	Description string
	CategoryID  int64
	UserID      int64
	Image       []byte
	Status      ItemStatus
	CreatedAt   string
	UpdatedAt   string
}

type GetItemResponse struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	CategoryID   int64      `json:"category_id"`
	CategoryName string     `json:"category_name"`
	UserID       int64      `json:"user_id"`
	Price        int64      `json:"price"`
	Description  string     `json:"description"`
	Status       ItemStatus `json:"status"`
}

type Category struct {
	ID   int64
	Name string
}

/*
$ head -6 10_data.sql
BEGIN TRANSACTION;

INSERT INTO "category" VALUES(1,'food');
INSERT INTO "category" VALUES(2,'fashion');
INSERT INTO "category" VALUES(3,'furniture');
*/
var (
	categories = []Category{{1, "food"}, {2, "fashion"}, {3, "furniture"}}
)

func (i *Item) ConvertToGetItemResponse() GetItemResponse {
	// TODO: validation
	return GetItemResponse{
		ID:           i.ID,
		Name:         i.Name,
		CategoryID:   i.CategoryID,
		CategoryName: categories[i.CategoryID-1].Name,
		UserID:       i.UserID,
		Price:        i.Price,
		Description:  i.Description,
		Status:       i.Status,
	}
}
