//go:build !release

// runtime check int overflow

package as

import (
	"fmt"
	"math"
	"reflect"
)

func Uint8[T int8 | int16 | int32 | int64 | int | uint16 | uint32 | uint64 | uint](v T) uint8 {
	rv := reflect.ValueOf(v)
	switch rv.Kind() { //nolint:exhaustive
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		if rv.Uint() > math.MaxUint8 {
			panic(fmt.Sprintf("%d overflow uint32", v))
		}
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		if !(0 <= rv.Int() && rv.Int() <= math.MaxUint8) {
			panic(fmt.Sprintf("%d overflow uint32", v))
		}
	default:
		panic("unhandled default case")
	}

	return uint8(v)
}

func Uint16[T int8 | int16 | int32 | int64 | int | uint8 | uint32 | uint64 | uint](v T) uint16 {
	rv := reflect.ValueOf(v)
	switch rv.Kind() { //nolint:exhaustive
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		if rv.Uint() > math.MaxUint16 {
			panic(fmt.Sprintf("%d overflow uint32", v))
		}
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		if !(0 <= rv.Int() && rv.Int() <= math.MaxUint16) {
			panic(fmt.Sprintf("%d overflow uint32", v))
		}
	default:
		panic("unhandled default case")
	}

	return uint16(v)
}

func Uint32[T int8 | int16 | int32 | int64 | int | uint8 | uint16 | uint64 | uint](v T) (result uint32) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() { //nolint:exhaustive
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		if rv.Uint() > math.MaxUint32 {
			panic(fmt.Sprintf("%d overflow uint32", v))
		}
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		if !(0 <= rv.Int() && rv.Int() <= math.MaxUint32) {
			panic(fmt.Sprintf("%d overflow uint32", v))
		}
	default:
		panic("unhandled default case")
	}

	return uint32(v)
}

//
//func Uint64[T int8 | int16 | int32 | int64 | int | uint8 | uint16 | uint32 | uint64 | uint](v T) uint32 {
//	if v > math.MaxUint64 {
//		panic(fmt.Sprintf("%d overflow uint64", v))
//	}
//	if v < 0 {
//		panic(fmt.Sprintf("%d overflow uint64", v))
//	}
//
//	return uint32(v)
//}
//
//func Uint[T int8 | int16 | int32 | int64 | int | uint8 | uint16 | uint32 | uint64 | uint](v T) uint32 {
//	if v > math.MaxUint {
//		panic(fmt.Sprintf("%d overflow uint", v))
//	}
//	if v < 0 {
//		panic(fmt.Sprintf("%d overflow uint", v))
//	}
//
//	return uint32(v)
//}
