package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Worker struct {
	ent.Schema
}

func (Worker) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty(),
		field.Time("last_seen").Default(time.Now),
		field.Int64("runs_handled").NonNegative().Default(0),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (Worker) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Unique(),
		index.Fields("last_seen"),
	}
}
