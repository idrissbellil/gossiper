package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Job holds the schema definition for the Job entity.
type Job struct {
	ent.Schema
}

// Fields of the Job.
func (Job) Fields() []ent.Field {
	return []ent.Field{
		field.String("email").NotEmpty(),
		field.String("from_regex").Default(".*"),

		field.String("url").
			NotEmpty().
			Comment("The URL to the webhook"),

		field.Enum("method").
			Values("GET", "POST", "PUT", "DELETE", "PATCH").
			Default("GET").
			Comment("HTTP method to use"),

		field.JSON("headers", map[string]string{}).
			Optional().
			Comment("HTTP headers to send with the request"),

		field.Text("payload_template").
			Optional().
			Comment("Template for the payload to be sent"),

		field.Bool("is_active").
			Default(true).
			Comment("Whether this webhook is currently active"),

		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

func (Job) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("user", User.Type).Required().Unique(),
	}
}

func (Job) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email").
			Unique(),
	}
}
