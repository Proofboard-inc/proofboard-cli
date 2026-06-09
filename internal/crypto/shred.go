package crypto

func ZeroBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func DropStrings(values []string) []string {
	for i := range values {
		values[i] = ""
	}
	return nil
}
