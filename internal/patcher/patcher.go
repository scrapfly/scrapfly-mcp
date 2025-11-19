package patcher

import (
	"errors"
	"fmt"
	"reflect"
	"unsafe"
)

func SetUnexportedField(ptr interface{}, fieldName string, value interface{}) error {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return errors.New("ptr must be a non-nil pointer to a struct")
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return errors.New("ptr must point to a struct")
	}

	field := elem.FieldByName(fieldName)
	if !field.IsValid() {
		return fmt.Errorf("field %q not found in struct", fieldName)
	}

	val := reflect.ValueOf(value)
	if !val.IsValid() {
		val = reflect.Zero(field.Type())
	} else {
		if !val.Type().AssignableTo(field.Type()) {
			if val.Type().ConvertibleTo(field.Type()) {
				val = val.Convert(field.Type())
			} else {
				return fmt.Errorf("value of type %s is not assignable/convertible to field %s (type %s)",
					val.Type(), fieldName, field.Type())
			}
		}
	}

	if field.CanSet() {
		field.Set(val)
		return nil
	}

	fieldPtr := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr()))
	fieldPtr.Elem().Set(val)
	return nil
}

func GetUnexportedField(ptr interface{}, fieldName string) (interface{}, error) {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return nil, errors.New("ptr must be a non-nil pointer to a struct")
	}
	elem := v.Elem()
	if elem.Kind() != reflect.Struct {
		return nil, errors.New("ptr must point to a struct")
	}

	field := elem.FieldByName(fieldName)
	if !field.IsValid() {
		return nil, fmt.Errorf("field %q not found in struct", fieldName)
	}

	if field.CanInterface() {
		return field.Interface(), nil
	}

	fieldPtr := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr()))
	return fieldPtr.Elem().Interface(), nil
}
