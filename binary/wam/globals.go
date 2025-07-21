package wam

import (
	"reflect"
	"strconv"
	"strings"

	"go.mau.fi/whatsmeow/wam"
)

func encodeGlobalAttributes(globals wam.WAMGlobals) []byte {
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

		fieldVal := val.Field(i).Interface()

		result = append(result, serializeData(id, fieldVal, FLAG_GLOBAL)...)
	}

	return result
}
