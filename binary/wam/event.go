package wam

import (
	"reflect"
	"strconv"
	"strings"
)

func extractEventMeta(event interface{}) EventMeta {
	meta := EventMeta{
		id:      -1,
		channel: "",
		weight:  -1,
		statsID: -1,
	}

	val := reflect.ValueOf(event).Elem()
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
					meta.id, _ = strconv.Atoi(v)
				case "channel":
					meta.channel = v
				case "weight":
					meta.weight, _ = strconv.Atoi(v)
				case "privateStatsIdInt":
					meta.statsID, _ = strconv.Atoi(v)
				}
			}
			break // Only need to process one _meta field
		}
	}

	return meta
}

func isExtended(event interface{}) bool {
	val := reflect.ValueOf(event).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := typ.Field(i)
		if field.Name == "_meta" {
			continue
		}
		if !val.Field(i).IsZero() {
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
	return serializeData(meta.id, -meta.weight, byte(flag))
}

func encodeEventField(key int, value interface{}, extended bool) []byte {
	flag := FLAG_FIELD
	if !extended {
		flag = FLAG_FIELD | FLAG_EXTENDED
	}
	return serializeData(key, value, byte(flag))
}

func isZeroValue(val interface{}) bool {
	return reflect.DeepEqual(val, reflect.Zero(reflect.TypeOf(val)).Interface())
}

func encodeEventProps(event interface{}) []byte {
	val := reflect.ValueOf(event).Elem()
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

		// Get field ID from the tag
		fieldID, err := strconv.Atoi(tag)
		if err != nil {
			continue
		}

		fieldVal := val.Field(i).Interface()

		// Check for non-nil/zero value to set "extended"
		if !extended && !isZeroValue(fieldVal) {
			extended = true
		}

		// Append serialized field
		props = append(props, encodeEventField(fieldID, fieldVal, extended)...)
	}

	// Adjust flags post-loop if necessary (if you're using it elsewhere)
	_ = extended // use this in outer logic if needed

	return props
}
