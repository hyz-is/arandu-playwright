//go:build e2e

package e2e_test

import (
	"testing"

	"github.com/arandu-io/hesape/arandutest/browser/conformance"

	playwright "github.com/hyz-is/arandu-playwright"
)

// TestConformance runs the suite every driver passes, in the engine
// ARANDU_E2E_BROWSER names.
func TestConformance(t *testing.T) {
	conformance.Run(t, playwright.Launch)
}
