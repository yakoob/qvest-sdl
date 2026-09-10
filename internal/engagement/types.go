package engagement

import "time"

// State is owned by session.Service's mutex, together with inventory.
type State struct {
	Revision     int64         `json:"revision"`
	Appointments []Appointment `json:"appointments"`
	Interactions []Interaction `json:"interactions"`
	Offers       []Offer       `json:"offers"`
	Choices      []Choice      `json:"choices"`
	Feedback     []Feedback    `json:"feedback"`
	Followups    []Followup    `json:"followups"`
	Events       []Event       `json:"events"`
}
type Appointment struct {
	ID        string    `json:"id"`
	StudentID string    `json:"student_id"`
	StaffID   string    `json:"staff_id"`
	Start     time.Time `json:"start"`
	End       time.Time `json:"end"`
	Place     string    `json:"place"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
type Interaction struct {
	ID            string     `json:"id"`
	StudentID     string     `json:"student_id"`
	AppointmentID string     `json:"appointment_id,omitempty"`
	Facilitator   string     `json:"facilitator"`
	StartedAt     time.Time  `json:"started_at"`
	CompletedAt   *time.Time `json:"completed_at,omitempty"`
}
type Offer struct {
	ID                string    `json:"id"`
	InteractionID     string    `json:"interaction_id"`
	BookIDs           []string  `json:"book_ids"`
	At                time.Time `json:"at"`
	InventoryRevision int64     `json:"inventory_revision"`
	EvidenceVersion   string    `json:"evidence_version"`
	Stretch           bool      `json:"stretch"`
	Under150          bool      `json:"under_150"`
	Short             bool      `json:"short"`
}
type Choice struct {
	ID               string     `json:"id"`
	InteractionID    string     `json:"interaction_id"`
	OfferID          string     `json:"offer_id,omitempty"`
	BookID           string     `json:"book_id,omitempty"`
	Source           string     `json:"source"`
	At               time.Time  `json:"at"`
	LoanID           string     `json:"loan_id,omitempty"`
	CheckoutAt       *time.Time `json:"checkout_at,omitempty"`
	CirculationStaff string     `json:"circulation_staff,omitempty"`
}
type Feedback struct {
	ID            string    `json:"id"`
	InteractionID string    `json:"interaction_id"`
	BookID        string    `json:"book_id"`
	LoanID        string    `json:"loan_id,omitempty"`
	Reading       string    `json:"reading"`
	Enjoyment     string    `json:"enjoyment"`
	Source        string    `json:"source"`
	StaffID       string    `json:"staff_id"`
	At            time.Time `json:"at"`
}
type Followup struct {
	AppointmentID string        `json:"appointment_id,omitempty"`
	ContactID     string        `json:"contact_id,omitempty"`
	Reservations  []Reservation `json:"reservations,omitempty"`
	ID            string        `json:"id"`
	InteractionID string        `json:"interaction_id"`
	Due           time.Time     `json:"due"`
	CreatedAt     time.Time     `json:"created_at"`
	CompletedAt   *time.Time    `json:"completed_at,omitempty"`
	StaffID       string        `json:"staff_id,omitempty"`
}

// Reservation preserves dated scheduling changes for as-of reports.
type Reservation struct {
	At            time.Time `json:"at"`
	AppointmentID string    `json:"appointment_id"`
	Due           time.Time `json:"due"`
	Status        string    `json:"status"`
}

type Event struct {
	ID       string    `json:"id"`
	Action   string    `json:"action"`
	EntityID string    `json:"entity_id"`
	At       time.Time `json:"at"`
	Revision int64     `json:"revision"`
}

// Command is bounded by the HTTP layer; all mutations require a request ID and revision.
type Command struct {
	RequestID        string `json:"request_id"`
	ExpectedRevision int64  `json:"expected_revision"`
	Action           string `json:"action"`
	ID               string `json:"id,omitempty"`
	StudentID        string `json:"student_id,omitempty"`
	StaffID          string `json:"staff_id,omitempty"`
	AppointmentID    string `json:"appointment_id,omitempty"`
	InteractionID    string `json:"interaction_id,omitempty"`
	OfferID          string `json:"offer_id,omitempty"`
	BookID           string `json:"book_id,omitempty"`
	Source           string `json:"source,omitempty"`
	Start            string `json:"start,omitempty"`
	Duration         int    `json:"duration,omitempty"`
	Place            string `json:"place,omitempty"`
	Confirmed        bool   `json:"confirmed,omitempty"`
	Due              string `json:"due,omitempty"`
	Reading          string `json:"reading,omitempty"`
	Enjoyment        string `json:"enjoyment,omitempty"`
	Query            string `json:"query,omitempty"`
	Stretch          bool   `json:"stretch,omitempty"`
}
