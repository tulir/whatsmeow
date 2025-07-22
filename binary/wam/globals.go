package wam

import (
	"reflect"
	"strconv"
	"strings"

	"go.mau.fi/whatsmeow/binary/wam/wamschema"
)

func encodeGlobalAttributes(globals wamschema.WAMGlobals) []byte {
	var result []byte
	val := reflect.ValueOf(globals)
	typ := reflect.TypeOf(globals)

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := field.Tag.Get("wam")
		if tag == "" {
			continue
		}

		idStr := strings.SplitN(tag, ",", 2)[0]
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}

		fieldVal := val.Field(i)
		if fieldVal.IsNil() {
			continue // Skip unset (nil) fields
		}

		// Dereference the pointer
		derefVal := fieldVal.Elem().Interface()

		result = append(result, serializeData(id, derefVal, FLAG_GLOBAL)...)
	}

	return result
}
