package advanced

import "reflect"

// SanitizeNils replaces nil slices/maps with empty containers
// and recursively "descends" into structs/pointers/slices/maps to
// sanitize deeply. Idempotent, and protected against cycles.
func SanitizeNilsExtended(obj interface{}) {
	if obj == nil {
		return
	}
	visited := make(map[uintptr]struct{})
	sanitizeValue(reflect.ValueOf(obj), visited)
}

// --- internals ---

func sanitizeValue(v reflect.Value, visited map[uintptr]struct{}) {
	if !v.IsValid() {
		return
	}
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return
		}
		// cycle guard
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
			// we can clean the dynamic value directly
			sanitizeValue(iv, visited)
		case reflect.Struct:
			// if we can set the interface, we copy + clean + reassign
			if v.CanSet() {
				cp := reflect.New(iv.Type())
				cp.Elem().Set(iv)
				sanitizeValue(cp, visited)
				v.Set(cp.Elem())
			}
		}

	case reflect.Struct:
		sanitizeStruct(v, visited)

	case reflect.Slice:
		// top-level slice (if ever called directly)
		if v.CanSet() && v.IsNil() {
			v.Set(reflect.MakeSlice(v.Type(), 0, 0))
		}
		for i := 0; i < v.Len(); i++ {
			el := v.Index(i)
			newEl := sanitizeElement(el, visited)
			if newEl.IsValid() && v.Index(i).CanSet() && newEl.Type().AssignableTo(v.Index(i).Type()) {
				v.Index(i).Set(newEl)
			}
		}

	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
			el := v.Index(i)
			newEl := sanitizeElement(el, visited)
			if newEl.IsValid() && v.Index(i).CanSet() && newEl.Type().AssignableTo(v.Index(i).Type()) {
				v.Index(i).Set(newEl)
			}
		}

	case reflect.Map:
		// top-level map (if ever called directly)
		if v.CanSet() && v.IsNil() {
			v.Set(reflect.MakeMap(v.Type()))
		}
		iter := v.MapRange()
		for iter.Next() {
			k := iter.Key()
			val := iter.Value()
			newVal := sanitizeElement(val, visited)
			if newVal.IsValid() {
				v.SetMapIndex(k, newVal)
			}
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
				f.Set(reflect.MakeSlice(f.Type(), 0, 0))
			}
			for j := 0; j < f.Len(); j++ {
				el := f.Index(j)
				newEl := sanitizeElement(el, visited)
				if newEl.IsValid() && f.Index(j).CanSet() && newEl.Type().AssignableTo(f.Index(j).Type()) {
					f.Index(j).Set(newEl)
				}
			}

		case reflect.Array:
			for j := 0; j < f.Len(); j++ {
				el := f.Index(j)
				newEl := sanitizeElement(el, visited)
				if newEl.IsValid() && f.Index(j).CanSet() && newEl.Type().AssignableTo(f.Index(j).Type()) {
					f.Index(j).Set(newEl)
				}
			}

		case reflect.Map:
			if f.CanSet() && f.IsNil() {
				f.Set(reflect.MakeMap(f.Type()))
			}
			iter := f.MapRange()
			for iter.Next() {
				k := iter.Key()
				val := iter.Value()
				newVal := sanitizeElement(val, visited)
				if newVal.IsValid() {
					f.SetMapIndex(k, newVal)
				}
			}

		case reflect.Struct:
			if f.CanAddr() {
				sanitizeValue(f.Addr(), visited)
			} else if f.CanSet() {
				// struct by value non addressable → copy + clean + reassignment
				cp := reflect.New(f.Type())
				cp.Elem().Set(f)
				sanitizeValue(cp, visited)
				f.Set(cp.Elem())
			}

		case reflect.Ptr, reflect.Interface:
			sanitizeValue(f, visited)

		default:
			// no-op
		}
	}
}

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
			// clean copy of the struct contained in the interface
			cp := reflect.New(iv.Type())
			cp.Elem().Set(iv)
			sanitizeValue(cp, visited)
			return cp.Elem()
		default:
			return el
		}

	case reflect.Struct:
		// struct by value: we copy, we clean, we return the copy
		cp := reflect.New(el.Type())
		cp.Elem().Set(el)
		sanitizeValue(cp, visited)
		return cp.Elem()

	case reflect.Slice:
		// slice by value: if nil -> empty slice ; otherwise clean copy element by element
		if el.IsNil() {
			return reflect.MakeSlice(el.Type(), 0, 0)
		}
		n := el.Len()
		newSlice := reflect.MakeSlice(el.Type(), n, n)
		for i := 0; i < n; i++ {
			se := sanitizeElement(el.Index(i), visited)
			if se.IsValid() && se.Type().AssignableTo(newSlice.Index(i).Type()) && newSlice.Index(i).CanSet() {
				newSlice.Index(i).Set(se)
			} else {
				newSlice.Index(i).Set(el.Index(i))
			}
		}
		return newSlice

	case reflect.Map:
		// map by value: if nil -> empty map ; otherwise clean copy with values
		if el.IsNil() {
			return reflect.MakeMap(el.Type())
		}
		newMap := reflect.MakeMap(el.Type())
		iter := el.MapRange()
		for iter.Next() {
			k := iter.Key()
			v := iter.Value()
			sv := sanitizeElement(v, visited)
			if !sv.IsValid() {
				sv = v
			}
			newMap.SetMapIndex(k, sv)
		}
		return newMap

	default:
		return el
	}
}
