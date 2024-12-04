package category

type CreateCategoryRequest struct {
	Name     string `json:"name" binding:"required"`
	Color    string `json:"color"`
	IconName string `json:"icon_name"`
}

type Category struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Color    string `json:"color"`
	IconName string `json:"icon_name"`
}

type UpdateCategoryRequest struct {
	Name     string `json:"name"`
	Color    string `json:"color"`
	IconName string `json:"icon_name"`
}
