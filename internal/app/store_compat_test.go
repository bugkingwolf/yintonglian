package app

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// Build a legacy database with SQL, independent of GORM's record mappings.
func TestGORMOpensLegacyDatabase(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "content.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(schema); err != nil {
		t.Fatal(err)
	}
	fixtures := []struct{ table, id, payload string }{
		{"settings", "initialized", `{"id":"initialized","version":1}`},
		{"categories", "legacy-category", `{"id":"legacy-category","name":"旧分类","services":[],"order":2}`},
		{"clients", "legacy-client", `{"id":"legacy-client","name":"旧客户","order":3}`},
	}
	for _, x := range fixtures {
		if _, err = db.Exec("INSERT INTO "+x.table+"(id,payload) VALUES(?,?)", x.id, x.payload); err != nil {
			t.Fatal(err)
		}
	}
	raw := `{"id":"legacy-case","title":"旧作品","categoryId":"legacy-category","clientId":"legacy-client","status":"draft","gallery":[],"services":[]}`
	if _, err = db.Exec("INSERT INTO cases(id,category_id,client_id,status,payload) VALUES(?,?,?,?,?)", "legacy-case", "legacy-category", "legacy-client", "draft", raw); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	c, err := getRow[Case](store.DB, "cases", "legacy-case")
	if err != nil || c.Title != "旧作品" || c.ClientID != "legacy-client" {
		t.Fatalf("legacy read: %+v, %v", c, err)
	}
	var stored contentRecord
	if err = store.DB.Table("cases").Where("id = ?", c.ID).Take(&stored).Error; err != nil || stored.Payload != raw {
		t.Fatalf("opening modified payload: %v", err)
	}
	c.Title = "修改后作品"
	c.Hero = true
	if err = putRow(store.DB, "cases", c); err != nil {
		t.Fatal(err)
	}
	got, err := getRow[Case](store.DB, "cases", c.ID)
	if err != nil || got.Title != c.Title || !got.Hero {
		t.Fatalf("legacy update: %+v, %v", got, err)
	}
	if err = deleteRow(store.DB, "categories", c.CategoryID); err == nil {
		t.Fatal("legacy foreign key protection lost")
	}
	c.ClientID = ""
	if err = putRow(store.DB, "cases", c); err != nil {
		t.Fatal(err)
	}
	var record caseRecord
	if err = store.DB.Take(&record, "id = ?", c.ID).Error; err != nil || record.ClientID != nil {
		t.Fatalf("empty client must remain SQL NULL: %+v, %v", record, err)
	}
}
