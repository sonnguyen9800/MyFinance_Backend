package tag

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Repository encapsulates tag data access (per-user).
type Repository struct {
	tags     *mongo.Collection
	expenses *mongo.Collection
}

func NewRepository(db *mongo.Database, tagsColl, expensesColl string) *Repository {
	return &Repository{tags: db.Collection(tagsColl), expenses: db.Collection(expensesColl)}
}

func (r *Repository) NameExists(ctx context.Context, userID, name string, excludeID *primitive.ObjectID) (bool, error) {
	filter := bson.M{"name": name, "user_id": userID}
	if excludeID != nil {
		filter["_id"] = bson.M{"$ne": *excludeID}
	}
	n, err := r.tags.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (r *Repository) Create(ctx context.Context, t *Tag) error {
	res, err := r.tags.InsertOne(ctx, t)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		t.ID = oid.Hex()
	}
	return nil
}

func (r *Repository) ListByUser(ctx context.Context, userID string) ([]Tag, error) {
	cur, err := r.tags.Find(ctx, bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	tags := make([]Tag, 0)
	if err := cur.All(ctx, &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *Repository) GetByID(ctx context.Context, userID string, oid primitive.ObjectID) (*Tag, error) {
	var t Tag
	if err := r.tags.FindOne(ctx, bson.M{"_id": oid, "user_id": userID}).Decode(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (r *Repository) UpdateName(ctx context.Context, userID string, oid primitive.ObjectID, name string) error {
	res, err := r.tags.UpdateOne(ctx, bson.M{"_id": oid, "user_id": userID}, bson.M{"$set": bson.M{"name": name}})
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, userID string, oid primitive.ObjectID) error {
	res, err := r.tags.DeleteOne(ctx, bson.M{"_id": oid, "user_id": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

// PullFromExpenses removes the tag id from every expense that references it.
func (r *Repository) PullFromExpenses(ctx context.Context, userID, tagHex string) {
	_, _ = r.expenses.UpdateMany(ctx,
		bson.M{"user_id": userID, "tag_ids": tagHex},
		bson.M{"$pull": bson.M{"tag_ids": tagHex}})
}
