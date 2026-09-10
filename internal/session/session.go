package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"school_district_reading/internal/domain"
	"school_district_reading/internal/engagement"
	"school_district_reading/internal/engine"
	"school_district_reading/internal/store"
)

const (
	ActionCheckout = "checkout"
	ActionReturn   = "return"

	ProvenanceSource  = "source"
	ProvenanceSession = "session"

	maxActivity = 80
	maxRetry    = 64
	loanPrefix  = "SESS-"
	dueDays     = 14
)

var (
	ErrInvalidInput        = errors.New("invalid input")
	ErrUnknownStudent      = errors.New("unknown student")
	ErrUnknownBook         = errors.New("unknown book")
	ErrUnknownStaff        = errors.New("unknown staff")
	ErrUnknownLoan         = errors.New("unknown loan")
	ErrDuplicateLoan       = errors.New("student already has this title out")
	ErrZeroCopies          = errors.New("no copies on the shelf")
	ErrAlreadyReturned     = errors.New("loan already returned")
	ErrInventoryConflict   = errors.New("returning this loan would exceed copies_total")
	ErrLoanStudentMismatch = errors.New("loan does not belong to this student")
)

type Snapshot struct {
	Engine    *engine.Engine
	Revision  int64
	StartedAt time.Time
}

type LoanView struct {
	LoanID       string `json:"loan_id"`
	EventID      string `json:"event_id"`
	StudentID    string `json:"student_id"`
	BookID       string `json:"book_id"`
	Title        string `json:"title"`
	Author       string `json:"author,omitempty"`
	CheckoutDate string `json:"checkout_date"`
	DueDate      string `json:"due_date,omitempty"`
	ReturnDate   string `json:"return_date,omitempty"`
	StaffID      string `json:"staff_id,omitempty"`
	Channel      string `json:"channel,omitempty"`
	Provenance   string `json:"provenance"`
	Open         bool   `json:"open"`
}

type Activity struct {
	ID              string    `json:"id"`
	TS              time.Time `json:"ts"`
	Action          string    `json:"action"`
	StudentID       string    `json:"student_id"`
	BookID          string    `json:"book_id"`
	Title           string    `json:"title"`
	StaffID         string    `json:"staff_id"`
	LoanID          string    `json:"loan_id"`
	CopiesDelta     int       `json:"copies_delta"`
	CopiesAvailable int       `json:"copies_available"`
	CopiesTotal     int       `json:"copies_total"`
	Revision        int64     `json:"revision"`
	Provenance      string    `json:"provenance"`
}

type CheckoutRequest struct {
	StudentID string
	BookID    string
	StaffID   string
	RetryID   string
}

type ReturnRequest struct {
	StudentID string
	LoanID    string
	StaffID   string
	RetryID   string
}

type CheckoutResult struct {
	Loan        LoanView `json:"loan"`
	CopiesAfter int      `json:"copies_available"`
	CopiesTotal int      `json:"copies_total"`
	Revision    int64    `json:"revision"`
	Idempotent  bool     `json:"idempotent"`
	Activity    Activity `json:"activity"`
}

type ReturnResult struct {
	Loan        LoanView `json:"loan"`
	CopiesAfter int      `json:"copies_available"`
	CopiesTotal int      `json:"copies_total"`
	Revision    int64    `json:"revision"`
	Idempotent  bool     `json:"idempotent"`
	Activity    Activity `json:"activity"`
}

type Service struct {
	mu              sync.Mutex
	snap            Snapshot
	activity        []Activity
	checkoutRetry   map[string]CheckoutResult
	returnRetry     map[string]ReturnResult
	nextLoan        int64
	clock           func() time.Time
	engagement      engagement.State
	calendar        *engagement.Calendar
	engagementRetry map[string]engagementReceipt
}

func New(eng *engine.Engine) *Service {
	if eng != nil && eng.Audit != nil {
		clone := *eng
		clone.Audit = nil
		eng = &clone
	}
	now := time.Now()
	return &Service{
		snap: Snapshot{
			Engine:    eng,
			Revision:  1,
			StartedAt: now,
		},
		checkoutRetry: map[string]CheckoutResult{},
		returnRetry:   map[string]ReturnResult{},
		nextLoan:      1,
		clock:         time.Now,
	}
}

func (s *Service) SetClock(now func() time.Time) {
	if now == nil {
		return
	}
	s.mu.Lock()
	s.clock = now
	s.mu.Unlock()
}

func (s *Service) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snap
}

func (s *Service) Activity() []Activity {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Activity, len(s.activity))
	copy(out, s.activity)
	return out
}

func (s *Service) Recommend(ctx context.Context, req domain.Request) (domain.Recommendation, int64, error) {
	snap := s.Snapshot()
	rec, err := snap.Engine.RecommendContext(ctx, req)
	return rec, snap.Revision, err
}

func (s *Service) Checkout(req CheckoutRequest) (CheckoutResult, error) {
	req.StudentID = strings.TrimSpace(req.StudentID)
	req.BookID = strings.TrimSpace(req.BookID)
	req.StaffID = strings.TrimSpace(req.StaffID)
	req.RetryID = strings.TrimSpace(req.RetryID)
	if req.StudentID == "" || req.BookID == "" {
		return CheckoutResult{}, fmt.Errorf("%w: student_id and book_id required", ErrInvalidInput)
	}
	if req.StaffID == "" {
		return CheckoutResult{}, fmt.Errorf("%w: staff_id required", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkoutLocked(req)
}

// checkoutLocked is also used by engagement under the same inventory mutex.
func (s *Service) checkoutLocked(req CheckoutRequest) (CheckoutResult, error) {
	if req.RetryID != "" {
		if cached, ok := s.checkoutRetry[req.RetryID]; ok {
			cached.Idempotent = true
			return cached, nil
		}
	}

	st := s.snap.Engine.Store
	if _, ok := st.Student(req.StudentID); !ok {
		return CheckoutResult{}, ErrUnknownStudent
	}
	book, ok := st.Book(req.BookID)
	if !ok {
		return CheckoutResult{}, ErrUnknownBook
	}
	if _, ok := st.LibrarianByID[req.StaffID]; !ok {
		return CheckoutResult{}, ErrUnknownStaff
	}
	for _, ev := range st.History[req.StudentID] {
		if ev.BookID == req.BookID && isOpen(ev) {
			return CheckoutResult{}, ErrDuplicateLoan
		}
	}
	if book.CopiesAvailable <= 0 {
		return CheckoutResult{}, ErrZeroCopies
	}

	now := s.clock()
	loanID := fmt.Sprintf("%s%04d", loanPrefix, s.nextLoan)
	s.nextLoan++
	event := domain.CirculationEvent{
		EventID:      loanID,
		StudentID:    req.StudentID,
		BookID:       req.BookID,
		CheckoutDate: now.Format("2006-01-02"),
		DueDate:      now.Add(dueDays * 24 * time.Hour).Format("2006-01-02"),
		ReturnDate:   "",
		StaffID:      req.StaffID,
		Channel:      "desk-session",
	}

	next := st.CloneCirculation()
	books := append([]domain.Book(nil), next.Books...)
	copiesAfter := 0
	copiesTotal := book.CopiesTotal
	found := false
	for i := range books {
		if books[i].BookID != req.BookID {
			continue
		}
		if books[i].CopiesAvailable <= 0 {
			return CheckoutResult{}, ErrZeroCopies
		}
		books[i].CopiesAvailable--
		copiesAfter = books[i].CopiesAvailable
		copiesTotal = books[i].CopiesTotal
		found = true
		break
	}
	if !found {
		return CheckoutResult{}, ErrUnknownBook
	}
	events := append(append([]domain.CirculationEvent(nil), next.Circulation...), event)
	next.ApplyCirculation(books, events)
	s.publishLocked(next)

	loan := loanView(next, event)
	act := Activity{
		ID:              "A-" + loanID,
		TS:              now.UTC(),
		Action:          ActionCheckout,
		StudentID:       req.StudentID,
		BookID:          req.BookID,
		Title:           loan.Title,
		StaffID:         req.StaffID,
		LoanID:          loanID,
		CopiesDelta:     -1,
		CopiesAvailable: copiesAfter,
		CopiesTotal:     copiesTotal,
		Revision:        s.snap.Revision,
		Provenance:      ProvenanceSession,
	}
	s.pushActivityLocked(act)
	out := CheckoutResult{
		Loan:        loan,
		CopiesAfter: copiesAfter,
		CopiesTotal: copiesTotal,
		Revision:    s.snap.Revision,
		Activity:    act,
	}
	if req.RetryID != "" {
		s.rememberCheckoutLocked(req.RetryID, out)
	}
	return out, nil
}

func (s *Service) Return(req ReturnRequest) (ReturnResult, error) {
	req.StudentID = strings.TrimSpace(req.StudentID)
	req.LoanID = strings.TrimSpace(req.LoanID)
	req.StaffID = strings.TrimSpace(req.StaffID)
	req.RetryID = strings.TrimSpace(req.RetryID)
	if req.StudentID == "" || req.LoanID == "" {
		return ReturnResult{}, fmt.Errorf("%w: student_id and loan_id required", ErrInvalidInput)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if req.RetryID != "" {
		if cached, ok := s.returnRetry[req.RetryID]; ok {
			cached.Idempotent = true
			return cached, nil
		}
	}

	st := s.snap.Engine.Store
	if _, ok := st.Student(req.StudentID); !ok {
		return ReturnResult{}, ErrUnknownStudent
	}
	if req.StaffID != "" {
		if _, ok := st.LibrarianByID[req.StaffID]; !ok {
			return ReturnResult{}, ErrUnknownStaff
		}
	}

	idx := -1
	var ev domain.CirculationEvent
	for i, row := range st.Circulation {
		if row.EventID == req.LoanID {
			idx = i
			ev = row
			break
		}
	}
	if idx < 0 {
		return ReturnResult{}, ErrUnknownLoan
	}
	if ev.StudentID != req.StudentID {
		return ReturnResult{}, ErrLoanStudentMismatch
	}
	if !isOpen(ev) {
		return ReturnResult{}, ErrAlreadyReturned
	}
	book, ok := st.Book(ev.BookID)
	if !ok {
		return ReturnResult{}, ErrUnknownBook
	}
	if book.CopiesAvailable+1 > book.CopiesTotal {
		return ReturnResult{}, ErrInventoryConflict
	}

	now := s.clock()
	next := st.CloneCirculation()
	books := append([]domain.Book(nil), next.Books...)
	copiesAfter := 0
	copiesTotal := book.CopiesTotal
	for i := range books {
		if books[i].BookID != ev.BookID {
			continue
		}
		if books[i].CopiesAvailable+1 > books[i].CopiesTotal {
			return ReturnResult{}, ErrInventoryConflict
		}
		books[i].CopiesAvailable++
		copiesAfter = books[i].CopiesAvailable
		copiesTotal = books[i].CopiesTotal
		break
	}
	events := append([]domain.CirculationEvent(nil), next.Circulation...)
	events[idx].ReturnDate = now.Format("2006-01-02")
	if req.StaffID != "" {
		events[idx].StaffID = req.StaffID
	}
	closed := events[idx]
	next.ApplyCirculation(books, events)
	s.publishLocked(next)

	loan := loanView(next, closed)
	act := Activity{
		ID:              "A-RET-" + closed.EventID,
		TS:              now.UTC(),
		Action:          ActionReturn,
		StudentID:       req.StudentID,
		BookID:          closed.BookID,
		Title:           loan.Title,
		StaffID:         closed.StaffID,
		LoanID:          closed.EventID,
		CopiesDelta:     1,
		CopiesAvailable: copiesAfter,
		CopiesTotal:     copiesTotal,
		Revision:        s.snap.Revision,
		Provenance:      provenanceOf(closed.EventID),
	}
	s.pushActivityLocked(act)
	out := ReturnResult{
		Loan:        loan,
		CopiesAfter: copiesAfter,
		CopiesTotal: copiesTotal,
		Revision:    s.snap.Revision,
		Activity:    act,
	}
	if req.RetryID != "" {
		s.rememberReturnLocked(req.RetryID, out)
	}
	return out, nil
}

func (s *Service) LoansFor(studentID string) (open []LoanView, returned []LoanView) {
	snap := s.Snapshot()
	hist := snap.Engine.Store.History[studentID]
	for i := len(hist) - 1; i >= 0; i-- {
		view := loanView(snap.Engine.Store, hist[i])
		if view.Open {
			open = append(open, view)
		} else {
			returned = append(returned, view)
		}
	}
	return open, returned
}

func (s *Service) publishLocked(next *store.Store) {
	s.snap.Engine = s.snap.Engine.WithStore(next)
	s.snap.Revision++
}

func (s *Service) pushActivityLocked(act Activity) {
	s.activity = append([]Activity{act}, s.activity...)
	if len(s.activity) > maxActivity {
		s.activity = s.activity[:maxActivity]
	}
}

func (s *Service) rememberCheckoutLocked(id string, out CheckoutResult) {
	if len(s.checkoutRetry) >= maxRetry {
		s.checkoutRetry = map[string]CheckoutResult{}
	}
	s.checkoutRetry[id] = out
}

func (s *Service) rememberReturnLocked(id string, out ReturnResult) {
	if len(s.returnRetry) >= maxRetry {
		s.returnRetry = map[string]ReturnResult{}
	}
	s.returnRetry[id] = out
}

func isOpen(ev domain.CirculationEvent) bool {
	return strings.TrimSpace(ev.ReturnDate) == ""
}

func provenanceOf(eventID string) string {
	if strings.HasPrefix(eventID, loanPrefix) {
		return ProvenanceSession
	}
	return ProvenanceSource
}

func loanView(st *store.Store, ev domain.CirculationEvent) LoanView {
	title := ev.BookID
	author := ""
	if b, ok := st.Book(ev.BookID); ok {
		title = b.Title
		author = b.Author
	}
	return LoanView{
		LoanID:       ev.EventID,
		EventID:      ev.EventID,
		StudentID:    ev.StudentID,
		BookID:       ev.BookID,
		Title:        title,
		Author:       author,
		CheckoutDate: ev.CheckoutDate,
		DueDate:      ev.DueDate,
		ReturnDate:   ev.ReturnDate,
		StaffID:      ev.StaffID,
		Channel:      ev.Channel,
		Provenance:   provenanceOf(ev.EventID),
		Open:         isOpen(ev),
	}
}
