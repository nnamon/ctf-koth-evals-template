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
		// SHA-256 of the artifact bytes, hex-encoded. Surfaced on the UI so
		// operators can confirm at a glance whether two submissions are
		// byte-identical. Optional so the additive migration doesn't break
		// rows uploaded before this column existed.
		field.String("artifact_sha256").Optional().Immutable().MaxLen(64),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Submission) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("runs", Run.Type),
	}
}
