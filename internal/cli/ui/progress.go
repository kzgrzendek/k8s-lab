package ui

import (
	"fmt"
	"strings"
)

// ProgressTracker tracks progress for multi-step operations.
type ProgressTracker struct {
	total   int
	current int
	width   int
}

// NewProgressTracker creates a new progress tracker with the given total steps.
func NewProgressTracker(total int) *ProgressTracker {
	return &ProgressTracker{
		total:   total,
		current: 0,
		width:   40, // Width of the progress bar
	}
}

// Increment increments the progress by one step.
func (p *ProgressTracker) Increment() {
	if p.current < p.total {
		p.current++
	}
}

// Update updates the current progress to a specific step.
func (p *ProgressTracker) Update(current int) {
	if current >= 0 && current <= p.total {
		p.current = current
	}
}

// Percentage returns the current progress as a percentage (0-100).
func (p *ProgressTracker) Percentage() int {
	if p.total == 0 {
		return 0
	}
	return (p.current * 100) / p.total
}

// Render returns a visual representation of the progress bar.
func (p *ProgressTracker) Render() string {
	percentage := p.Percentage()
	filled := (p.width * p.current) / p.total

	if filled > p.width {
		filled = p.width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", p.width-filled)

	return fmt.Sprintf("[%s] %3d%% (%d/%d)",
		infoColor(bar),
		percentage,
		p.current,
		p.total)
}

// PrintProgress prints the current progress bar.
func (p *ProgressTracker) PrintProgress(message string) {
	if message != "" {
		fmt.Printf("\r%s %s", p.Render(), message)
	} else {
		fmt.Printf("\r%s", p.Render())
	}
}

// Complete marks the progress as complete and prints a final message.
func (p *ProgressTracker) Complete(message string) {
	p.current = p.total
	fmt.Printf("\r%s %s\n", p.Render(), successColor("✓ "+message))
}

// StepProgress represents a step-based progress indicator for named steps.
type StepProgress struct {
	steps   []string
	current int
	width   int
}

// NewStepProgress creates a new step-based progress indicator.
func NewStepProgress(steps []string) *StepProgress {
	return &StepProgress{
		steps:   steps,
		current: 0,
		width:   30, // Width of the progress bar
	}
}

// StartStep starts a new step and prints its status.
func (sp *StepProgress) StartStep(stepIndex int) {
	if stepIndex >= 0 && stepIndex < len(sp.steps) {
		sp.current = stepIndex

		// Calculate progress bar
		filled := (sp.width * (stepIndex + 1)) / len(sp.steps)
		if filled > sp.width {
			filled = sp.width
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", sp.width-filled)

		fmt.Printf("\n%s [%s] [%d/%d] %s...\n",
			stepColor("→"),
			infoColor(bar),
			stepIndex+1,
			len(sp.steps),
			sp.steps[stepIndex])
	}
}

// CompleteStep marks the current step as complete.
func (sp *StepProgress) CompleteStep(stepIndex int) {
	if stepIndex >= 0 && stepIndex < len(sp.steps) {
		fmt.Printf("%s Step %d/%d complete: %s\n",
			successColor("✓"),
			stepIndex+1,
			len(sp.steps),
			sp.steps[stepIndex])
	}
}

// FailStep marks the current step as failed.
func (sp *StepProgress) FailStep(stepIndex int, err error) {
	if stepIndex >= 0 && stepIndex < len(sp.steps) {
		fmt.Printf("%s Step %d/%d failed: %s - %v\n",
			errorColor("✗"),
			stepIndex+1,
			len(sp.steps),
			sp.steps[stepIndex],
			err)
	}
}

// Complete prints the final completion message.
func (sp *StepProgress) Complete() {
	// Show completed progress bar
	bar := strings.Repeat("█", sp.width)
	fmt.Printf("\n%s [%s] All steps complete! (%d/%d)\n\n",
		successColor("✓"),
		successColor(bar),
		len(sp.steps),
		len(sp.steps))
}
