package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Subscriptions holds the schema definition for the Subscriptions entity.
type Subscriptions struct {
	ent.Schema
}

// Fields of the Subscriptions.
func (Subscriptions) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id"),
		field.Bool("is_active"),
		field.String("tier"),
		field.Int("price"),
		field.Int64("bytes_total").Default(0),
		field.Int64("bytes_used").Default(0),
		field.Time("start_date"),
		field.Time("end_date"),
		field.Time("purchase_date"),
	}
}

// Edges of the Subscriptions.
func (Subscriptions) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("subscription").
			Unique().
			Field("user_id").
			Required(),
		edge.To("configs", Configs.Type),
	}
}
