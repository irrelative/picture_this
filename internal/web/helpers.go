package web

import "strconv"

func itoa(value int) string {
	return strconv.Itoa(value)
}

func utoa(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}
