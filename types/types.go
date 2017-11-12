package types

import (
	"golang.org/x/oauth2"
	"time"
)

type Chat struct {
	ID           int64
	Username     string
	Firstname    string
	Lastname     string
	Token        *oauth2.Token
	IsAuthorized bool
	Created      time.Time
	Updated      time.Time
}
