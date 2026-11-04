package store

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/lib/pq"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Postgres struct{ db *sql.DB }

// OpenPostgres connects, applies pending migrations and returns the store.
func OpenPostgres(ctx context.Context, url string) (*Postgres, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(4)
	db.SetConnMaxLifetime(30 * time.Minute)
	var perr error
	for i := 0; i < 10; i++ { // the database may still be starting (docker compose)
		if perr = db.PingContext(ctx); perr == nil {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	if perr != nil {
		return nil, perr
	}
	p := &Postgres{db}
	if err := p.migrate(ctx); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return p, nil
}

func (p *Postgres) migrate(ctx context.Context) error {
	conn, err := p.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock(727274)`); err != nil {
		return err
	}
	defer conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock(727274)`)
	if _, err := conn.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	files, _ := migrationFS.ReadDir("migrations")
	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name())
	}
	sort.Strings(names)
	for _, n := range names {
		var done bool
		if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, n).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		sqlText, _ := migrationFS.ReadFile("migrations/" + n)
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, string(sqlText)); err != nil {
			tx.Rollback()
			return fmt.Errorf("%s: %w", n, err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, n); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Postgres) Close() error { return p.db.Close() }

func isUnique(err error) bool {
	var pe *pq.Error
	return errors.As(err, &pe) && pe.Code == "23505"
}

func nt(t sql.NullTime) time.Time {
	if t.Valid {
		return t.Time
	}
	return time.Time{}
}

func (p *Postgres) CreateAccount(ctx context.Context, username, hash string) (Account, error) {
	a := Account{Username: strings.ToLower(username), PassHash: hash}
	err := p.db.QueryRowContext(ctx, `INSERT INTO accounts(username, pass_hash) VALUES($1,$2) RETURNING id`, a.Username, hash).Scan(&a.ID)
	if isUnique(err) {
		return Account{}, ErrExists
	}
	return a, err
}

const accCols = `id, username, pass_hash, banned_until, ban_reason, muted_until, is_admin`

func scanAcc(row *sql.Row) (Account, error) {
	var a Account
	var b, m sql.NullTime
	err := row.Scan(&a.ID, &a.Username, &a.PassHash, &b, &a.BanReason, &m, &a.IsAdmin)
	if errors.Is(err, sql.ErrNoRows) {
		return Account{}, ErrNotFound
	}
	a.BannedUntil, a.MutedUntil = nt(b), nt(m)
	return a, err
}

func (p *Postgres) AccountByName(ctx context.Context, username string) (Account, error) {
	return scanAcc(p.db.QueryRowContext(ctx, `SELECT `+accCols+` FROM accounts WHERE username=$1`, strings.ToLower(username)))
}

func (p *Postgres) AccountByID(ctx context.Context, id int64) (Account, error) {
	return scanAcc(p.db.QueryRowContext(ctx, `SELECT `+accCols+` FROM accounts WHERE id=$1`, id))
}

func (p *Postgres) CreateCharacter(ctx context.Context, c *Character) error {
	o, _ := json.Marshal(c.Outfit)
	d, _ := json.Marshal(c.Data)
	err := p.db.QueryRowContext(ctx, `INSERT INTO characters(account_id,name,outfit,data,money,rep,points,escapes,releases)
		VALUES($1,$2,$3::jsonb,$4::jsonb,$5,$6,$7,$8,$9) RETURNING id`,
		c.AccountID, c.Name, string(o), string(d), c.Money, c.Rep, c.Points, c.Escapes, c.Releases).Scan(&c.ID)
	if isUnique(err) {
		return ErrExists
	}
	return err
}

const chCols = `id, account_id, name, outfit, data, money, rep, points, escapes, releases`

func scanChar(row *sql.Row) (*Character, error) {
	var c Character
	var o, d []byte
	err := row.Scan(&c.ID, &c.AccountID, &c.Name, &o, &d, &c.Money, &c.Rep, &c.Points, &c.Escapes, &c.Releases)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(o, &c.Outfit); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(d, &c.Data); err != nil {
		return nil, err
	}
	if c.Data.Inv == nil {
		c.Data.Inv = map[string]int{}
	}
	return &c, nil
}

func (p *Postgres) CharacterByAccount(ctx context.Context, accountID int64) (*Character, error) {
	return scanChar(p.db.QueryRowContext(ctx, `SELECT `+chCols+` FROM characters WHERE account_id=$1`, accountID))
}

func (p *Postgres) CharacterByName(ctx context.Context, name string) (*Character, error) {
	return scanChar(p.db.QueryRowContext(ctx, `SELECT `+chCols+` FROM characters WHERE lower(name)=lower($1)`, name))
}

func (p *Postgres) SaveCharacter(ctx context.Context, c *Character) error {
	o, _ := json.Marshal(c.Outfit)
	d, _ := json.Marshal(c.Data)
	res, err := p.db.ExecContext(ctx, `UPDATE characters SET name=$2, outfit=$3::jsonb, data=$4::jsonb, money=$5, rep=$6, points=$7,
		escapes=$8, releases=$9, updated_at=now() WHERE id=$1`, c.ID, c.Name, string(o), string(d), c.Money, c.Rep, c.Points, c.Escapes, c.Releases)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) Top(ctx context.Context, by string, n int) ([]TopRow, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT name, money, rep, points, escapes, releases FROM characters ORDER BY `+TopColumn(by)+` DESC, id LIMIT $1`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TopRow
	for rows.Next() {
		var r TopRow
		if err := rows.Scan(&r.Name, &r.Money, &r.Rep, &r.Points, &r.Escapes, &r.Releases); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *Postgres) SetBan(ctx context.Context, username string, until time.Time, reason string) error {
	res, err := p.db.ExecContext(ctx, `UPDATE accounts SET banned_until=$2, ban_reason=$3 WHERE username=$1`, strings.ToLower(username), nullTime(until), reason)
	return affected(res, err)
}

func (p *Postgres) SetMute(ctx context.Context, username string, until time.Time) error {
	res, err := p.db.ExecContext(ctx, `UPDATE accounts SET muted_until=$2 WHERE username=$1`, strings.ToLower(username), nullTime(until))
	return affected(res, err)
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) AddReport(ctx context.Context, reporter, target int64, reason string) error {
	_, err := p.db.ExecContext(ctx, `INSERT INTO reports(reporter,target,reason) VALUES($1,$2,$3)`, reporter, target, reason)
	return err
}

func (p *Postgres) Reports(ctx context.Context, n int) ([]Report, error) {
	rows, err := p.db.QueryContext(ctx, `SELECT r.id, COALESCE(a.name,'?'), COALESCE(b.name,'?'), r.reason, r.created_at
		FROM reports r LEFT JOIN characters a ON a.id=r.reporter LEFT JOIN characters b ON b.id=r.target
		ORDER BY r.id DESC LIMIT $1`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Report
	for rows.Next() {
		var r Report
		if err := rows.Scan(&r.ID, &r.Reporter, &r.Target, &r.Reason, &r.At); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
