package category

import "go.mongodb.org/mongo-driver/bson/primitive"

type CreateCategoryRequest struct {
	Name     string `json:"name" binding:"required" bson:"name"`
	Color    string `json:"color" bson:"color"`
	IconName string `json:"icon_name" bson:"icon_name"`
}

type Category struct {
	ID       primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID   string             `json:"user_id" bson:"user_id"`
	Name     string             `json:"name" bson:"name"`
	Color    string             `json:"color" bson:"color"`
	IconName string             `json:"icon_name" bson:"icon_name"`
}

type UpdateCategoryRequest struct {
	Name     string `json:"name" bson:"name,omitempty"`
	Color    string `json:"color" bson:"color,omitempty"`
	IconName string `json:"icon_name" bson:"icon_name,omitempty"`
}
