package errorprinter

import (
	"fmt"
	"testing"
)

type Interface interface {
	Error(...interface{})
	Errorf(string, ...interface{})
	Fatal(...interface{})
	Fatalf(string, ...interface{})
}

var _ Interface = (*testing.T)(nil)

type Panicker struct{}

func (Panicker) Error(args ...interface{})            { panic(fmt.Sprint(args...)) }
func (Panicker) Errorf(f string, args ...interface{}) { panic(fmt.Sprintf(f, args...)) }
func (Panicker) Fatal(args ...interface{})            { panic(fmt.Sprint(args...)) }
func (Panicker) Fatalf(f string, args ...interface{}) { panic(fmt.Sprintf(f, args...)) }
