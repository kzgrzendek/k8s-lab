// Package errors provides structured error types for NOVA.
// This package defines custom error types that can be used for:
// - Type checking with errors.As()
// - Better error messages with context
// - Distinguishing between different failure categories
package errors

import (
	"errors"
	"fmt"
)

// --- Error Categories ---

// NotFoundError indicates that a required resource was not found.
// This could be a file, binary, container, pod, etc.
type NotFoundError struct {
	Resource string // What was not found (e.g., "kubectl binary", "config file")
	Path     string // Optional: where we looked
	Err      error  // Optional: underlying error
}

func (e *NotFoundError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s not found at %s", e.Resource, e.Path)
	}
	return fmt.Sprintf("%s not found", e.Resource)
}

func (e *NotFoundError) Unwrap() error {
	return e.Err
}

// NotAvailableError indicates that a required tool or service is not available.
// This is similar to NotFoundError but specifically for executables and services.
type NotAvailableError struct {
	Tool    string // Name of the tool (e.g., "kubectl", "docker", "minikube")
	Message string // Optional: additional context or installation instructions
	Err     error  // Optional: underlying error
}

func (e *NotAvailableError) Error() string {
	msg := fmt.Sprintf("%s not available", e.Tool)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

func (e *NotAvailableError) Unwrap() error {
	return e.Err
}

// AlreadyExistsError indicates that a resource already exists.
// This is typically not a fatal error but useful for idempotent operations.
type AlreadyExistsError struct {
	Resource string // What already exists (e.g., "namespace", "container")
	Name     string // Name of the resource
	Err      error  // Optional: underlying error
}

func (e *AlreadyExistsError) Error() string {
	return fmt.Sprintf("%s %s already exists", e.Resource, e.Name)
}

func (e *AlreadyExistsError) Unwrap() error {
	return e.Err
}

// NotRunningError indicates that a required service or cluster is not running.
type NotRunningError struct {
	Service string // Name of the service (e.g., "minikube", "docker daemon")
	Message string // Optional: additional context
	Err     error  // Optional: underlying error
}

func (e *NotRunningError) Error() string {
	msg := fmt.Sprintf("%s is not running", e.Service)
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

func (e *NotRunningError) Unwrap() error {
	return e.Err
}

// ValidationError indicates that input validation failed.
type ValidationError struct {
	Field   string // Field or parameter that failed validation
	Value   string // The invalid value
	Message string // Why it's invalid
	Err     error  // Optional: underlying error
}

func (e *ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("validation failed for %s=%s: %s", e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

// DeploymentError indicates a failure during deployment of a component.
type DeploymentError struct {
	Component string // Component being deployed (e.g., "Cilium", "Falco")
	Tier      string // Optional: tier number (e.g., "1", "2")
	Message   string // Error message
	Err       error  // Underlying error
}

func (e *DeploymentError) Error() string {
	msg := fmt.Sprintf("failed to deploy %s", e.Component)
	if e.Tier != "" {
		msg = fmt.Sprintf("failed to deploy %s (tier %s)", e.Component, e.Tier)
	}
	if e.Message != "" {
		msg += ": " + e.Message
	}
	return msg
}

func (e *DeploymentError) Unwrap() error {
	return e.Err
}

// ConfigurationError indicates a problem with configuration.
type ConfigurationError struct {
	Field   string // Configuration field with the issue
	Message string // Error message
	Err     error  // Optional: underlying error
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("configuration error in %s: %s", e.Field, e.Message)
}

func (e *ConfigurationError) Unwrap() error {
	return e.Err
}

// --- Convenience Constructors ---

// NewNotFound creates a new NotFoundError.
func NewNotFound(resource string) error {
	return &NotFoundError{Resource: resource}
}

// NewNotFoundAt creates a new NotFoundError with a path.
func NewNotFoundAt(resource, path string) error {
	return &NotFoundError{Resource: resource, Path: path}
}

// NewNotAvailable creates a new NotAvailableError.
func NewNotAvailable(tool string) error {
	return &NotAvailableError{Tool: tool}
}

// NewNotAvailableWithMessage creates a new NotAvailableError with a message.
func NewNotAvailableWithMessage(tool, message string) error {
	return &NotAvailableError{Tool: tool, Message: message}
}

// NewAlreadyExists creates a new AlreadyExistsError.
func NewAlreadyExists(resource, name string) error {
	return &AlreadyExistsError{Resource: resource, Name: name}
}

// NewNotRunning creates a new NotRunningError.
func NewNotRunning(service string) error {
	return &NotRunningError{Service: service}
}

// NewNotRunningWithMessage creates a new NotRunningError with a message.
func NewNotRunningWithMessage(service, message string) error {
	return &NotRunningError{Service: service, Message: message}
}

// NewValidation creates a new ValidationError.
func NewValidation(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}

// NewValidationWithValue creates a new ValidationError with a value.
func NewValidationWithValue(field, value, message string) error {
	return &ValidationError{Field: field, Value: value, Message: message}
}

// NewDeployment creates a new DeploymentError.
func NewDeployment(component string, err error) error {
	return &DeploymentError{Component: component, Err: err}
}

// NewDeploymentWithTier creates a new DeploymentError with tier information.
func NewDeploymentWithTier(component, tier string, err error) error {
	return &DeploymentError{Component: component, Tier: tier, Err: err}
}

// NewConfiguration creates a new ConfigurationError.
func NewConfiguration(field, message string) error {
	return &ConfigurationError{Field: field, Message: message}
}

// --- Error Checking Helpers ---

// IsNotFound checks if an error is a NotFoundError.
func IsNotFound(err error) bool {
	var notFoundErr *NotFoundError
	return errors.As(err, &notFoundErr)
}

// IsNotAvailable checks if an error is a NotAvailableError.
func IsNotAvailable(err error) bool {
	var notAvailErr *NotAvailableError
	return errors.As(err, &notAvailErr)
}

// IsAlreadyExists checks if an error is an AlreadyExistsError.
func IsAlreadyExists(err error) bool {
	var existsErr *AlreadyExistsError
	return errors.As(err, &existsErr)
}

// IsNotRunning checks if an error is a NotRunningError.
func IsNotRunning(err error) bool {
	var notRunningErr *NotRunningError
	return errors.As(err, &notRunningErr)
}

// IsValidation checks if an error is a ValidationError.
func IsValidation(err error) bool {
	var validErr *ValidationError
	return errors.As(err, &validErr)
}

// IsDeployment checks if an error is a DeploymentError.
func IsDeployment(err error) bool {
	var deployErr *DeploymentError
	return errors.As(err, &deployErr)
}

// IsConfiguration checks if an error is a ConfigurationError.
func IsConfiguration(err error) bool {
	var configErr *ConfigurationError
	return errors.As(err, &configErr)
}
