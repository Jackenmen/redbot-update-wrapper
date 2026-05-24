package main

import "strconv"

type Int interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

func ParseInt[T Int](s string) T {
	i, err := strconv.Atoi(s)
	if err != nil {
		panic(err)
	}
	return T(i)
}
