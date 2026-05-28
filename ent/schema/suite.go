package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Suite struct {
	ent.Schema
}

func (Suite) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		field.String("description").Optional(),
		field.JSON("parameters", map[string]any{}).Optional(),
		field.JSON("seeds", []string{}),
		field.Int("timeout_seconds").Positive().Default(3600),
		field.JSON("scoring", map[string]any{}).Optional(),
		field.Bool("sealed").Default(false),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Suite) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("challenge", Challenge.Type).
			Ref("suites").
			Unique().
			Required(),
		edge.To("runs", Run.Type),
		edge.To("clones", Suite.Type).
			From("parent").
			Unique(),
	}
}

func (Suite) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sealed"),
	}
}
