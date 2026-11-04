// Package store persists accounts, characters and moderation data.
package store

import (
	"context"
	"errors"
	"time"
)

var (
	ErrExists   = errors.New("already exists")
	ErrNotFound = errors.New("not found")
)

type Account struct {
	ID          int64
	Username    string
	PassHash    string
	BannedUntil time.Time
	BanReason   string
	MutedUntil  time.Time
	IsAdmin     bool
}

type Outfit struct {
	Shirt int `json:"shirt"`
	Pants int `json:"pants"`
	Skin  int `json:"skin"`
	Hair  int `json:"hair"`
	HairC int `json:"hairc"`
	Hat   int `json:"hat"`
}

type QState struct {
	ID   string `json:"id"`
	Prog int    `json:"prog"`
	Done bool   `json:"done"`
}

type Quests struct {
	DayKey   string   `json:"dayKey"`
	Daily    []QState `json:"daily"`
	WeekKey  string   `json:"weekKey"`
	WeekProg int      `json:"weekProg"`
	WeekDone bool     `json:"weekDone"`
}

// CharData is the free-form, JSON-persisted part of a character.
type CharData struct {
	Inv        map[string]int `json:"inv"`
	Stamina    int            `json:"stamina"`
	Discipline int            `json:"disc"`
	Sentence   int            `json:"sentence"`
	Cell       int            `json:"cell"`
	X          float64        `json:"x"`
	Y          float64        `json:"y"`
	Z          float64        `json:"z"`
	Yaw        float64        `json:"yaw"`
	Face       int            `json:"face"`
	Quests     Quests         `json:"quests"`
}

type Character struct {
	ID, AccountID                         int64
	Name                                  string
	Outfit                                Outfit
	Money, Rep, Points, Escapes, Releases int
	Data                                  CharData
}

type TopRow struct {
	Name                                  string `json:"name"`
	Money, Rep, Points, Escapes, Releases int
}

type Report struct {
	ID               int64
	Reporter, Target string
	Reason           string
	At               time.Time
}

type Store interface {
	CreateAccount(ctx context.Context, username, hash string) (Account, error)
	AccountByName(ctx context.Context, username string) (Account, error)
	AccountByID(ctx context.Context, id int64) (Account, error)
	CreateCharacter(ctx context.Context, c *Character) error
	CharacterByAccount(ctx context.Context, accountID int64) (*Character, error)
	CharacterByName(ctx context.Context, name string) (*Character, error)
	SaveCharacter(ctx context.Context, c *Character) error
	Top(ctx context.Context, by string, n int) ([]TopRow, error)
	SetBan(ctx context.Context, username string, until time.Time, reason string) error
	SetMute(ctx context.Context, username string, until time.Time) error
	AddReport(ctx context.Context, reporterChar, targetChar int64, reason string) error
	Reports(ctx context.Context, n int) ([]Report, error)
	Close() error
}

// TopColumn whitelists the leaderboard sort keys.
func TopColumn(by string) string {
	switch by {
	case "money", "rep", "points", "escapes", "releases":
		return by
	}
	return "rep"
}
