package command

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
)

type ViewCommand struct {
}

func NewViewCommand() Command {
	return &ViewCommand{}
}

func (c *ViewCommand) Execute(rw http.ResponseWriter, r *http.Request, state *request.DriveState) {
	http.Redirect(
		rw, r,
		fmt.Sprintf("/api/editor?state=%s", url.QueryEscape(string(state.ToJSON()))),
		http.StatusMovedPermanently,
	)
}
