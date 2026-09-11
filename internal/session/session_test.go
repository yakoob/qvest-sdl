package session

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"school_district_reading/internal/academics"
	"school_district_reading/internal/domain"
	"school_district_reading/internal/engine"
	"school_district_reading/internal/explain"
	"school_district_reading/internal/store"
	"school_district_reading/internal/support"
)

func loadEngine(t *testing.T) *engine.Engine {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	st, err := store.Load(filepath.Join(root, "data", "json"))
	if err != nil {
		t.Fatal(err)
	}
	eng := engine.New(st)
	eng.Explain = explain.TemplateExplainer{}
	eng.LLMOn = false
	eng.Audit = nil
	return eng
}

func TestCheckoutDecrementsInventoryAndAddsLoan(t *testing.T) {
	svc := New(loadEngine(t))
	svc.SetClock(func() time.Time { return time.Date(2026, 9, 9, 15, 0, 0, 0, time.UTC) })
	before := svc.Snapshot().Engine.Store.BookByID["B-007"].CopiesAvailable
	out, err := svc.Checkout(CheckoutRequest{StudentID: "S-406", BookID: "B-007", StaffID: "L-002", RetryID: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Loan.LoanID == "" || !out.Loan.Open {
		t.Fatalf("loan %+v", out.Loan)
	}
	if out.CopiesAfter != before-1 {
		t.Fatalf("copies %d want %d", out.CopiesAfter, before-1)
	}
	open, _ := svc.LoansFor("S-406")
	found := false
	for _, l := range open {
		if l.BookID == "B-007" {
			found = true
		}
	}
	if !found {
		t.Fatal("Mateo should now have Cat Kid as a current loan")
	}
	again, err := svc.Checkout(CheckoutRequest{StudentID: "S-406", BookID: "B-007", StaffID: "L-002", RetryID: "r1"})
	if err != nil {
		t.Fatal(err)
	}
	if !again.Idempotent || again.Loan.LoanID != out.Loan.LoanID {
		t.Fatalf("retry should replay loan, got %+v", again)
	}
	if svc.Snapshot().Engine.Store.BookByID["B-007"].CopiesAvailable != out.CopiesAfter {
		t.Fatal("retry consumed another copy")
	}
}

func TestDuplicateActiveLoanRejected(t *testing.T) {
	svc := New(loadEngine(t))
	if _, err := svc.Checkout(CheckoutRequest{StudentID: "S-406", BookID: "B-007", StaffID: "L-001"}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.Checkout(CheckoutRequest{StudentID: "S-406", BookID: "B-007", StaffID: "L-001"})
	if !errors.Is(err, ErrDuplicateLoan) {
		t.Fatalf("want duplicate, got %v", err)
	}
}

func TestZeroCopiesRejected(t *testing.T) {
	svc := New(loadEngine(t))
	_, err := svc.Checkout(CheckoutRequest{StudentID: "S-406", BookID: "B-008", StaffID: "L-001"})
	if !errors.Is(err, ErrZeroCopies) {
		t.Fatalf("want zero copies, got %v", err)
	}
}

func TestUnknownIDs(t *testing.T) {
	svc := New(loadEngine(t))
	if _, err := svc.Checkout(CheckoutRequest{StudentID: "S-NOPE", BookID: "B-007", StaffID: "L-001"}); !errors.Is(err, ErrUnknownStudent) {
		t.Fatalf("student %v", err)
	}
	if _, err := svc.Checkout(CheckoutRequest{StudentID: "S-406", BookID: "B-NOPE", StaffID: "L-001"}); !errors.Is(err, ErrUnknownBook) {
		t.Fatalf("book %v", err)
	}
	if _, err := svc.Checkout(CheckoutRequest{StudentID: "S-406", BookID: "B-007", StaffID: "L-999"}); !errors.Is(err, ErrUnknownStaff) {
		t.Fatalf("staff %v", err)
	}
	if _, err := svc.Checkout(CheckoutRequest{StudentID: "S-406", BookID: "B-007"}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("missing staff %v", err)
	}
}

func TestReturnExactOnceAndRestoresCopy(t *testing.T) {
	svc := New(loadEngine(t))
	out, err := svc.Checkout(CheckoutRequest{StudentID: "S-509", BookID: "B-007", StaffID: "L-001", RetryID: "c1"})
	if err != nil {
		t.Fatal(err)
	}
	mid := svc.Snapshot().Engine.Store.BookByID["B-007"].CopiesAvailable
	ret, err := svc.Return(ReturnRequest{StudentID: "S-509", LoanID: out.Loan.LoanID, StaffID: "L-001", RetryID: "ret1"})
	if err != nil {
		t.Fatal(err)
	}
	if ret.Loan.Open {
		t.Fatal("returned loan still open")
	}
	if ret.CopiesAfter != mid+1 {
		t.Fatalf("copies after return %d", ret.CopiesAfter)
	}
	again, err := svc.Return(ReturnRequest{StudentID: "S-509", LoanID: out.Loan.LoanID, StaffID: "L-001", RetryID: "ret1"})
	if err != nil {
		t.Fatal(err)
	}
	if !again.Idempotent {
		t.Fatal("return retry should be idempotent")
	}
	_, err = svc.Return(ReturnRequest{StudentID: "S-509", LoanID: out.Loan.LoanID, StaffID: "L-001", RetryID: "ret-other"})
	if !errors.Is(err, ErrAlreadyReturned) {
		t.Fatalf("second return %v", err)
	}
	open, hist := svc.LoansFor("S-509")
	if len(open) != 0 {
		t.Fatalf("open %v", open)
	}
	if len(hist) == 0 || hist[0].BookID != "B-007" {
		t.Fatalf("history should keep returned title, got %v", hist)
	}
}

func TestReturnWouldExceedTotal(t *testing.T) {
	eng := loadEngine(t)
	st := eng.Store.CloneCirculation()
	books := append([]domain.Book(nil), st.Books...)
	for i := range books {
		if books[i].BookID == "B-043" {
			books[i].CopiesAvailable = books[i].CopiesTotal
		}
	}
	events := append([]domain.CirculationEvent(nil), st.Circulation...)
	events = append(events, domain.CirculationEvent{
		EventID:      "C-FAKE-OPEN",
		StudentID:    "S-509",
		BookID:       "B-043",
		CheckoutDate: "2026-09-01",
		DueDate:      "2026-09-15",
		StaffID:      "L-001",
		Channel:      "desk",
	})
	st.ApplyCirculation(books, events)
	eng = eng.WithStore(st)
	svc := New(eng)
	_, err := svc.Return(ReturnRequest{StudentID: "S-509", LoanID: "C-FAKE-OPEN", StaffID: "L-001"})
	if !errors.Is(err, ErrInventoryConflict) {
		t.Fatalf("want inventory conflict, got %v", err)
	}
}

func TestCheckoutUpdatesRecommendationPolicy(t *testing.T) {
	svc := New(loadEngine(t))
	before, _, err := svc.Recommend(context.Background(), domain.Request{StudentID: "S-406", Limit: 8})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range before.Items {
		if it.BookID == "B-007" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Cat Kid before checkout, got %#v", idsOf(before))
	}
	if _, err := svc.Checkout(CheckoutRequest{StudentID: "S-406", BookID: "B-007", StaffID: "L-001"}); err != nil {
		t.Fatal(err)
	}
	after, _, err := svc.Recommend(context.Background(), domain.Request{StudentID: "S-406", Limit: 8})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range after.Items {
		if it.BookID == "B-007" {
			t.Fatal("checked-out title must drop from recommendations")
		}
	}
}

func TestConcurrentLastCopy(t *testing.T) {
	svc := New(loadEngine(t))
	// B-003 has 1 copy. Two students racing for it: only one wins.
	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	for _, sid := range []string{"S-509", "S-405"} {
		sid := sid
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.Checkout(CheckoutRequest{StudentID: sid, BookID: "B-003", StaffID: "L-001"})
			errCh <- err
		}()
	}
	wg.Wait()
	close(errCh)
	var ok, fail int
	for err := range errCh {
		if err == nil {
			ok++
			continue
		}
		if errors.Is(err, ErrZeroCopies) {
			fail++
			continue
		}
		t.Fatalf("unexpected %v", err)
	}
	if ok != 1 || fail != 1 {
		t.Fatalf("last copy race ok=%d fail=%d", ok, fail)
	}
	if svc.Snapshot().Engine.Store.BookByID["B-003"].CopiesAvailable != 0 {
		t.Fatal("last copy should be 0")
	}
}

func TestSessionDoesNotWriteAudit(t *testing.T) {
	eng := loadEngine(t)
	svc := New(eng)
	if svc.Snapshot().Engine.Audit != nil {
		t.Fatal("session engine must not carry disk audit")
	}
}

func TestReturnWrongStudent(t *testing.T) {
	svc := New(loadEngine(t))
	out, err := svc.Checkout(CheckoutRequest{StudentID: "S-509", BookID: "B-007", StaffID: "L-001"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.Return(ReturnRequest{StudentID: "S-406", LoanID: out.Loan.LoanID, StaffID: "L-001"})
	if !errors.Is(err, ErrLoanStudentMismatch) {
		t.Fatalf("got %v", err)
	}
}

func idsOf(rec domain.Recommendation) []string {
	out := make([]string, 0, len(rec.Items))
	for _, it := range rec.Items {
		out = append(out, it.BookID)
	}
	return out
}

func TestClassifiedBelowGradeRecommendStaysClosedWorld(t *testing.T) {
	eng := loadEngine(t)
	svc := New(eng)
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	dir := filepath.Join(root, "data", "json")
	cat, err := academics.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	sup, err := support.Load(dir, eng.Store)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetDeskContext(cat, sup)
	plain, err := eng.Recommend(domain.Request{StudentID: "S-504", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	classified, _, err := svc.Recommend(context.Background(), domain.Request{StudentID: "S-504", Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if classified.QueryParsed.Raw != "" && classified.QueryParsed.Raw == "secret" {
		t.Fatal("raw query leaked")
	}
	for _, it := range classified.Items {
		if _, ok := eng.Store.Book(it.BookID); !ok {
			t.Fatalf("unknown book %s", it.BookID)
		}
		if it.BookID == "B-008" {
			t.Fatal("zero-copy title recommended")
		}
	}
	if len(classified.Items) == 0 {
		t.Fatal("empty recs")
	}
	_ = plain
}
