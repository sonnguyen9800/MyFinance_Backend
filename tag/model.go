package tag

type Tag struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CreateTagRequest struct {
	Name string `json:"name" binding:"required"`
}
