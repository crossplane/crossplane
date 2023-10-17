package marketplace

type MarketplaceCmd struct {
	Login loginCmd `cmd:"" help:"Login to the Upbound Marketplace."`
	//Logout logoutCmd `cmd:"" help:"Logout of the Upbound Marketplace."`
}
