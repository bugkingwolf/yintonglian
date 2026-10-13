package app

type Category struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Subtitle    string   `json:"subtitle"`
	Description string   `json:"description"`
	Services    []string `json:"services"`
	Order       int      `json:"order"`
}
type Client struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Order int    `json:"order"`
}
type Case struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	ClientID      string   `json:"clientId"`
	CategoryID    string   `json:"categoryId"`
	Type          string   `json:"type"`
	Summary       string   `json:"summary"`
	Description   string   `json:"description"`
	Cover         string   `json:"cover"`
	Gallery       []string `json:"gallery"`
	Video         string   `json:"video"`
	ExperienceURL string   `json:"experienceUrl"`
	Services      []string `json:"services"`
	Featured      bool     `json:"featured"`
	Hero          bool     `json:"hero"`
	Demo          bool     `json:"demo"`
	Status        string   `json:"status"`
	Order         int      `json:"order"`
	UpdatedAt     string   `json:"updatedAt"`
}
type Company struct {
	ID                  string   `json:"id"`
	Name                string   `json:"name"`
	English             string   `json:"english"`
	LegalName           string   `json:"legalName"`
	Headline            string   `json:"headline"`
	Slogan              string   `json:"slogan"`
	Introduction        string   `json:"introduction"`
	Vision              string   `json:"vision"`
	Philosophy          string   `json:"philosophy"`
	Team                string   `json:"team"`
	Qualifications      string   `json:"qualifications"`
	QualificationImages []string `json:"qualificationImages"`
	Phone               string   `json:"phone"`
	Email               string   `json:"email"`
	Address             string   `json:"address"`
	ContactNote         string   `json:"contactNote"`
	Founded             string   `json:"founded"`
	HomeFeaturedLimit   int      `json:"homeFeaturedLimit"`
}
type Platform struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Subtitle    string `json:"subtitle"`
	Description string `json:"description"`
	URL         string `json:"url"`
	MobileURL   string `json:"mobileUrl"`
	Enabled     bool   `json:"enabled"`
	Order       int    `json:"order"`
}
type platformSetting struct {
	ID    string     `json:"id"`
	Items []Platform `json:"items"`
}
type Upload struct {
	ID           string `json:"id"`
	Filename     string `json:"filename"`
	MIME         string `json:"mime"`
	Size         int64  `json:"size"`
	OriginalName string `json:"original_name"`
	CreatedAt    int64  `json:"created_at"`
}
type admin struct {
	ID       int `gorm:"primaryKey;autoIncrement:false"`
	Username string
	Password string
}

func (admin) TableName() string { return "admins" }

type session struct {
	Hash    string `gorm:"primaryKey"`
	CSRF    string
	Expires int64
}

func (session) TableName() string { return "sessions" }

func (Upload) TableName() string { return "uploads" }

type Snapshot struct {
	Cases      []Case     `json:"cases"`
	Categories []Category `json:"categories"`
	Clients    []Client   `json:"clients"`
	Company    Company    `json:"company"`
	Platforms  []Platform `json:"platforms"`
	Uploads    []Upload   `json:"uploads"`
}
