// THIS FILE IS PART OF GQLENGINE PROJECT, COPYRIGHTS BELONGS TO 凯斐德科技（杭州）有限公司.
package gqlengine

import (
	"github.com/karfield/graphql"
)

type (
	tagEntry struct {
		Type      string                   `json:"type"`
		FieldName string                   `json:"fieldName"`
		Field     *graphql.FieldDefinition `json:"field"`
	}

	tag struct {
		Name    string `json:"name"`
		Entries []tagEntry
	}

	tagEntries struct {
		queries       map[string]struct{}
		mutations     map[string]struct{}
		subscriptions map[string]struct{}
	}
)

var (
	_tagEntry = graphql.NewObject(graphql.ObjectConfig{
		Name: "__TagEntry",
		Fields: graphql.Fields{
			"type":      {Type: graphql.String},
			"fieldName": {Type: graphql.String},
			"field":     {Type: graphql.FieldType},
		},
	})

	_tag = graphql.NewObject(graphql.ObjectConfig{
		Name: "__Tag",
		Fields: graphql.Fields{
			"name":    {Type: graphql.String},
			"entries": {Type: graphql.NewList(_tagEntry)},
		},
	})
)

const (
	tagQuery = iota
	tagMutation
	tagSubscription
)

func (t *tagEntries) add(op int, name string) {
	var m *map[string]struct{}
	switch op {
	case tagQuery:
		m = &t.queries
	case tagMutation:
		m = &t.mutations
	case tagSubscription:
		m = &t.subscriptions
	default:
		return
	}
	if *m == nil {
		*m = map[string]struct{}{}
	}
	(*m)[name] = struct{}{}
}

func (engine *Engine) queryTags() {
	if engine.query == nil {
		engine.query = graphql.NewObject(graphql.ObjectConfig{
			Name:   "Query",
			Fields: graphql.Fields{},
		})
	}

	engine.query.AddFieldConfig("__tags", &graphql.Field{
		Description: "get docs by tags",
		Type:        graphql.NewList(_tag),
		Resolve: graphql.ResolveField(func(params graphql.ResolveParams) (interface{}, error) {
			var tags []*tag
			if len(engine.tags) > 0 {
				for t, ents := range engine.tags {
					var entries []tagEntry
					getEntries := func(object *graphql.Object) {
						fieldMap := object.FieldMap()
						for name := range ents.queries {
							f := fieldMap[name]
							entry := tagEntry{
								Type:      "query",
								FieldName: name,
								Field:     f,
							}
							entries = append(entries, entry)
						}
					}
					if len(ents.queries) > 0 {
						getEntries(engine.schema.QueryType())
					}
					if len(ents.mutations) > 0 {
						getEntries(engine.schema.MutationType())
					}
					if len(ents.subscriptions) > 0 {
						getEntries(engine.schema.SubscriptionType())
					}
					tags = append(tags, &tag{Name: t})
				}
			}
			return tags, nil
		}),
	})
}

func (engine *Engine) addTags(op int, name string, tags []string) {
	if len(tags) > 0 {
		for _, tag := range tags {
			entries, ok := engine.tags[tag]
			if !ok {
				entries = &tagEntries{}
			}
			entries.add(op, name)
			if !ok {
				engine.tags[tag] = entries
			}
		}
	} else {
		entries, ok := engine.tags[""]
		if !ok {
			entries = &tagEntries{}
		}
		entries.add(op, name)
		if !ok {
			engine.tags[""] = entries
		}
	}
}
