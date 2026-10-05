package service

import "encoding/json"

func marshalJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
