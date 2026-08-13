package utils

func IsPowerOfTwo(n uint64) bool {
	return n > 0 && (n&(n-1)) == 0
}
