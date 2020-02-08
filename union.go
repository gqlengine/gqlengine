package gqlengine

import (
	"fmt"
	"reflect"

	"github.com/karfield/graphql"
)

type unionConfig struct {
	union graphql.Union
	types map[reflect.Type]struct{}
}

func (engine *Engine) PreRegisterUnion(interfacePrototype interface{}, types ...interface{}) error {
	unionType := unwrapForKind(interfacePrototype, reflect.Interface)
	if unionType == nil {
		return fmt.Errorf("")
	}

	if _, ok := engine.unions[unionType]; ok {
		return nil
	}

	if len(types) == 0 {
		return fmt.Errorf("missing types")
	}

	config := &unionConfig{
		types: map[reflect.Type]struct{}{},
	}
	engine.types[unionType] = &config.union

	for _, typ := range types {
		t := reflect.TypeOf(typ)
		_, info, err := engine.asObject(t)
		if err != nil {
			return fmt.Errorf("%s is not an object: %E", t, err)
		}
		config.types[info.baseType] = struct{}{}
	}
	engine.unions[unionType] = config
	return nil
}

func (engine *Engine) completeUnions() error {
	for ut, uc := range engine.unions {
		var types []*graphql.Object
		for t := range uc.types {
			if ot, ok := engine.types[t]; !ok {
				return fmt.Errorf("no such type %s for union %s", t.Name(), ut.Name())
			} else if o, ok := ot.(*graphql.Object); !ok {
				return fmt.Errorf("type %s is not an object for union %s", ot.Name(), ut.Name())
			} else {
				types = append(types, o)
			}
		}
		err := graphql.InitUnion(&uc.union, graphql.UnionConfig{
			Name:        ut.Name(),
			Types:       types,
			Description: "",
			ResolveType: func(p graphql.ResolveTypeParams) *graphql.Object {
				pt := reflect.TypeOf(p.Value)
				if pt.Implements(ut) {
					if info, err := unwrap(reflect.TypeOf(p.Value)); err == nil {
						if typ, ok := engine.types[info.baseType]; ok {
							if obj, ok := typ.(*graphql.Object); ok {
								return obj
							}
						}
					}
				}
				return nil
			},
		})
		if err != nil {
			return fmt.Errorf("init union %s failed %E", ut.Name(), err)
		}
	}
	return nil
}

func (engine *Engine) asUnionResult(p reflect.Type) (*unwrappedInfo, error) {
	_, unionType := unwrapInterface(p)
	if unionType == nil {
		return nil, nil
	}
	if _, ok := engine.unions[unionType]; ok {
		return &unwrappedInfo{
			ptrType:  unionType,
			implType: unionType,
			baseType: unionType,
		}, nil
	}
	return nil, nil
}

func (engine *Engine) asUnionField(field *reflect.StructField) (graphql.Type, *unwrappedInfo, error) {
	array, unionType := unwrapInterface(field.Type)
	if unionType == nil {
		return nil, nil, nil
	}
	if uc, ok := engine.unions[unionType]; ok {
		return wrapType(field, &uc.union, array), &unwrappedInfo{
			ptrType:  unionType,
			implType: unionType,
			baseType: unionType,
		}, nil
	}
	return nil, nil, nil
}
