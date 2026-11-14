package dag

import (
	"fmt"
	"reflect"
)

// Key 标识一个依赖或产物。
// 类型 key（TypeKey）与命名 key（NameKey）属于不同命名空间，互不冲突。
type Key struct {
	typ  reflect.Type
	name string
}

// TypeKey 返回类型 T 的类型 key
func TypeKey[T any]() Key {
	return Key{typ: reflect.TypeFor[T]()}
}

// NameKey 返回指定名称的命名 key
func NameKey(name string) Key {
	return Key{name: name}
}

// IsType 报告 key 是否为类型 key
func (k Key) IsType() bool {
	return k.typ != nil
}

// IsZero 报告 key 是否未设置
func (k Key) IsZero() bool {
	return k.typ == nil && k.name == ""
}

// String 返回可读的 key 名称。
// 类型 key 使用完整包路径，避免不同包同名类型冲突。
func (k Key) String() string {
	if k.typ != nil {
		return typeKeyString(k.typ)
	}
	return k.name
}

// keySortString 用于稳定排序，区分类型 key 与命名 key
func keySortString(k Key) string {
	if k.typ != nil {
		return "t:" + k.String()
	}
	return "n:" + k.name
}

func typeKeyOf(t reflect.Type) Key {
	return Key{typ: t}
}

func typeKeyString(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Pointer:
		return "*" + typeKeyString(t.Elem())
	case reflect.Slice:
		return "[]" + typeKeyString(t.Elem())
	case reflect.Array:
		return fmt.Sprintf("[%d]%s", t.Len(), typeKeyString(t.Elem()))
	case reflect.Map:
		return "map[" + typeKeyString(t.Key()) + "]" + typeKeyString(t.Elem())
	case reflect.Chan:
		return t.String()
	}
	if t.Name() != "" && t.PkgPath() != "" {
		return t.PkgPath() + "." + t.Name()
	}
	return t.String()
}
