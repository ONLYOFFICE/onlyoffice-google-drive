package command

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/ONLYOFFICE/onlyoffice-gdrive/services/shared/request"
)

type EditCommand struct {
}

func NewEditCommand() Command {
	return &EditCommand{}
}

func (c *EditCommand) Execute(rw http.ResponseWriter, r *http.Request, state *request.DriveState) {
	state.ForceEdit = true
	http.Redirect(
		rw, r,
		fmt.Sprintf("/api/editor?state=%s", url.QueryEscape(string(state.ToJSON()))),
		http.StatusMovedPermanently,
	)
}
