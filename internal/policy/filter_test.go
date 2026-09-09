package policy

import (
	"testing"

	"school_district_reading/internal/domain"
)

func TestGradeOK(t *testing.T) {
	book := domain.Book{GradeMin: 4, GradeMax: 8}
	if !gradeOK(4, book, false) {
		t.Fatal("grade 4 in 4-8")
	}
	if gradeOK(3, book, false) {
		t.Fatal("grade 3 should fail without stretch")
	}
	if !gradeOK(3, book, true) {
		t.Fatal("stretch allows grade+1 on min")
	}
}
