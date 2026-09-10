package domain

// Book is a closed-world catalog title. Recommendations may only emit BookID values
// that exist in the loaded catalog.
type Book struct {
	BookID          string   `json:"book_id"`
	ISBN            string   `json:"isbn"`
	Title           string   `json:"title"`
	Author          string   `json:"author"`
	Year            int      `json:"year"`
	Pages           int      `json:"pages"`
	LexileApprox    int      `json:"lexile_approx"`
	GradeMin        int      `json:"grade_min"`
	GradeMax        int      `json:"grade_max"`
	Genre           string   `json:"genre"`
	Cluster         string   `json:"cluster"`
	Series          string   `json:"series"`
	Subjects        []string `json:"subjects"`
	CopiesTotal     int      `json:"copies_total"`
	CopiesAvailable int      `json:"copies_available"`
	Blurb           string   `json:"blurb"`
}

type Student struct {
	StudentID      string `json:"student_id"`
	Grade          int    `json:"grade"`
	HomeroomID     string `json:"homeroom_id"`
	ReadingBand    string `json:"reading_band"`
	LexileApprox   int    `json:"lexile_approx"`
	Cluster        string `json:"cluster"`
	PageComfort    string `json:"page_comfort"`
	EnglishLearner bool   `json:"english_learner"`
	NewThisYear    bool   `json:"new_this_year"`
	FirstName      string `json:"first_name"`
	LastInitial    string `json:"last_initial"`
	DemoRole       string `json:"demo_role"`
	Anecdote       string `json:"anecdote"`
}

type CirculationEvent struct {
	EventID      string `json:"event_id"`
	StudentID    string `json:"student_id"`
	BookID       string `json:"book_id"`
	CheckoutDate string `json:"checkout_date"`
	DueDate      string `json:"due_date"`
	ReturnDate   string `json:"return_date"`
	StaffID      string `json:"staff_id"`
	Channel      string `json:"channel"`
}

type Librarian struct {
	StaffID      string   `json:"staff_id"`
	FullName     string   `json:"full_name"`
	Role         string   `json:"role"`
	SiteID       string   `json:"site_id"`
	SiteName     string   `json:"site_name"`
	FTE          float64  `json:"fte"`
	Email        string   `json:"email"`
	GradesServed string   `json:"grades_served"`
	Specialties  []string `json:"specialties"`
	TypicalDays  string   `json:"typical_days"`
	StartLocal   string   `json:"start_local"`
	EndLocal     string   `json:"end_local"`
	PilotRole    string   `json:"pilot_role"`
	Notes        string   `json:"notes"`
}

type Request struct {
	StudentID string
	StaffID   string
	Query     string
	Stretch   bool
	Limit     int
}

type ScoredBook struct {
	Book    Book
	Score   float64
	CF      float64
	Content float64
	Bonus   float64
	Reasons []string
}

const (
	ExplainTemplate = "template"
	ExplainLive     = "live"
	ExplainFallback = "fallback"
)

// QueryInterpretation is the locally parsed librarian query. Raw text stays on
// the desk; it is not a model payload.
type QueryInterpretation struct {
	Under150 bool   `json:"under_150"`
	Short    bool   `json:"short"`
	Raw      string `json:"raw,omitempty"`
}

type Recommendation struct {
	StudentID     string              `json:"student_id"`
	StaffID       string              `json:"staff_id"`
	Query         string              `json:"query,omitempty"`
	QueryParsed   QueryInterpretation `json:"query_parsed"`
	Stretch       bool                `json:"stretch"`
	LLMEnabled    bool                `json:"llm_enabled"`
	ExplainMode   string              `json:"explain_mode"`
	ExplainNote   string              `json:"explain_note,omitempty"`
	Version       string              `json:"version,omitempty"`
	Items         []RecItem           `json:"items"`
	Dropped       []Dropped           `json:"dropped,omitempty"`
	TalkingPoints []string            `json:"talking_points"`
	AuditError    string              `json:"audit_error,omitempty"`
}

type RecItem struct {
	BookID          string   `json:"book_id"`
	Title           string   `json:"title"`
	Author          string   `json:"author"`
	Cluster         string   `json:"cluster"`
	Series          string   `json:"series"`
	Pages           int      `json:"pages"`
	CopiesAvailable int      `json:"copies_available"`
	Score           float64  `json:"score"`
	Reasons         []string `json:"reasons"`
	TalkingPoint    string   `json:"talking_point"`
}

type Dropped struct {
	BookID string `json:"book_id"`
	Title  string `json:"title"`
	Why    string `json:"why"`
}
