package users

type User struct {
	ID           string `bson:"_id"`
	Name         string `bson:"name"`
	Role         string `bson:"role"`
	Email        string `bson:"email"`
	PasswordHash string `bson:"password_hash"`
}
