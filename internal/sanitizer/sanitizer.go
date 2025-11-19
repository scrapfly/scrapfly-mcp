package sanitizer

import (
	"github.com/scrapfly/scrapfly-mcp/internal/sanitizer/advanced"
	"github.com/scrapfly/scrapfly-mcp/internal/sanitizer/advanced/recoverable"
	"github.com/scrapfly/scrapfly-mcp/internal/sanitizer/basic"
)

type SanitizerType int

const (
	SanitizerTypeBasic SanitizerType = iota
	SanitizerTypeAdvanced
	SanitizerTypeRecoverable
)

func SanitizeNils(obj interface{}, sanitizerType SanitizerType) {
	switch sanitizerType {
	case SanitizerTypeAdvanced:
		advanced.SanitizeNilsExtended(obj)
	case SanitizerTypeBasic:
		basic.SanitizeNils(obj)
	case SanitizerTypeRecoverable:
		recoverable.SanitizeNils(obj)
	default:
		basic.SanitizeNils(obj)
	}
}

func BasicSanitizeNils(obj interface{}) {
	basic.SanitizeNils(obj)
}

func AdvancedSanitizeNils(obj interface{}) {
	advanced.SanitizeNilsExtended(obj)
}

func RecoverableSanitizeNils(obj interface{}) {
	recoverable.SanitizeNils(obj)
}
