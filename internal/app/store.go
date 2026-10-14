package app

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

//go:embed seed.json
var seedJSON []byte

type Store struct {
	DB      *gorm.DB
	DataDir string
}

// These models retain the existing on-disk layout, including JSON content payloads.
// HTTP models stay independent from storage models to keep the public API stable.
type contentRecord struct {
	ID      string `gorm:"primaryKey"`
	Payload string
}
type caseRecord struct {
	ID         string `gorm:"primaryKey"`
	CategoryID string
	ClientID   *string
	Status     string
	Payload    string
}

func (caseRecord) TableName() string { return "cases" }

const schema = `CREATE TABLE IF NOT EXISTS categories(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS clients(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS cases(id TEXT PRIMARY KEY,category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,client_id TEXT REFERENCES clients(id) ON DELETE RESTRICT,status TEXT NOT NULL CHECK(status IN ('draft','published','archived')),payload TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS settings(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS uploads(id TEXT PRIMARY KEY,filename TEXT NOT NULL,mime TEXT NOT NULL,size INTEGER NOT NULL,original_name TEXT NOT NULL,created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS admins(id INTEGER PRIMARY KEY CHECK(id=1),username TEXT NOT NULL UNIQUE,password TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS sessions(hash TEXT PRIMARY KEY,csrf TEXT NOT NULL,expires INTEGER NOT NULL);`

func OpenStore(dataDir string) (*Store, error) {
	abs, err := filepath.Abs(dataDir)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(abs, 0700); err != nil {
		return nil, err
	}
	u := url.URL{Scheme: "file", Path: filepath.Join(abs, "content.sqlite")}
	params := url.Values{}
	params.Add("_pragma", "foreign_keys(1)")
	params.Add("_pragma", "busy_timeout(5000)")
	params.Add("_pragma", "journal_mode(WAL)")
	u.RawQuery = params.Encode()
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	orm, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", Conn: db}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// Each multi-step write has an explicit transaction; avoid redundant transactions.
		SkipDefaultTransaction: true,
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	s := &Store{DB: orm, DataDir: abs}
	// Explicit DDL preserves CHECK constraints and foreign keys of existing databases.
	if err = orm.Exec(schema).Error; err != nil {
		db.Close()
		return nil, err
	}
	var count int64
	if err = orm.Table("settings").Where("id = ?", "initialized").Count(&count).Error; err != nil {
		db.Close()
		return nil, err
	}
	if count == 0 {
		if err = s.seed(); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err = s.seedTeamMembers(); err != nil {
		db.Close()
		return nil, err
	}
	_ = os.Chmod(filepath.Join(abs, "content.sqlite"), 0600)
	return s, nil
}
func validTable(table string) bool {
	return table == "categories" || table == "clients" || table == "cases" || table == "settings"
}
func listRows[T any](q *gorm.DB, table string) ([]T, error) {
	if !validTable(table) {
		return nil, fmt.Errorf("invalid table")
	}
	var records []contentRecord
	if err := q.Table(table).Select("payload").Order("id").Find(&records).Error; err != nil {
		return nil, err
	}
	result := make([]T, 0, len(records))
	for _, record := range records {
		var item T
		if err := json.Unmarshal([]byte(record.Payload), &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}
func getRow[T any](q *gorm.DB, table, id string) (T, error) {
	var item T
	if !validTable(table) {
		return item, fmt.Errorf("invalid table")
	}
	var record contentRecord
	if err := q.Table(table).Select("payload").Where("id = ?", id).Take(&record).Error; err != nil {
		return item, err
	}
	err := json.Unmarshal([]byte(record.Payload), &item)
	return item, err
}
func putRow(q *gorm.DB, table string, value any) error {
	if !validTable(table) {
		return fmt.Errorf("invalid table")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var meta struct {
		ID string `json:"id"`
	}
	if err = json.Unmarshal(raw, &meta); err != nil {
		return err
	}
	if meta.ID == "" {
		return fmt.Errorf("empty record ID")
	}
	conflict := clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoUpdates: clause.AssignmentColumns([]string{"payload"})}
	if c, ok := value.(Case); ok {
		var client *string
		if c.ClientID != "" {
			client = &c.ClientID
		}
		record := caseRecord{ID: c.ID, CategoryID: c.CategoryID, ClientID: client, Status: c.Status, Payload: string(raw)}
		conflict.DoUpdates = clause.AssignmentColumns([]string{"category_id", "client_id", "status", "payload"})
		return q.Clauses(conflict).Create(&record).Error
	}
	record := contentRecord{ID: meta.ID, Payload: string(raw)}
	return q.Table(table).Clauses(conflict).Create(&record).Error
}
func deleteRow(q *gorm.DB, table, id string) error {
	if !validTable(table) {
		return fmt.Errorf("invalid table")
	}
	result := q.Table(table).Where("id = ?", id).Delete(&contentRecord{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (s *Store) seed() error {
	var data Snapshot
	if err := json.Unmarshal(seedJSON, &data); err != nil {
		return err
	}
	tx := s.DB.Begin()
	var err error
	if err = tx.Error; err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range data.Categories {
		if err = putRow(tx, "categories", c); err != nil {
			return err
		}
	}
	for _, c := range data.Clients {
		if err = putRow(tx, "clients", c); err != nil {
			return err
		}
	}
	for _, c := range data.Cases {
		if err = putRow(tx, "cases", c); err != nil {
			return err
		}
	}
	data.Company.ID = "company"
	if err = putRow(tx, "settings", data.Company); err != nil {
		return err
	}
	if err = putRow(tx, "settings", platformSetting{ID: "platforms", Items: data.Platforms}); err != nil {
		return err
	}
	if err = putRow(tx, "settings", struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
	}{"initialized", 1}); err != nil {
		return err
	}
	return tx.Commit().Error
}
func (s *Store) Uploads() ([]Upload, error) {
	items := make([]Upload, 0)
	err := s.DB.Order("created_at DESC").Find(&items).Error
	return items, err
}
func (s *Store) Upload(filename string) (Upload, error) {
	var item Upload
	err := s.DB.Where("filename = ?", filename).Take(&item).Error
	return item, err
}
func (s *Store) Snapshot() (Snapshot, error) {
	var data Snapshot
	var err error
	if data.Cases, err = listRows[Case](s.DB, "cases"); err != nil {
		return data, err
	}
	if data.Categories, err = listRows[Category](s.DB, "categories"); err != nil {
		return data, err
	}
	if data.Clients, err = listRows[Client](s.DB, "clients"); err != nil {
		return data, err
	}
	if data.Company, err = getRow[Company](s.DB, "settings", "company"); err != nil {
		return data, err
	}
	p, err := getRow[platformSetting](s.DB, "settings", "platforms")
	if err != nil {
		return data, err
	}
	data.Platforms = p.Items
	if data.Uploads, err = s.Uploads(); err != nil {
		return data, err
	}
	sort.SliceStable(data.Cases, func(i, j int) bool { return data.Cases[i].Order < data.Cases[j].Order })
	sort.SliceStable(data.Categories, func(i, j int) bool { return data.Categories[i].Order < data.Categories[j].Order })
	sort.SliceStable(data.Clients, func(i, j int) bool { return data.Clients[i].Order < data.Clients[j].Order })
	sort.SliceStable(data.Platforms, func(i, j int) bool { return data.Platforms[i].Order < data.Platforms[j].Order })
	return data, nil
}
func (s *Store) Close() error {
	db, err := s.DB.DB()
	if err != nil {
		return err
	}
	return db.Close()
}

// Add PPT team data once for legacy content. An explicit empty array remains empty.
func (s *Store) seedTeamMembers() error {
	record, err := getRow[map[string]json.RawMessage](s.DB, "settings", "company")
	if err == gorm.ErrRecordNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if _, exists := record["teamMembers"]; exists {
		return nil
	}
	var defaults Snapshot
	if err = json.Unmarshal(seedJSON, &defaults); err != nil {
		return err
	}
	raw, err := json.Marshal(defaults.Company.TeamMembers)
	if err != nil {
		return err
	}
	record["teamMembers"] = raw
	return putRow(s.DB, "settings", record)
}
