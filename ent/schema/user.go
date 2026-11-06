package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.String("email").Unique(),
		field.String("password"),
		field.String("session_token").Optional().Nillable(),
		field.Time("created_at").Default(time.Now()).Immutable(),
		field.Time("updated_at"),
		field.Time("last_watched_ad_date").Optional().Nillable(),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("device", Devices.Type).
			Unique(),
		edge.To("subscription", Subscriptions.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
