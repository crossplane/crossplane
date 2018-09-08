package clients

import (
	"net/http"

	googleapi "google.golang.org/api/googleapi"
)

func IsGoogleAPINotFound(err error) bool {
	if err == nil {
		return false
	}
	googleapiErr, ok := err.(*googleapi.Error)
	return ok && googleapiErr.Code == http.StatusNotFound
}
