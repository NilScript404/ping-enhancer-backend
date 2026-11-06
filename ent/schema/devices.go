package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Devices holds the schema definition for the Devices entity.
type Devices struct {
	ent.Schema
}

// Fields of the Devices.
func (Devices) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Optional(),
		field.String("hwid").Unique(),
		field.Int64("bytes_total").Default(0),
		field.Int64("bytes_used").Default(0),
	}
}

// Edges of the Devices.
func (Devices) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("device").
			Unique().
			Field("user_id"),
		edge.To("configs", Configs.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}
