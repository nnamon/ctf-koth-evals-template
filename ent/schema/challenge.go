package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Challenge struct {
	ent.Schema
}

func (Challenge) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		field.String("version").NotEmpty(),
		field.String("description").Optional(),
		field.JSON("manifest", map[string]any{}),
		field.Bytes("bundle"),
		field.Int64("bundle_size").NonNegative(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Challenge) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("suites", Suite.Type),
	}
}

func (Challenge) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "version").Unique(),
		index.Fields("version"),
	}
}
