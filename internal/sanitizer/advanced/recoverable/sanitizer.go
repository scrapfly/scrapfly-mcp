package recoverable

import (
	"log"
	"reflect"
)

func SanitizeNils(obj interface{}) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("SanitizeNils: recovered at top-level: %v", r)
		}
	}()
	if obj == nil {
		return
	}
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}
	visited := make(map[uintptr]struct{})
	sanitizeValue(v, visited)
}

/* ====== internals ====== */

func sanitizeValue(v reflect.Value, visited map[uintptr]struct{}) {
	if !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return
		}
		// Cycle guard
		p := v.Pointer()
		if p != 0 {
			if _, ok := visited[p]; ok {
				return
			}
			visited[p] = struct{}{}
		}
		sanitizeValue(v.Elem(), visited)

	case reflect.Interface:
		if v.IsNil() {
			return
		}
		iv := v.Elem()
		switch iv.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Map:
			sanitizeValue(iv, visited)
		case reflect.Struct:
			if v.CanSet() {
				cp := reflect.New(iv.Type())
				cp.Elem().Set(iv)
				sanitizeValue(cp, visited)
				trySet(v, cp.Elem()) // interface <- struct
			}
		}

	case reflect.Struct:
		sanitizeStruct(v, visited)

	case reflect.Slice:
		// Slice container
		if v.CanSet() && v.IsNil() {
			defer recoverIn("make slice")
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		}
		// Elements
		for i := 0; i < v.Len(); i++ {
			el := v.Index(i)
			newEl := sanitizeElement(el, visited)
			trySetSliceIndex(v, i, newEl)
		}

	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			el := v.Index(i)
			newEl := sanitizeElement(el, visited)
			trySetSliceIndex(v, i, newEl)
		}

	case reflect.Map:
		if v.CanSet() && v.IsNil() {
			defer recoverIn("make map")
			v.Set(reflect.MakeMap(v.Type()))
		}
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key()
			val := iter.Value()
			newVal := sanitizeElement(val, visited)
			trySetMapIndex(v, k, newVal)
		}

	default:
		// primitives: no-op
	}
}

func sanitizeStruct(v reflect.Value, visited map[uintptr]struct{}) {
	n := v.NumField()
	for i := 0; i < n; i++ {
		f := v.Field(i)
		switch f.Kind() {
		case reflect.Slice:
			if f.CanSet() && f.IsNil() {
				defer recoverIn("make slice(field)")
				f.Set(reflect.MakeSlice(f.Type(), 0, 0))
			}
			for j := 0; j < f.Len(); j++ {
				el := f.Index(j)
				newEl := sanitizeElement(el, visited)
				trySetSliceIndex(f, j, newEl)
			}

		case reflect.Array:
			for j := 0; j < f.Len(); j++ {
				el := f.Index(j)
				newEl := sanitizeElement(el, visited)
				trySetSliceIndex(f, j, newEl)
			}

		case reflect.Map:
			if f.CanSet() && f.IsNil() {
				defer recoverIn("make map(field)")
				f.Set(reflect.MakeMap(f.Type()))
			}
			iter := f.MapRange()
			for iter.Next() {
				k := iter.Key()
				val := iter.Value()
				newVal := sanitizeElement(val, visited)
				trySetMapIndex(f, k, newVal)
			}

		case reflect.Struct:
			if f.CanAddr() {
				sanitizeValue(f.Addr(), visited)
			} else if f.CanSet() {
				cp := reflect.New(f.Type())
				cp.Elem().Set(f)
				sanitizeValue(cp, visited)
				trySet(f, cp.Elem())
			}

		case reflect.Ptr, reflect.Interface:
			sanitizeValue(f, visited)

		default:
			// no-op
		}
	}
}

// sanitizeElement clean an element and return a possibly assignable copy.
func sanitizeElement(el reflect.Value, visited map[uintptr]struct{}) reflect.Value {
	switch el.Kind() {
	case reflect.Ptr:
		if el.IsNil() {
			return el
		}
		sanitizeValue(el, visited)
		return el

	case reflect.Interface:
		if el.IsNil() {
			return el
		}
		iv := el.Elem()
		switch iv.Kind() {
		case reflect.Ptr, reflect.Slice, reflect.Map:
			sanitizeValue(iv, visited)
			return el
		case reflect.Struct:
			cp := reflect.New(iv.Type())
			cp.Elem().Set(iv)
			sanitizeValue(cp, visited)
			return cp.Elem()
		default:
			return el
		}

	case reflect.Struct:
		cp := reflect.New(el.Type())
		cp.Elem().Set(el)
		sanitizeValue(cp, visited)
		return cp.Elem()

	case reflect.Slice:
		if el.IsNil() {
			defer recoverIn("make slice(elem)")
			return reflect.MakeSlice(el.Type(), 0, 0)
		}
		n := el.Len()
		defer recoverIn("copy slice(elem)")
		newSlice := reflect.MakeSlice(el.Type(), n, n)
		for i := 0; i < n; i++ {
			se := sanitizeElement(el.Index(i), visited)
			trySet(newSlice.Index(i), se)
		}
		return newSlice

	case reflect.Map:
		if el.IsNil() {
			defer recoverIn("make map(elem)")
			return reflect.MakeMap(el.Type())
		}
		defer recoverIn("copy map(elem)")
		newMap := reflect.MakeMap(el.Type())
		iter := el.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()
			sv := sanitizeElement(v, visited)
			trySetMapIndex(newMap, k, sv)
		}
		return newMap

	default:
		return el
	}
}

/* ====== safe setters & recover helpers ====== */

func trySet(dst, src reflect.Value) {
	defer recoverIn("trySet")
	if !dst.IsValid() || !dst.CanSet() || !src.IsValid() {
		return
	}
	if src.Type().AssignableTo(dst.Type()) {
		dst.Set(src)
		return
	}
	if src.Type().ConvertibleTo(dst.Type()) {
		dst.Set(src.Convert(dst.Type()))
	}
}

func trySetSliceIndex(slice reflect.Value, idx int, src reflect.Value) {
	defer recoverIn("trySetSliceIndex")
	if !slice.IsValid() || idx < 0 || idx >= slice.Len() {
		return
	}
	dst := slice.Index(idx)
	trySet(dst, src)
}

func trySetMapIndex(m, k, v reflect.Value) {
	defer recoverIn("trySetMapIndex")
	if !m.IsValid() || m.Kind() != reflect.Map {
		return
	}
	t := m.Type().Elem()
	if !v.IsValid() {
		// keep the existing value
		return
	}
	if v.Type().AssignableTo(t) {
		m.SetMapIndex(k, v)
		return
	}
	if v.Type().ConvertibleTo(t) {
		m.SetMapIndex(k, v.Convert(t))
		return
	}
	// otherwise no-op : avoid panic
}

func recoverIn(where string) {
	if r := recover(); r != nil {
		log.Printf("SanitizeNils: recovered in %s: %v", where, r)
	}
}
