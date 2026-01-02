package ui

import (
	"time"

	"github.com/briandowns/spinner"
)

// Spinner wraps the briandowns/spinner for consistent styling.
type Spinner struct {
	s *spinner.Spinner
}

// NewSpinner creates a new spinner with the given message.
func NewSpinner(message string) *Spinner {
	s := spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	s.Suffix = " " + message
	return &Spinner{s: s}
}

// Start begins the spinner animation.
func (sp *Spinner) Start() {
	sp.s.Start()
}

// Stop stops the spinner.
func (sp *Spinner) Stop() {
	sp.s.Stop()
}

// Success stops the spinner and shows a success message.
func (sp *Spinner) Success(message string) {
	sp.s.Stop()
	Success("%s", message)
}

// Error stops the spinner and shows an error message.
func (sp *Spinner) Error(message string) {
	sp.s.Stop()
	Error("%s", message)
}

// UpdateMessage changes the spinner's message.
func (sp *Spinner) UpdateMessage(message string) {
	sp.s.Suffix = " " + message
}

// WithSpinner runs a function with a spinner, showing success or error on completion.
func WithSpinner(message string, fn func() error) error {
	sp := NewSpinner(message)
	sp.Start()

	err := fn()
	if err != nil {
		sp.Error(message + " - failed")
		return err
	}

	sp.Success(message)
	return nil
}
