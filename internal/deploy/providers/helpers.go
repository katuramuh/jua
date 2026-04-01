package providers

import (
	"os"
	"time"
)

// noopProgress is a ProgressReporter that silently discards all output.
// Used internally when a provider calls Deploy recursively for rollback.
type noopProgress struct{}

func (n *noopProgress) Step(name string)                              {}
func (n *noopProgress) StepDone(name string, d time.Duration)        {}
func (n *noopProgress) StepFailed(name string, err error)            {}
func (n *noopProgress) SubStep(parent, name string, pct int, done bool) {}
func (n *noopProgress) Log(message string)                           {}
func (n *noopProgress) Success(message string)                         {}
func (n *noopProgress) Error(message string)                           {}
func (n *noopProgress) URL(label, url string)                          {}
func (n *noopProgress) Hint(message string)                            {}

// readFileBytes reads a file and returns its content.
func readFileBytes(path string) ([]byte, error) {
	return os.ReadFile(path)
}
