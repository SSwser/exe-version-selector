package internal

// 合并默认参数和额外参数
func MergeArgs(defaults, extras []string) []string {
	return append(defaults, extras...)
}
