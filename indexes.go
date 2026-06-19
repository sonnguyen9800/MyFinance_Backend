package main

import (
	"context"
	"my-finance-backend/config"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ensureIndexes creates the database indexes the app relies on. Idempotent —
// MongoDB ignores indexes that already exist, so it is safe on every startup.
func ensureIndexes(client *mongo.Client, cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	db := client.Database(cfg.DatabaseName)

	// expenses:
	//   {user_id, date desc}    dominant "list my recent expenses" query
	//   {user_id, category_id}  category filter (user_id prefix keeps it selective)
	//   {user_id, tag_ids}      multikey, supports the tag filter
	// Name search still uses a case-insensitive substring regex (not index-backed)
	// to preserve substring UX; an indexed search is tracked as its own task.
	if _, err := db.Collection(cfg.CollectionExpensesName).Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "date", Value: -1}}},
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "category_id", Value: 1}}},
		{Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "tag_ids", Value: 1}}},
	}); err != nil {
		return err
	}

	// categories: {user_id, name}
	if _, err := db.Collection(cfg.CollectionCategoriesName).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "name", Value: 1}},
	}); err != nil {
		return err
	}

	// tags: {user_id, name}
	if _, err := db.Collection(cfg.CollectionTagsName).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "user_id", Value: 1}, {Key: "name", Value: 1}},
	}); err != nil {
		return err
	}

	// users: unique {email}
	if _, err := db.Collection(cfg.CollectionUserName).Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		return err
	}

	return nil
}
