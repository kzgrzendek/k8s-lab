package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProgressTracker(t *testing.T) {
	// Test basic progress tracking
	p := NewProgressTracker(10)

	assert.Equal(t, 0, p.Percentage(), "Initial percentage should be 0")
	assert.Equal(t, 0, p.current, "Initial current should be 0")
	assert.Equal(t, 10, p.total, "Total should be 10")

	// Test increment
	p.Increment()
	assert.Equal(t, 1, p.current, "Current should be 1 after increment")
	assert.Equal(t, 10, p.Percentage(), "Percentage should be 10%")

	// Test update
	p.Update(5)
	assert.Equal(t, 5, p.current, "Current should be 5 after update")
	assert.Equal(t, 50, p.Percentage(), "Percentage should be 50%")

	// Test max
	p.Update(10)
	assert.Equal(t, 100, p.Percentage(), "Percentage should be 100% when complete")

	// Test render doesn't panic
	assert.NotEmpty(t, p.Render(), "Render should return non-empty string")
}

func TestProgressTrackerEdgeCases(t *testing.T) {
	// Test zero total
	p := NewProgressTracker(0)
	assert.Equal(t, 0, p.Percentage(), "Percentage should be 0 for zero total")

	// Test increment beyond total
	p2 := NewProgressTracker(5)
	for i := 0; i < 10; i++ {
		p2.Increment()
	}
	assert.Equal(t, 5, p2.current, "Current should not exceed total")

	// Test negative update
	p3 := NewProgressTracker(10)
	p3.Update(-1)
	assert.Equal(t, 0, p3.current, "Current should stay at 0 for negative update")

	// Test update beyond total
	p4 := NewProgressTracker(10)
	p4.Update(20)
	assert.Equal(t, 0, p4.current, "Current should not update beyond total")
}

func TestStepProgress(t *testing.T) {
	steps := []string{"Step 1", "Step 2", "Step 3"}
	sp := NewStepProgress(steps)

	assert.Equal(t, 3, len(sp.steps), "Should have 3 steps")
	assert.Equal(t, 0, sp.current, "Initial current should be 0")

	// Test that methods don't panic
	sp.StartStep(0)
	sp.CompleteStep(0)
	sp.StartStep(1)
	sp.CompleteStep(1)
	sp.Complete()
}

func TestStepProgressEdgeCases(t *testing.T) {
	steps := []string{"Step 1", "Step 2"}
	sp := NewStepProgress(steps)

	// Test invalid step index
	sp.StartStep(-1)  // Should not panic
	sp.StartStep(100) // Should not panic
	sp.CompleteStep(-1)
	sp.CompleteStep(100)
	sp.FailStep(-1, nil)
	sp.FailStep(100, nil)
}
