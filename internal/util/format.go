package util

import "fmt"

func FormatURL(base string, endpoint string, useTLS bool) string {
	prefix := "https://"
	if useTLS == false {
		prefix = "http://"
	}

	return fmt.Sprintf("%s%s%s", prefix, base, endpoint)
}
