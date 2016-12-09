package inject

import (
	"container/list"
	"fmt"
	"reflect"
)

const (
	inject_token = "inject"
)

// PendingComponent is unresolved
type PendingComponent struct {
	wrapped        interface{}
	pending_fields map[reflect.Type]*list.List
}
type field struct {
	FieldValue reflect.Value
	FieldType  reflect.StructField
}

func (p *PendingComponent) GetWrappedType() reflect.Type {
	return reflect.TypeOf(p.wrapped)
}

func (p *PendingComponent) GetWrapped() interface{} {
	return p.wrapped
}

func (p *PendingComponent) IsResolved() bool {
	return len(p.pending_fields) == 0
}

func NewPendingComponent(v interface{}) *PendingComponent {
	p := &PendingComponent{}
	p.wrapped = v
	p.pending_fields = make(map[reflect.Type]*list.List)

	value := reflect.Indirect(reflect.ValueOf(v))
	_type := value.Type()
	for i := 0; i < value.NumField(); i++ {
		f := value.Field(i)
		fieldType := _type.Field(i)
		if _, ok := fieldType.Tag.Lookup(inject_token); ok {
			fields, ok := p.pending_fields[f.Type()]
			if !ok {
				fields = list.New()
				p.pending_fields[f.Type()] = fields
			}
			fields.PushBack(&field{FieldValue: f, FieldType: fieldType})
		}
	}
	return p
}

func (p *PendingComponent) WhenNewResolvedComponent(v interface{}) bool {
	for fieldValueType, fields := range p.pending_fields {
		componentType := reflect.TypeOf(v)
		if (fieldValueType.Kind() == reflect.Interface &&
			componentType.Implements(fieldValueType)) || fieldValueType == componentType {
			// bind v to the pending fields
			begin := fields.Front()
			for cur := begin; cur != nil; cur = cur.Next() {
				if fi, ok := cur.Value.(*field); ok {
					if !fi.FieldValue.CanSet() {
						panic(fmt.Errorf("Can't set the value on field %s",
							fi.FieldType.Name))
					}
					fi.FieldValue.Set(reflect.ValueOf(v))

				}
			}
			delete(p.pending_fields, fieldValueType)
		}
	}
	return p.IsResolved()
}

// ComponentFactoryBuilder create `ComponentFactory`
type ComponentFactoryBuilder interface {
	// Register register any interface{} to `ComponentFactory`
	Register(interface{}) ComponentFactoryBuilder

	// Build create `ComponentFactory` object
	Build() ComponentFactory
}

type ComponentFactory interface {
	// ComponentsOfType get all components that implements the type
	ComponentsOfType(reflect.Type) []interface{}

	// ComponentsOfInterface get all components that implements the interface
	// v is interface pointer
	ComponentsOfInterface(v interface{}) []interface{}

	// ResolveOne pass interface ptr, and factory would bind implement to the ptr of `pv`
	ResolveOne(pv interface{}) bool
}

type FactoryAware interface {
	SetFactory(ComponentFactory)
}

type componentFactory struct {
	mapping  map[reflect.Type]*PendingComponent
	resolved map[reflect.Type]interface{}
}

func newComponentFactory() *componentFactory {
	c := &componentFactory{}
	c.mapping = make(map[reflect.Type]*PendingComponent)
	c.resolved = make(map[reflect.Type]interface{})
	return c
}

func NewComponentFactoryBuilder() ComponentFactoryBuilder {
	return newComponentFactory()
}

func (c *componentFactory) Build() ComponentFactory {
	c.checkResolved()

	// after factory built, i can do something
	// notify all components implements FactoryAware
	for _, wrapped := range c.resolved {
		if wrapped, ok := wrapped.(FactoryAware); ok {
			wrapped.SetFactory(c)
		}
	}
	return c
}

func (c *componentFactory) checkResolved() {
	for t, pc := range c.mapping {
		//if fields, ok := pc.pending_fields[t]; ok {
		//	for cur := fields.Front(); cur != nil; cur=cur.Next() {
		//		if fi, ok := cur.Value.(*field); ok {
		//			fmt.Println(fi.FieldValue.Type())
		//			fmt.Println(fi.FieldType.Name)
		//			panic("go")
		//		}
		//	}
		//}
		panic(fmt.Errorf("%v unresolved dependency:%v\n please register the dependencied",
			t, pc.pending_fields))
	}
}

func (c *componentFactory) resolving(v interface{}) {
	for key, pc := range c.mapping {
		if pc.WhenNewResolvedComponent(v) {
			c.resolved[pc.GetWrappedType()] = pc.GetWrapped()
			delete(c.mapping, key)
			c.resolving(pc.GetWrapped())
		}
	}
}

func (c *componentFactory) Register(v interface{}) ComponentFactoryBuilder {
	pc := NewPendingComponent(v)
	if !pc.IsResolved() {
		// try to resolve the new component
		if pc.GetWrappedType().Kind() != reflect.Ptr {
			panic(fmt.Errorf("Never to register an unresolved struct"))
		}
		for _, wrapped := range c.resolved {
			if pc.WhenNewResolvedComponent(wrapped) {
				break
			}
		}
	}

	if pc.IsResolved() {
		c.resolved[pc.GetWrappedType()] = pc.GetWrapped()
		// try to resolving pending components
		c.resolving(pc.GetWrapped())
	} else {
		c.mapping[pc.GetWrappedType()] = pc
	}
	return c
}

func (c *componentFactory) ComponentsOfType(t reflect.Type) []interface{} {
	// maybe pass in a ptr of interface, convert it as interface
	//for k := t.Kind(); k == reflect.Ptr; k=t.Kind(){
	//	t = t.Elem()
	//}
	//// only can lookup interface
	//if t.Kind() != reflect.Interface {
	//	panic(fmt.Errorf("I need interface but got %v", t.Kind()))
	//}
	matched := []interface{}{}
	for key, val := range c.resolved {
		if key.AssignableTo(t) {
			matched = append(matched, val)
		} else if t.Kind() == reflect.Interface && key.Implements(t) {
			matched = append(matched, val)
		}
	}
	if len(matched) > 0 {
		return matched
	}
	return nil
}

func (c *componentFactory) ComponentsOfInterface(v interface{}) []interface{} {
	if v == nil {
		panic(fmt.Errorf("Expect interface pointer, got:nil"))
	}
	targetType := reflect.TypeOf(v)
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}
	if targetType.Kind() != reflect.Interface {
		panic(fmt.Errorf("Expect interface pointer, got:%v", targetType.Kind()))
	}
	return c.ComponentsOfType(targetType)
}

func (c *componentFactory) ResolveOne(pv interface{}) bool {
	if reflect.ValueOf(pv).Kind() != reflect.Ptr {
		panic(fmt.Errorf("Expect ptr kind"))
	}
	pointedType := reflect.TypeOf(pv).Elem()
	cp := c.ComponentsOfType(pointedType)
	if cp == nil {
		return false
	}
	if len(cp) > 1 {
		panic(fmt.Errorf("Multi matched components"))
	}
	val := reflect.ValueOf(cp[0])

	target := reflect.ValueOf(pv)
	target.Elem().Set(val)
	return true
}
