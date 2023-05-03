package command

import (
	"net/http"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
)

type Command interface {
	Execute(rw http.ResponseWriter, r *http.Request, state *request.DriveState)
}
