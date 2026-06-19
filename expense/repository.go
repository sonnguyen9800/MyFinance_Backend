package expense

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Repository encapsulates all expense data access. Handlers depend on this,
// not on *mongo.Client / bson directly — which keeps business logic testable.
type Repository struct {
	expenses   *mongo.Collection
	categories *mongo.Collection
}

func NewRepository(db *mongo.Database, expensesColl, categoriesColl string) *Repository {
	return &Repository{
		expenses:   db.Collection(expensesColl),
		categories: db.Collection(categoriesColl),
	}
}

// ListQuery is the set of filters supported by List.
type ListQuery struct {
	UserID        string
	Offset        int
	Limit         int
	Search        string
	CategoryID    string
	TagID         string
	PaymentMethod string
	From          string
	To            string
}

// SumSince totals the amount of expenses within the last `days` calendar days.
func (r *Repository) SumSince(ctx context.Context, userID string, days int) (float64, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "user_id", Value: userID},
			{Key: "date", Value: bson.D{{Key: "$gte", Value: cutoff}}},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: "$amount"}}},
		}}},
	}
	cur, err := r.expenses.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cur.Close(ctx)
	var res []bson.M
	if err := cur.All(ctx, &res); err != nil {
		return 0, err
	}
	if len(res) > 0 {
		switch v := res[0]["total"].(type) {
		case float64:
			return v, nil
		case int64:
			return float64(v), nil
		case int32:
			return float64(v), nil
		}
	}
	return 0, nil
}

// CategoryExists reports whether a category with the given hex id exists.
func (r *Repository) CategoryExists(ctx context.Context, id string) (bool, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return false, err
	}
	count, err := r.categories.CountDocuments(ctx, bson.M{"_id": oid})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) Create(ctx context.Context, e *Expense) error {
	res, err := r.expenses.InsertOne(ctx, e)
	if err != nil {
		return err
	}
	if oid, ok := res.InsertedID.(primitive.ObjectID); ok {
		e.ID = oid.Hex()
	}
	return nil
}

func (r *Repository) Monthly(ctx context.Context, userID string, month, year int) ([]Expense, float64, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	filter := bson.M{
		"user_id": userID,
		"date":    bson.M{"$gte": start.Format("2006-01-02"), "$lt": end.Format("2006-01-02")},
	}
	cur, err := r.expenses.Find(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	expenses := make([]Expense, 0)
	if err := cur.All(ctx, &expenses); err != nil {
		return nil, 0, err
	}
	var total float64
	for _, e := range expenses {
		total += e.Amount
	}
	return expenses, total, nil
}

func (r *Repository) List(ctx context.Context, q ListQuery) ([]Expense, int64, error) {
	filter := bson.M{"user_id": q.UserID}
	if q.CategoryID != "" {
		filter["category_id"] = q.CategoryID
	}
	if q.Search != "" {
		filter["name"] = bson.M{"$regex": q.Search, "$options": "i"}
	}
	if q.PaymentMethod != "" {
		filter["payment_method"] = q.PaymentMethod
	}
	if q.TagID != "" {
		filter["tag_ids"] = q.TagID
	}
	if q.From != "" || q.To != "" {
		dr := bson.M{}
		if q.From != "" {
			dr["$gte"] = q.From
		}
		if q.To != "" {
			dr["$lte"] = q.To
		}
		filter["date"] = dr
	}

	total, err := r.expenses.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().
		SetSkip(int64(q.Offset)).
		SetLimit(int64(q.Limit)).
		SetSort(bson.D{{Key: "date", Value: -1}})
	cur, err := r.expenses.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)
	expenses := make([]Expense, 0)
	if err := cur.All(ctx, &expenses); err != nil {
		return nil, 0, err
	}
	return expenses, total, nil
}

func (r *Repository) GetByID(ctx context.Context, userID, id string) (*Expense, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var e Expense
	if err := r.expenses.FindOne(ctx, bson.M{"_id": oid, "user_id": userID}).Decode(&e); err != nil {
		return nil, err
	}
	return &e, nil
}

func (r *Repository) Update(ctx context.Context, userID, id string, update bson.M) (*Expense, error) {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	res, err := r.expenses.UpdateOne(ctx, bson.M{"_id": oid, "user_id": userID}, bson.M{"$set": update})
	if err != nil {
		return nil, err
	}
	if res.MatchedCount == 0 {
		return nil, mongo.ErrNoDocuments
	}
	return r.GetByID(ctx, userID, id)
}

func (r *Repository) Delete(ctx context.Context, userID, id string) error {
	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	res, err := r.expenses.DeleteOne(ctx, bson.M{"_id": oid, "user_id": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}
