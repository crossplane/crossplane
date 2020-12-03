package version

import (
	"github.com/Masterminds/semver"
)

var version string

// Operations provides semantic version operations.
type Operations interface {
	GetVersionString() string
	GetSemVer() (*semver.Version, error)
	InConstraints(c string) (bool, error)
}

// Versioner provides semantic version operations.
type Versioner struct {
	version string
}

// New creates a new versioner.
func New() *Versioner {
	return &Versioner{
		version: version,
	}
}

// GetVersionString returns the current Crossplane version as string.
func (v *Versioner) GetVersionString() string {
	return v.version
}

// GetSemVer returns the current Crossplane version as a semantic version.
func (v *Versioner) GetSemVer() (*semver.Version, error) {
	return semver.NewVersion(v.version)
}

// InConstraints is a helper function that checks if the current Crossplane
// version is in the semantic version constraints.
func (v *Versioner) InConstraints(c string) (bool, error) {
	ver, err := v.GetSemVer()
	if err != nil {
		return false, err
	}
	constraint, err := semver.NewConstraint(c)
	if err != nil {
		return false, err
	}
	return constraint.Check(ver), nil
}
