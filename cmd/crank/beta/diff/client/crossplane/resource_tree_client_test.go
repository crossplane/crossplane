package crossplane

// This file intentionally left blank.  The ResourceTreeClient is a thin shim around the xrm.Client struct.  We
// went to construct an interface around the xrm.Client in order to test this class, but it turned out to be the
// same interface that this one provides.  No real logic in ResourceTreeClient except for converting to unstructured
// and logging.
