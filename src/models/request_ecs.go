package models

type BindResourceToStreeNodeRequest struct {
	NodeId      int      `json:"node_id"`
	ResourceIds []string `json:"resource_ids" validate:"required,min=1"`
}
