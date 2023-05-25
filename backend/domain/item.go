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

func (i *Item) ConvertToGetItemResponse(categoryName string) GetItemResponse {
	return GetItemResponse{
		ID:           i.ID,
		Name:         i.Name,
		CategoryID:   i.CategoryID,
		CategoryName: categoryName,
		UserID:       i.UserID,
		Price:        i.Price,
		Description:  i.Description,
		Status:       i.Status,
	}
}
