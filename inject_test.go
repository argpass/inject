package inject

import (
	"fmt"
	"testing"
)

type sayer interface {
	Say()
}

type person struct {
}

var _ sayer = &person{}

func (p *person) Say() {
	fmt.Println("person say")
}

type male struct {
	name string
}

type holder interface {
	Hold()
}

type sHolder struct {
	Sayer sayer `inject:"tmp"`
}

func (p *sHolder) Hold(){
	fmt.Printf("hold sayer:%v", p.Sayer)
}

var _ sayer = &male{}

func (p *male) Say() {
	fmt.Println("male say ->", p.name)
}

func TestComponentFactory_ComponentsOfInterface_and_ResolveOne(t *testing.T) {

	var impl sayer = &person{}
	var impl2 sayer = &male{"akun"}
	var holderImpl = &sHolder{}

	builder := NewComponentFactoryBuilder()
	builder.Register(holderImpl)
	builder.Register(impl)
	builder.Register(impl2)
	builder.Register(person{})

	factory := builder.Build()
	got := factory.ComponentsOfInterface((*sayer)(nil))
	if len(got) == 0 {
		t.Log("should got one, but got 0")
		t.Fail()
	}
	t.Logf("got_len:%d", len(got))
	if _, ok := got[1].(sayer); ok {
	} else {
		t.Log("expect Sayer")
		t.Fail()
	}

	// got person by pointer
	var p *male
	success := factory.ResolveOne(&p)
	if !success {
		t.Log("expect to resolve *male")
		t.FailNow()
		t.Fail()
	}

	// fail to got male by struct
	var m = male{"no"}
	success = factory.ResolveOne(&m)
	if success {
		t.Log("expect to fail to resovle male struct")
		t.Fail()
	}

	var per person
	if !factory.ResolveOne(&per) {
		t.Log("expect to resolve person struct")
		t.Fail()
	}

	var h holder
	success = factory.ResolveOne(&h)
	if success {
		h.Hold()
	}else {
		t.Log("expect to resolve holder")
		t.Fail()
	}
}
