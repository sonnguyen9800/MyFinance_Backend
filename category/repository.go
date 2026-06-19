package category

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Repository encapsulates category data access (per-user) and the expense
// reassignment needed when a category is deleted.
type Repository struct {
	categories *mongo.Collection
	expenses   *mongo.Collection
}

func NewRepository(db *mongo.Database, categoriesColl, expensesColl string) *Repository {
	return &Repository{categories: db.Collection(categoriesColl), expenses: db.Collection(expensesColl)}
}

// EnsureDefault creates the user's Default category if it doesn't exist.
func (r *Repository) EnsureDefault(ctx context.Context, userID string) error {
	n, err := r.categories.CountDocuments(ctx, bson.M{"name": DefaultCategoryName, "user_id": userID})
	if err != nil {
		return err
	}
	if n == 0 {
		_, err = r.categories.InsertOne(ctx, Category{
			UserID:   userID,
			Name:     DefaultCategoryName,
			Color:    DefaultCategoryColor,
			IconName: DefaultCategoryIconName,
		})
		return err
	}
	return nil
}

// FindByName returns the category or mongo.ErrNoDocuments.
func (r *Repository) FindByName(ctx context.Context, userID, name string) (*Category, error) {
	var cat Category
	if err := r.categories.FindOne(ctx, bson.M{"name": name, "user_id": userID}).Decode(&cat); err != nil {
		return nil, err
	}
	return &cat, nil
}

func (r *Repository) Create(ctx context.Context, cat *Category) error {
	res, err := r.categories.InsertOne(ctx, cat)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		cat.ID = oid.Hex()
	}
	return nil
}

func (r *Repository) ListByUser(ctx context.Context, userID string) ([]Category, error) {
	cur, err := r.categories.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	cats := make([]Category, 0)
	if err := cur.All(ctx, &cats); err != nil {
		return nil, err
	}
	return cats, nil
}

func (r *Repository) GetByID(ctx context.Context, userID string, oid primitive.ObjectID) (*Category, error) {
	var cat Category
	if err := r.categories.FindOne(ctx, bson.M{"_id": oid, "user_id": userID}).Decode(&cat); err != nil {
		return nil, err
	}
	return &cat, nil
}

func (r *Repository) NameConflict(ctx context.Context, userID, name string, excludeID primitive.ObjectID) (bool, error) {
	n, err := r.categories.CountDocuments(ctx, bson.M{
		"name":    name,
		"user_id": userID,
		"_id":     bson.M{"$ne": excludeID},
	})
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *Repository) Update(ctx context.Context, userID string, oid primitive.ObjectID, update bson.M) error {
	res, err := r.categories.UpdateOne(ctx, bson.M{"_id": oid, "user_id": userID}, bson.M{"$set": update})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, userID string, oid primitive.ObjectID) error {
	res, err := r.categories.DeleteOne(ctx, bson.M{"_id": oid, "user_id": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// ReassignExpenses moves expenses from one category id to another (used on delete).
func (r *Repository) ReassignExpenses(ctx context.Context, userID, fromCatID, toCatID string) {
	_, _ = r.expenses.UpdateMany(ctx,
		bson.M{"user_id": userID, "category_id": fromCatID},
		bson.M{"$set": bson.M{"category_id": toCatID}})
}
