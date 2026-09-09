package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"school_district_reading/internal/domain"
)

type Store struct {
	Dir          string
	District     map[string]any
	Books        []domain.Book
	BookByID     map[string]domain.Book
	Students     []domain.Student
	StudentByID  map[string]domain.Student
	Circulation  []domain.CirculationEvent
	History      map[string][]domain.CirculationEvent
	Librarians   []domain.Librarian
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
