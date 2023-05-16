package request

import "encoding/json"

type CreateRequest struct {
	Type  string     `json:"type"`
	State DriveState `json:"state"`
}

func (r CreateRequest) ToJSON() []byte {
	buf, _ := json.Marshal(r)
	return buf
}
