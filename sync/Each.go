package sync

import (
	"errors"
	"reflect"
)

func expects(str string) error {
	return errors.New("fun/async.Each expects " + str)
}

func Each(items interface{}, fn interface{}) (err error) {
	vItems := reflect.ValueOf(items)
	tItems := vItems.Type()
	if tItems.Kind() != reflect.Slice {
		return expects("items to be a slice")
	}

	vFn := reflect.ValueOf(fn)
	tFn := vFn.Type()
	numIn := tFn.NumIn()
	if numIn != 1 && numIn != 2 {
		return expects("fn to take one or two arguments")
	}
	if tFn.In(0) != tItems.Elem() {
		return expects("fn argument type to match items")
	}
	if tFn.NumOut() != 1 {
		return expects("fn to return an error")
	}

	for i := 0; i < vItems.Len(); i++ {
		var args []reflect.Value
		if numIn == 1 {
			args = []reflect.Value{vItems.Index(i)}
		} else {
			args = []reflect.Value{vItems.Index(i), reflect.ValueOf(i)}
		}

		vErr := vFn.Call(args)
		if !vErr[0].IsNil() {
			return vErr[0].Interface().(error)
		}
	}

	return nil
}
