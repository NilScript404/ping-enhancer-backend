package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Games holds the schema definition for the Games entity.
type Games struct {
	ent.Schema
}

// Fields of the Games.
func (Games) Fields() []ent.Field {
	return []ent.Field{
		field.String("name"),
		field.String("process_name"),
	}
}

// Edges of the Games.
func (Games) Edges() []ent.Edge {
	return nil
}
