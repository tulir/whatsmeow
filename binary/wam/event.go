package wam

import (
	"reflect"
	"strconv"
	"strings"
)

func extractEventMeta(event any) EventMeta {
	meta := EventMeta{
		ID:      -1,
		Channel: "",
		Weight:  -1,
		StatsID: -1,
	}

	val := reflect.ValueOf(event)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		if metaTag, ok := field.Tag.Lookup("wammeta"); ok {
			entries := strings.Split(metaTag, ",")
			for _, entry := range entries {
				kv := strings.SplitN(entry, "=", 2)
				if len(kv) != 2 {
					continue
				}
				k, v := kv[0], kv[1]
				switch k {
				case "id":
					meta.ID, _ = strconv.Atoi(v)
				case "channel":
					meta.Channel = v
				case "weight":
					meta.Weight, _ = strconv.Atoi(v)
				case "privateStatsIdInt":
					meta.StatsID, _ = strconv.Atoi(v)
				}
			}
			break // Only need to process one _meta field
		}
	}

	return meta
}

func isExtended(event any) bool {
	val := reflect.ValueOf(event)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		if field.Name == "_meta" {
			continue
		}

		fieldVal := val.Field(i)

		if fieldVal.Kind() == reflect.Ptr && !fieldVal.IsNil() {
			return true
		}
	}

	return false
}

func encodeEventHeader(meta EventMeta, extended bool) []byte {
	flag := FLAG_EVENT
	if !extended {
		flag = FLAG_EVENT | FLAG_EXTENDED
	}
	return serializeData(meta.ID, int32(-meta.Weight), byte(flag))
}

func encodeEventField(key int, value any, extended bool) []byte {
	flag := FLAG_FIELD
	if !extended {
		flag = FLAG_FIELD | FLAG_EXTENDED
	}
	return serializeData(key, value, byte(flag))
}

func isZeroValue(val any) bool {
	return reflect.DeepEqual(val, reflect.Zero(reflect.TypeOf(val)).Interface())
}

func encodeEventProps(event any) []byte {
	val := reflect.ValueOf(event)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	typ := val.Type()

	var extended bool
	var props []byte

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Name == "_meta" {
			continue
		}

		tag := field.Tag.Get("wam")
		if tag == "" {
			continue
		}

		fieldID, err := strconv.Atoi(tag)
		if err != nil {
			continue
		}

		fieldVal := val.Field(i)
		if fieldVal.Kind() == reflect.Ptr {
			if fieldVal.IsNil() {
				continue
			}

			deref := fieldVal.Elem().Interface()

			if !extended && !isZeroValue(deref) {
				extended = true
			}

			props = append(props, encodeEventField(fieldID, deref, extended)...)
		} else {
			panic("received non-ptr value in wam event prop")
		}
	}

	return props
}
