package marshtojsonstring

import (
	"encoding/json"
	"reflect"
)

func MarshToJSONString(v interface{}) ([]string, error) {
	types := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	var result []string
	if types.Kind() == reflect.Slice {
		for i := 0; i < val.Len(); i++ {
			tmp, err := json.Marshal(val.Index(i).Interface())
			if err != nil {
				return nil, err
			}
			result = append(result, string(tmp))
		}
	} else {
		tmp, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		result = append(result, string(tmp))
	}
	return result, nil
}
