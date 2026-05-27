package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Submission struct {
	ent.Schema
}

func (Submission) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Optional(),
		field.String("submitter").Optional(),
		field.String("artifact_name").NotEmpty(),
		field.Bytes("artifact"),
		field.Int64("artifact_size").NonNegative(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Submission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("runs", Run.Type),
	}
}
