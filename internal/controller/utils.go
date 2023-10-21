package controller

func GiB2Bytes(datavolume int) int {
	return datavolume * 1024 * 1024 * 1024
}
