package caddy

import (
	"os"
	"sort"
)

func environmentWithOverrides(overrides map[string]string) []string {
	values := make(map[string]string)
	for _, value := range os.Environ() {
		for index := 0; index < len(value); index++ {
			if value[index] == '=' {
				values[value[:index]] = value[index+1:]
				break
			}
		}
	}
	for key, value := range overrides {
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}
