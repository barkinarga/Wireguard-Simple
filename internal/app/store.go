package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db        *sql.DB
	dir       string
	telemetry *sql.DB
	teleMu    sync.Mutex
}

func OpenStore(dir, iface string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0700); err != nil {
		return nil, err
	}
	file := filepath.Join(dir, "control.db")
	f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	if err := os.Chmod(file, 0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db, dir: dir}
	_, err = db.Exec(`PRAGMA busy_timeout=5000; PRAGMA journal_mode=DELETE; PRAGMA synchronous=FULL; PRAGMA temp_store=MEMORY; PRAGMA cache_size=-1024; PRAGMA max_page_count=4096;
	CREATE TABLE IF NOT EXISTS config (key TEXT PRIMARY KEY, value TEXT NOT NULL);
	CREATE TABLE IF NOT EXISTS peers (id TEXT PRIMARY KEY, public_key TEXT UNIQUE NOT NULL, value TEXT NOT NULL);
	CREATE TABLE IF NOT EXISTS admin (id INTEGER PRIMARY KEY CHECK(id=1), username TEXT NOT NULL, password TEXT NOT NULL);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	var n int
	if err = db.QueryRow(`SELECT COUNT(*) FROM config`).Scan(&n); err != nil {
		_ = db.Close()
		return nil, err
	}
	if n == 0 {
		tx, e := db.Begin()
		if e != nil {
			_ = db.Close()
			return nil, e
		}
		for k, v := range map[string]any{"server": DefaultServer(iface), "settings": DefaultSettings()} {
			b, _ := json.Marshal(v)
			if _, e = tx.Exec(`INSERT INTO config(key,value) VALUES(?,?)`, k, string(b)); e != nil {
				_ = tx.Rollback()
				_ = db.Close()
				return nil, e
			}
		}
		if e = tx.Commit(); e != nil {
			_ = db.Close()
			return nil, e
		}
	}
	return s, nil
}

func (s *Store) Close() error {
	s.teleMu.Lock()
	defer s.teleMu.Unlock()
	if s.telemetry != nil {
		_ = s.telemetry.Close()
	}
	return s.db.Close()
}
func (s *Store) readConfig(key string, v any) error {
	var raw string
	if err := s.db.QueryRow(`SELECT value FROM config WHERE key=?`, key).Scan(&raw); err != nil {
		return err
	}
	return json.Unmarshal([]byte(raw), v)
}
func (s *Store) Server() (v Server, err error)     { err = s.readConfig("server", &v); return }
func (s *Store) Settings() (v Settings, err error) { err = s.readConfig("settings", &v); return }
func (s *Store) Peers() ([]Peer, error) {
	rows, err := s.db.Query(`SELECT value FROM peers ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Peer{}
	for rows.Next() {
		var raw string
		var p Peer
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal([]byte(raw), &p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// The caller serializes control changes and restores the previous runtime config
// if SQLite cannot commit after a successful WireGuard update.
func (s *Store) SaveControl(server Server, peers []Peer) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	b, err := json.Marshal(server)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(`UPDATE config SET value=? WHERE key='server'`, string(b)); err != nil {
		return err
	}
	if _, err = tx.Exec(`DELETE FROM peers`); err != nil {
		return err
	}
	for _, p := range peers {
		b, err = json.Marshal(p)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO peers(id,public_key,value) VALUES(?,?,?)`, p.ID, p.PublicKey, string(b)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func (s *Store) SaveSettings(v Settings) error {
	if err := v.Validate(); err != nil {
		return err
	}
	b, _ := json.Marshal(v)
	_, err := s.db.Exec(`UPDATE config SET value=? WHERE key='settings'`, string(b))
	return err
}
func (s *Store) HasAdmin() bool {
	var n int
	return s.db.QueryRow(`SELECT COUNT(*) FROM admin`).Scan(&n) == nil && n == 1
}
func (s *Store) CreateAdmin(username, hash string) error {
	_, err := s.db.Exec(`INSERT INTO admin(id,username,password) VALUES(1,?,?)`, username, hash)
	return err
}
func (s *Store) Admin() (username, hash string, err error) {
	err = s.db.QueryRow(`SELECT username,password FROM admin WHERE id=1`).Scan(&username, &hash)
	return
}
func (s *Store) ChangePassword(hash string) error {
	_, err := s.db.Exec(`UPDATE admin SET password=? WHERE id=1`, hash)
	return err
}

func (s *Store) openTelemetry(cfg Settings) (*sql.DB, error) {
	if s.telemetry != nil {
		return s.telemetry, nil
	}
	file := filepath.Join(s.dir, "telemetry.db")
	f, err := os.OpenFile(file, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	_ = f.Close()
	if err = os.Chmod(file, 0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	// DELETE journal may temporarily be as large as the database. Reserve half
	// the total budget for it; all query temp storage remains in RAM.
	_, err = db.Exec(`PRAGMA page_size=4096; PRAGMA auto_vacuum=FULL; PRAGMA journal_mode=DELETE; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=2000; PRAGMA temp_store=MEMORY; PRAGMA cache_size=-512;
	CREATE TABLE IF NOT EXISTS records(id INTEGER PRIMARY KEY, time INTEGER NOT NULL, kind TEXT NOT NULL, value TEXT NOT NULL);
	CREATE INDEX IF NOT EXISTS records_time ON records(time);`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	s.telemetry = db
	return db, nil
}

func (s *Store) pruneTelemetry(db *sql.DB, cfg Settings) error {
	if _, err := db.Exec(`DELETE FROM records WHERE time < ?`, time.Now().Add(-time.Duration(cfg.RetentionDays)*24*time.Hour).Unix()); err != nil {
		return err
	}
	maxPages := (int64(cfg.BudgetMB)*1024*1024 - 1024*1024) / 2 / 4096
	for i := 0; i < 64; i++ {
		var pages int64
		if err := db.QueryRow(`PRAGMA page_count`).Scan(&pages); err != nil {
			return err
		}
		if pages < maxPages*9/10 {
			break
		}
		var count int64
		if err := db.QueryRow(`SELECT count(*) FROM records`).Scan(&count); err != nil {
			return err
		}
		batch := count / 4
		if batch < 512 {
			batch = 512
		}
		result, err := db.Exec(`DELETE FROM records WHERE id IN (SELECT id FROM records ORDER BY id LIMIT ?)`, batch)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			break
		}
	}
	var actual int64
	if err := db.QueryRow(fmt.Sprintf(`PRAGMA max_page_count=%d`, maxPages)).Scan(&actual); err != nil {
		return err
	}
	if actual > maxPages {
		return errors.New("Could not shrink history database to the selected budget")
	}
	return nil
}

func (s *Store) Record(cfg Settings, kind string, value any) error {
	if (kind == "sample" && !cfg.HistoryEnabled) || (kind == "audit" && !cfg.AuditEnabled) {
		return nil
	}
	s.teleMu.Lock()
	defer s.teleMu.Unlock()
	free, _, err := diskSpace(s.dir)
	if err == nil && free < 64*1024*1024 {
		return errors.New("Less than 64 MB free disk space: recording paused")
	}
	db, err := s.openTelemetry(cfg)
	if err != nil {
		return errors.New("Could not open history storage")
	}
	if err = s.pruneTelemetry(db, cfg); err != nil {
		return errors.New("Could not complete history cleanup")
	}
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(b) > 256*1024 {
		return errors.New("History sample exceeds the size limit")
	}
	if _, err = db.Exec(`INSERT INTO records(time,kind,value) VALUES(?,?,?)`, time.Now().Unix(), kind, string(b)); err != nil {
		return errors.New("History budget reached or disk write failed")
	}
	return nil
}

type PeerSample struct {
	ID        string     `json:"id"`
	Rx        uint64     `json:"rx"`
	Tx        uint64     `json:"tx"`
	Handshake *time.Time `json:"handshake,omitempty"`
}
type HistorySample struct {
	Point Point        `json:"point"`
	Peers []PeerSample `json:"peers"`
}

func (s *Store) History(limit int) ([]Point, error) {
	s.teleMu.Lock()
	defer s.teleMu.Unlock()
	out := []Point{}
	if s.telemetry == nil {
		if _, err := os.Stat(filepath.Join(s.dir, "telemetry.db")); err != nil {
			return out, nil
		}
		cfg, err := s.Settings()
		if err != nil {
			return nil, err
		}
		if _, err = s.openTelemetry(cfg); err != nil {
			return nil, err
		}
	}
	rows, err := s.telemetry.Query(`SELECT value FROM records WHERE kind='sample' ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		var sample HistorySample
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(raw), &sample) == nil {
			out = append(out, sample.Point)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func (s *Store) Audits(limit int) ([]Event, error) {
	s.teleMu.Lock()
	defer s.teleMu.Unlock()
	out := []Event{}
	if _, err := os.Stat(filepath.Join(s.dir, "telemetry.db")); os.IsNotExist(err) {
		return out, nil
	}
	cfg, err := s.Settings()
	if err != nil {
		return nil, err
	}
	db, err := s.openTelemetry(cfg)
	if err != nil {
		return nil, err
	}
	if err = s.pruneTelemetry(db, cfg); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT value FROM records WHERE kind='audit' ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		var event Event
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if json.Unmarshal([]byte(raw), &event) == nil {
			out = append(out, event)
		}
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, rows.Err()
}

func (s *Store) MetricsHash() string {
	var hash string
	_ = s.readConfig("metricsHash", &hash)
	return hash
}
func (s *Store) SetMetricsHash(hash string) error {
	b, _ := json.Marshal(hash)
	_, err := s.db.Exec(`INSERT INTO config(key,value) VALUES('metricsHash',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, string(b))
	return err
}
func (s *Store) ReconfigureTelemetry(cfg Settings) error {
	s.teleMu.Lock()
	defer s.teleMu.Unlock()
	if s.telemetry == nil {
		if _, err := os.Stat(filepath.Join(s.dir, "telemetry.db")); os.IsNotExist(err) {
			return nil
		}
		db, err := s.openTelemetry(cfg)
		if err != nil {
			return err
		}
		s.telemetry = db
	}
	return s.pruneTelemetry(s.telemetry, cfg)
}
func (s *Store) ClearTelemetry() error {
	s.teleMu.Lock()
	defer s.teleMu.Unlock()
	if s.telemetry != nil {
		if err := s.telemetry.Close(); err != nil {
			return err
		}
		s.telemetry = nil
	}
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		if err := os.Remove(filepath.Join(s.dir, "telemetry.db"+suffix)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}
func (s *Store) StorageBytes() (control, telemetry int64) {
	for _, name := range []string{"control.db", "control.db-journal", "telemetry.db", "telemetry.db-journal", "telemetry.db-wal", "telemetry.db-shm"} {
		if st, e := os.Stat(filepath.Join(s.dir, name)); e == nil {
			if strings.HasPrefix(name, "control") {
				control += st.Size()
			} else {
				telemetry += st.Size()
			}
		}
	}
	return
}

func atomicFile(path string, b []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".wgsimple-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	ce := f.Close()
	if err == nil {
		err = ce
	}
	if err != nil {
		return err
	}
	return os.Rename(name, path)
}
