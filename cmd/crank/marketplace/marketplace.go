// Package marketplace contains commands for interacting with the Upbound
// Marketplace.
package marketplace

// Cmd contains commands for interacting with the Upbound Marketplace.
type Cmd struct {
	Login  loginCmd  `cmd:"" help:"Login to the Upbound Marketplace."`
	Logout logoutCmd `cmd:"" help:"Logout of the Upbound Marketplace."`
}
