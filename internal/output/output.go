package output

import "github.com/efan/proxyyopick/internal/model"

// Writer writes test results to a destination.
type Writer interface {
	Write(results []model.TestResult) error
}
