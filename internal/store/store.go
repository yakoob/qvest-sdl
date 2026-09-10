package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"school_district_reading/internal/domain"
)

type Store struct {
	Dir           string
	District      map[string]any
	Books         []domain.Book
	BookByID      map[string]domain.Book
	Students      []domain.Student
	StudentByID   map[string]domain.Student
	Circulation   []domain.CirculationEvent
	History       map[string][]domain.CirculationEvent
	Librarians    []domain.Librarian
	LibrarianByID map[string]domain.Librarian
}

func Load(dir string) (*Store, error) {
	s := &Store{
		Dir:           dir,
		BookByID:      map[string]domain.Book{},
		StudentByID:   map[string]domain.Student{},
		History:       map[string][]domain.CirculationEvent{},
		LibrarianByID: map[string]domain.Librarian{},
	}
	if err := readJSON(filepath.Join(dir, "district.json"), &s.District); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, "catalog.json"), &s.Books); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, "students.json"), &s.Students); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, "circulation.json"), &s.Circulation); err != nil {
		return nil, err
	}
	if err := readJSON(filepath.Join(dir, "librarians.json"), &s.Librarians); err != nil {
		return nil, err
	}
	for _, b := range s.Books {
		s.BookByID[b.BookID] = b
	}
	for _, st := range s.Students {
		s.StudentByID[st.StudentID] = st
	}
	for _, ev := range s.Circulation {
		s.History[ev.StudentID] = append(s.History[ev.StudentID], ev)
	}
	for _, lib := range s.Librarians {
		s.LibrarianByID[lib.StaffID] = lib
	}
	return s, nil
}

func readJSON(path string, dest any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("json %s: %w", path, err)
	}
	return nil
}

func (s *Store) Student(id string) (domain.Student, bool) {
	st, ok := s.StudentByID[id]
	return st, ok
}

func (s *Store) Book(id string) (domain.Book, bool) {
	b, ok := s.BookByID[id]
	return b, ok
}

// CloneCirculation copies Books, BookByID, Circulation, and History so a
// session mutation can publish a new snapshot without mutating slices or maps
// still held by readers. Catalog Subjects remain shared (read-only). Students
// and librarians stay the same pointers; they are never mutated after Load.
func (s *Store) CloneCirculation() *Store {
	if s == nil {
		return nil
	}
	out := &Store{
		Dir:           s.Dir,
		District:      s.District,
		Books:         make([]domain.Book, len(s.Books)),
		BookByID:      make(map[string]domain.Book, len(s.BookByID)),
		Students:      s.Students,
		StudentByID:   s.StudentByID,
		Circulation:   make([]domain.CirculationEvent, len(s.Circulation)),
		History:       make(map[string][]domain.CirculationEvent, len(s.History)),
		Librarians:    s.Librarians,
		LibrarianByID: s.LibrarianByID,
	}
	copy(out.Books, s.Books)
	copy(out.Circulation, s.Circulation)
	out.reindexCirculation()
	return out
}

// reindexCirculation rebuilds BookByID and History from Books and Circulation.
// Callers must own the destination Store; shared student/librarian maps are
// left untouched.
func (s *Store) reindexCirculation() {
	s.BookByID = make(map[string]domain.Book, len(s.Books))
	for _, b := range s.Books {
		s.BookByID[b.BookID] = b
	}
	s.History = make(map[string][]domain.CirculationEvent, len(s.History))
	for _, ev := range s.Circulation {
		s.History[ev.StudentID] = append(s.History[ev.StudentID], ev)
	}
}

// ApplyCirculation replaces Books and Circulation, then rebuilds indexes.
// The receiver must not be shared with concurrent readers.
func (s *Store) ApplyCirculation(books []domain.Book, events []domain.CirculationEvent) {
	s.Books = books
	s.Circulation = events
	s.reindexCirculation()
}
