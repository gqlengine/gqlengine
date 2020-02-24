package gqlengine

import (
	"testing"
)

type CustomIntScalar int

func (c *CustomIntScalar) GraphQLScalarSerialize() interface{} {
	return int(*c)
}

func (c *CustomIntScalar) GraphQLScalarParseValue(value interface{}) {
	*c = CustomIntScalar(value.(int))
}

func (c *CustomIntScalar) GraphQLScalarDescription() string {
	return "custom int scalar"
}

type CustomStringScalar string

func (c *CustomStringScalar) GraphQLScalarSerialize() interface{} {
	return string(*c)
}

func (c *CustomStringScalar) GraphQLScalarParseValue(value interface{}) {
	*c = CustomStringScalar(value.(string))
}

func (c *CustomStringScalar) GraphQLScalarDescription() string {
	return "custom string scalar"
}

type CustomStructScalar struct {
	intField int
}

func (c *CustomStructScalar) GraphQLScalarSerialize() interface{} {
	return c.intField
}

func (c *CustomStructScalar) GraphQLScalarParseValue(value interface{}) {
	c.intField = value.(int)
}

func (c *CustomStructScalar) GraphQLScalarDescription() string {
	return "custom string scalar"
}

func TestScalar(t *testing.T) {
	engine := NewEngine(Options{})
	scalar, err := engine.RegisterScalar(CustomIntScalar(0))
	if err != nil {
		t.Fatal(err)
	}
	if scalar.Serialize(CustomIntScalar(1)) != 1 {
		t.Fatal("serialize scalar failed")
	}
	intValue := scalar.ParseValue(1).(*CustomIntScalar)
	if *intValue != CustomIntScalar(1) {
		t.Fatal("parseValue failed")
	}

	scalar, err = engine.RegisterScalar(CustomStringScalar(""))
	if err != nil {
		t.Fatal(err)
	}
	if scalar.Serialize(CustomStringScalar("any word")) != "any word" {
		t.Fatal("serialize scalar string")
	}
	stringValue := scalar.ParseValue("text").(*CustomStringScalar)
	if *stringValue != CustomStringScalar("text") {
		t.Fatal("parseValue failed")
	}

	scalar, err = engine.RegisterScalar(CustomStructScalar{0})
	if err != nil {
		if err.Error() != "struct-based scalar contains unexported field, may not be serialized" {
			t.Fatal(err)
		}
	} else {
		if result := scalar.Serialize(CustomStructScalar{1}); result != 1 {
			t.Errorf("serialize expect 1 but %d", result)
		}
		structValue := scalar.ParseValue(-1).(*CustomStructScalar)
		if structValue.intField != -1 {
			t.Fatal("parseValue failed")
		}
	}
}
