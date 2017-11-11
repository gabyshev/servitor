package types

import (
	"time"
)

type Chat struct {
	ID           int64
	Username     string
	Firstname    string
	Lastname     string
	IsAuthorized bool
	Created      time.Time
	Updated      time.Time
}
