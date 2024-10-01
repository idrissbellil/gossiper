package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Job holds the schema definition for the Job entity.
type Job struct {
	ent.Schema
}

// Fields of the Job.
func (Job) Fields() []ent.Field {
	return []ent.Field{
		field.String("url").NotEmpty(),
		field.Enum("method").Values("get", "post").Default("get"),
		field.JSON("headers", map[string]string{}),
		field.String("data").Optional(),
		field.String("email").NotEmpty(),
		field.String("password").NotEmpty(),
		field.String("smtp_host").Optional(),
		field.Int("smtp_port").Optional(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the Job.
func (Job) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Required().Unique(),
	}
}
