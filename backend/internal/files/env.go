package files

import "os"

func envLookup(key string) string {
	v, _ := os.LookupEnv(key)
	return v
}
