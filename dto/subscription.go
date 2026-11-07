package dto

import "time"

type PurchaseSubscriptionRequest struct {
	UserID int    `json:"user_id"`
	Tier   string `json:"tier"`
}

type SubscriptionResponse struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	IsActive     bool      `json:"is_active"`
	Tier         string    `json:"tier"`
	Price        int       `json:"price"`
	BytesTotal   int64     `json:"bytes_total"`
	BytesUsed    int64     `json:"bytes_used"`
	StartDate    time.Time `json:"start_date"`
	EndDate      time.Time `json:"end_date"`
	PurchaseDate time.Time `json:"purchase_date"`
}
