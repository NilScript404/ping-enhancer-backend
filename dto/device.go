package dto

type DeviceRequest struct {
	Hwid string `json:"hwid"`
}

type DeviceResponse struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id,omitempty"`
	Hwid       string `json:"hwid"`
	BytesTotal int64  `json:"bytes_total"`
	BytesUsed  int64  `json:"bytes_used"`
}
