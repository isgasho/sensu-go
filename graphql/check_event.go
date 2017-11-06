package graphqlschema

import (
	"errors"

	"github.com/graphql-go/graphql"
	"github.com/sensu/sensu-go/backend/authorization"
	"github.com/sensu/sensu-go/backend/store"
	"github.com/sensu/sensu-go/graphql/globalid"
	"github.com/sensu/sensu-go/graphql/relay"
	"github.com/sensu/sensu-go/types"
)

var checkEventType *graphql.Object
var checkEventConnection *relay.ConnectionDefinitions

func init() {
	initNodeInterface()
	initCheckEventType()
	initCheckEventConnection()

	nodeResolver := newCheckEventNodeResolver()
	nodeRegister.RegisterResolver(nodeResolver)
}

func initCheckEventType() {
	if checkEventType != nil {
		return
	}

	checkEventType = graphql.NewObject(graphql.ObjectConfig{
		Name:        "CheckEvent",
		Description: "A check result",
		Interfaces: graphql.InterfacesThunk(func() []*graphql.Interface {
			return []*graphql.Interface{
				nodeInterface,
			}
		}),
		Fields: graphql.FieldsThunk(func() graphql.Fields {
			return graphql.Fields{
				"id": &graphql.Field{
					Name:        "id",
					Description: "The ID of an object",
					Type:        graphql.NewNonNull(graphql.ID),
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						idComponents := globalid.EventTranslator.Encode(p.Source)
						return idComponents.String(), nil
					},
				},

				"timestamp": &graphql.Field{
					Type:        timeScalar,
					Description: "The time at which the event occurred.",
				},

				"entity": &graphql.Field{
					Type:        entityType,
					Description: "The entity in-which the event occurred on.",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						abilities := authorization.Entities.WithContext(p.Context)
						event, ok := p.Source.(*types.Event)
						if !ok {
							return nil, errors.New("source object is not an Event")
						}

						if abilities.CanRead(event.Entity) {
							return event.Entity, nil
						}
						return nil, nil
					},
				},

				"output": &graphql.Field{
					Type:        graphql.String,
					Description: "The output that was returned after executing the check.",
					Resolve:     AliasResolver("Check", "Output"),
				},

				"status": &graphql.Field{
					Type:        graphql.Int,
					Description: `The status code that was received after executing the check.`,
					Resolve:     AliasResolver("Check", "Status"),
				},

				"issued": &graphql.Field{
					Type:        timeScalar,
					Description: "The time at which the check was scheduled.",
					Resolve:     AliasResolver("Check", "Issued"),
				},

				"executed": &graphql.Field{
					Type:        timeScalar,
					Description: "The time at which the check was executed.",
					Resolve:     AliasResolver("Check", "Executed"),
				},

				"config": &graphql.Field{
					Type:        checkConfigType,
					Description: "The configuration of the check that was executed.",
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						abilities := authorization.Checks.WithContext(p.Context)
						event, ok := p.Source.(*types.Event)
						if !ok {
							return nil, errors.New("source object is not an Event")
						}

						if abilities.CanRead(event.Check.Config) {
							return event.Check.Config, nil
						}
						return nil, nil
					},
				},
			}
		}),
		IsTypeOf: func(p graphql.IsTypeOfParams) bool {
			if e, ok := p.Value.(*types.Event); ok {
				return e.Check != nil
			}
			return false
		},
	})
}

func initCheckEventConnection() {
	if checkEventConnection != nil {
		return
	}

	checkEventConnection = relay.NewConnectionDefinition(relay.ConnectionConfig{
		Name:     "CheckEvent",
		NodeType: checkEventType,
	})
}

func newCheckEventNodeResolver() relay.NodeResolver {
	return relay.NodeResolver{
		Object:     checkEventType,
		Translator: globalid.EventTranslator,
		Resolve: func(p relay.NodeResolverParams) (interface{}, error) {
			components := p.IDComponents.(globalid.EventComponents)
			store := p.Context.Value(types.StoreKey).(store.EventStore)
			events, err := store.GetEventsByEntity(p.Context, components.EntityName())
			if err != nil {
				return nil, err
			}

			abilities := authorization.Events.WithContext(p.Context)
			for _, event := range events {
				if event.Timestamp == components.Timestamp() &&
					event.Check.Config.Name == components.CheckName() &&
					abilities.CanRead(event) {
					return event, nil
				}
			}

			return nil, nil
		},
		IsKindOf: func(components globalid.Components) bool {
			return components.ResourceType() == "check"
		},
	}
}