package util

func UnwrapUInt64(v *uint64) uint64 {
	if v == nil {
		return 0
	}
	return *v
}
