package tag

type Tag struct {
	ID     string `bson:"_id,omitempty" json:"id"`
	UserID string `bson:"user_id" json:"user_id"`
	Name   string `bson:"name" json:"name"`
}

type CreateTagRequest struct {
	Name string `json:"name" binding:"required"`
}

type UpdateTagRequest struct {
	Name string `json:"name" binding:"required"`
}
