package models

type Message struct {
	Email    string `json:"email"`
	Telegram string `json:"telegram"`
	Phone    int    `json:"phone"`
	Code     string `json:"code"`
	Uuid     string `json:"uuid"`
}
