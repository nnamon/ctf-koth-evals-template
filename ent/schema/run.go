package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Run struct {
	ent.Schema
}

func (Run) Fields() []ent.Field {
	return []ent.Field{
		field.String("seed").NotEmpty(),
		field.Enum("status").
			Values("pending", "claimed", "running", "succeeded", "failed", "timed_out").
			Default("pending"),
		field.Float("score").Optional().Nillable(),
		field.JSON("result", map[string]any{}).Optional(),
		field.String("error").Optional(),
		field.String("worker_id").Optional(),
		field.Time("claimed_at").Optional().Nillable(),
		field.Time("started_at").Optional().Nillable(),
		field.Time("finished_at").Optional().Nillable(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Run) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("submission", Submission.Type).
			Ref("runs").
			Unique().
			Required(),
		edge.From("suite", Suite.Type).
			Ref("runs").
			Unique().
			Required(),
	}
}

func (Run) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "claimed_at"),
	}
}
