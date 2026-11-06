package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Configs holds the schema definition for the Configs entity.
type Configs struct {
	ent.Schema
}

// Fields of the Configs.
func (Configs) Fields() []ent.Field {
	return []ent.Field{
		field.Int("subscription_id").Optional(),
		field.Int("device_id"),
		field.String("location"),
		field.String("value"),
		field.String("name"),
		field.String("route_type"),
		field.String("route_address"),
	}
}

// Edges of the Configs.
func (Configs) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("subscription", Subscriptions.Type).
			Ref("configs").
			Unique().
			Field("subscription_id"),

		edge.From("device", Devices.Type).
			Ref("configs").
			Unique().
			Field("device_id").
			Required(),
	}
}
