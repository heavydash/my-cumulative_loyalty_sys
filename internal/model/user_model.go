package model

import "time"

type User struct {
	ID           int64     `db:"id"`
	Login        string    `db:"login"`
	PasswordHash string    `db:"password_hash"`
	CreatedAt    time.Time `db:"created_at"`
}

type creds struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}
